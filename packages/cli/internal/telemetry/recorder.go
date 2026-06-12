package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// Recorder is the sink for telemetry events. Implementations must be
// safe to call from a single goroutine (the CLI runs in a single
// goroutine per invocation). Errors are logged by the caller but
// never fatal — telemetry is best-effort by design.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// NoopRecorder drops every event. It is the default factory return
// value and the recorder used when telemetry is disabled or no
// endpoint is configured. Per CONTEXT, this path must produce zero
// network egress, which the package-level tests assert by wrapping
// http.DefaultTransport with a counting transport.
type NoopRecorder struct{}

// Record always returns nil. It is a no-op; the event is discarded.
func (NoopRecorder) Record(_ context.Context, _ Event) error {
	return nil
}

// RecorderFactoryFunc is a swappable function variable for
// NewRecorder. Tests reassign this to inject fakes; production code
// calls NewRecorder (the wrapper) so the swap is transparent to
// callers. The default closure is the 2-way SetDefaultFactory logic.
var RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }

// NewRecorder returns the package's current Recorder implementation.
// The factory is invoked on every call so callers receive a fresh
// recorder (and, in the NewRelicRecorder case, a fresh *http.Client).
func NewRecorder() Recorder {
	return RecorderFactoryFunc()
}

// NewHTTPClientFunc is a swappable function variable for the HTTP
// client the NewRelicRecorder uses to POST events. The default
// returns a *http.Client with a 10-second timeout. The
// counting-transport tests swap it to return a client with a
// counting transport to assert zero network egress.
var NewHTTPClientFunc = func() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// NewRelicEndpoint and NewRelicAPIKey are injected at build time via
// -ldflags. The user never configures these. An empty value means
// the binary was not built with credentials; the factory falls back
// to NoopRecorder.
var (
	NewRelicEndpoint = ""
	NewRelicAPIKey   = ""
)

// RecorderConfig is the input to the default factory. The cmd
// package's PersistentPreRun sets this from the user's
// TelemetryConfig (Phase 5 REQ-10: one key, telemetry.enabled).
// The New Relic endpoint and API key are build-time vars, not
// user-configurable: see NewRelicEndpoint and NewRelicAPIKey above.
type RecorderConfig struct {
	Enabled bool
}

// SetDefaultFactory swaps RecorderFactoryFunc to a 2-way closure:
//
//  1. NewRelicRecorder when telemetry is enabled AND both
//     NewRelicEndpoint and NewRelicAPIKey build-time vars are
//     non-empty (the binary was built with credentials).
//  2. NoopRecorder otherwise (zero network egress).
//
// The "0 network egress when disabled" guarantee is preserved: the
// closure returns a NoopRecorder{} value when !cfg.Enabled, which
// has no methods that touch the network. A dev build that leaves
// the build-time vars empty also routes to NoopRecorder (the
// empty-string guard is the dev-build escape hatch).
func SetDefaultFactory(cfg RecorderConfig) {
	RecorderFactoryFunc = func() Recorder {
		if !cfg.Enabled {
			return NoopRecorder{}
		}
		if NewRelicEndpoint != "" && NewRelicAPIKey != "" {
			return &NewRelicRecorder{
				Endpoint:   NewRelicEndpoint,
				InsertKey:  NewRelicAPIKey,
				HTTPClient: NewHTTPClientFunc(),
				Version:    RecorderVersion,
			}
		}
		return NoopRecorder{}
	}
}

// RecorderVersion is set by the cmd package (cmd/root.go) at
// PersistentPreRun time. The default empty string means the
// User-Agent header is omitted; production sets it to the CLI
// semver. Exported so cmd/root.go can write to it.
var RecorderVersion = ""

// newRelicEndpointTemplate is the default endpoint for the New
// Relic Insights Events API. The $ACCOUNT_ID placeholder is
// substituted with cfg.AccountID in the constructor. Per
// CONTEXT, the EU data center variant
// (insights-collector.eu01.nr-data.net) is documented in
// OBSERVABILITY.md but not defaulted here — users in the EU
// set telemetry.endpoint to override.
const newRelicEndpointTemplate = "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events"

// NewNewRelicRecorder returns a Recorder that POSTs a
// New-Relic-shaped array envelope to the New Relic Insights
// Events API. The endpointTemplate is the URL template with the
// account_id placeholder; the constructor substitutes the
// AccountID at recorder-construction time. The InsertKey is
// sent in the X-Insert-Key header per the CONTEXT decision.
func NewNewRelicRecorder(accountID, insertKey, endpointTemplate string) Recorder {
	endpoint := newRelicEndpointTemplate
	if endpointTemplate != "" {
		endpoint = endpointTemplate
	}
	endpoint = strings.ReplaceAll(endpoint, "$ACCOUNT_ID", accountID)
	return &NewRelicRecorder{
		Endpoint:   endpoint,
		InsertKey:  insertKey,
		HTTPClient: NewHTTPClientFunc(),
		Version:    RecorderVersion,
	}
}

// NewRelicRecorder POSTs events to the New Relic Insights Events
// API. The envelope is a JSON array of length 1 (one event per
// POST — the buffer drain calls Record once per event, not in
// batches). The envelope adds:
//
//   - "eventType": "skill_organizer_command" (New Relic requires
//     this field; it groups events in the NRDB UI).
//   - "clientTime": the RFC3339 string from event.Timestamp.
//     New Relic RESERVES the "timestamp" attribute name for
//     Unix-epoch integers (RESEARCH NP1); an RFC3339 string sent
//     in the "timestamp" field is silently dropped at ingest. The
//     rename is an envelope-only transform — the flat 5-field
//     schema in event.go and the byte-for-byte NewRelicRecorder
//     test are unchanged.
//
// Status code handling:
//   - 2xx: return nil. Service moves on.
//   - 413, 429: log a one-line warning via pterm and return nil.
//     The event is DROPPED (no buffer write). Per CONTEXT, the
//     local buffer is for network-down, not server-quota.
//     Returning a non-nil error here would trigger Service's
//     "recorder failed -> buffer write" path, creating an
//     infinite drain loop (RESEARCH NP4).
//   - 503: 1 retry with 250ms context-aware backoff. Final 503
//     returns the error (so the event is buffered for the next
//     drain).
//   - Other 4xx, 5xx, network errors: return the error. The
//     buffer is the right fallback for transient failures.
//
// The X-Insert-Key header is the New Relic Insights Event API
// auth method (per CONTEXT lock). The User-Agent is
// "skill-organizer/<version>" for ops visibility on the New
// Relic side.
type NewRelicRecorder struct {
	Endpoint   string // resolved URL (account_id substituted)
	InsertKey  string // X-Insert-Key header value
	HTTPClient *http.Client
	Version    string // for User-Agent; may be empty
}

// Record marshals the event as a New-Relic-shaped JSON array, POSTs
// it to the configured endpoint with the X-Insert-Key auth header,
// and handles 413/429 (hard drop) and 503 (1 retry) per the
// struct's doc. The envelope is the 5 schema fields plus
// eventType (the New Relic namespace) and clientTime (the RFC3339
// timestamp renamed per the struct's doc).
func (r NewRelicRecorder) Record(ctx context.Context, event Event) error {
	elem := map[string]any{
		"eventType":   "skill_organizer_command",
		"command":     event.Command,
		"exit_status": event.ExitStatus,
		"clientTime":  event.Timestamp, // renamed: see struct doc
		"version":     event.Version,
		"event_id":    event.EventID,
	}
	body, err := json.Marshal([]map[string]any{elem})
	if err != nil {
		return fmt.Errorf("marshal newrelic envelope: %w", err)
	}

	send := func() (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, fmt.Errorf("build newrelic request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Insert-Key", r.InsertKey)
		if r.Version != "" {
			req.Header.Set("User-Agent", "skill-organizer/"+r.Version)
		}
		resp, err := r.HTTPClient.Do(req)
		if err != nil {
			return 0, fmt.Errorf("post event to newrelic: %w", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode, nil
	}

	status, err := send()
	if err != nil {
		return err
	}
	// 413/429: hard drop, return nil, log a warning.
	if status == http.StatusRequestEntityTooLarge || status == http.StatusTooManyRequests {
		WarningFunc("telemetry: dropping event due to %d from New Relic (quota or rate-limit; will not retry)", status)
		return nil
	}
	// 503: 1 retry with 250ms context-aware backoff.
	if status == http.StatusServiceUnavailable {
		select {
		case <-time.After(250 * time.Millisecond):
			// fall through to the retry
		case <-ctx.Done():
			return ctx.Err()
		}
		status, err = send()
		if err != nil {
			return err
		}
		if status == http.StatusRequestEntityTooLarge || status == http.StatusTooManyRequests {
			WarningFunc("telemetry: dropping event due to %d from New Relic (quota or rate-limit after retry)", status)
			return nil
		}
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("post event to newrelic: unexpected status %d", status)
	}
	return nil
}

// WarningFunc is a swappable function variable for emitting
// warnings. The default writes to stderr via pterm.Warning
// (light-magenta, per the project's color rules — yellow is
// reserved for keyboard hints). Tests reassign in t.Cleanup
// to capture or silence the output.
var WarningFunc = func(format string, args ...any) {
	pterm.Warning.Printfln(format, args...)
}
