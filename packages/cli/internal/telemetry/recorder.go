package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
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
// callers. Plan 02 replaces the default factory to return an
// HTTPRecorder when telemetry is enabled and an endpoint is
// configured.
var RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }

// NewRecorder returns the package's current Recorder implementation.
// The factory is invoked on every call so callers receive a fresh
// recorder (and, in the HTTPRecorder case, a fresh *http.Client).
func NewRecorder() Recorder {
	return RecorderFactoryFunc()
}

// NewHTTPClientFunc is a swappable function variable for the HTTP
// client the HTTPRecorder uses to POST events. The default returns
// a *http.Client with a 10-second timeout. Plan 02 wires it; tests
// in plan 02 (and the future zero-egress test) swap it to return a
// client with a counting transport.
var NewHTTPClientFunc = func() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// HTTPRecorder POSTs each event as a JSON body to a fixed endpoint.
// The transport is swappable via NewHTTPClientFunc (the package var
// declared above), so the counting-transport test in plan 03 can
// intercept the call. The HTTP client has a 10s timeout (the default
// from NewHTTPClientFunc).
type HTTPRecorder struct {
	Endpoint string
	Client   *http.Client
}

// Record marshals event to JSON, POSTs it to the configured endpoint,
// and returns an error on any non-2xx status. 4xx and 5xx are failures
// (the caller should append the event to the buffer for a later drain).
// 3xx redirects are followed by the default http.Client; a final 2xx
// counts as success.
func (r HTTPRecorder) Record(ctx context.Context, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("post event: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// NewHTTPRecorder returns an HTTPRecorder with a default client. The
// client is built by calling NewHTTPClientFunc (the package-level seam
// for the counting-transport test injection).
func NewHTTPRecorder(endpoint string) Recorder {
	return HTTPRecorder{Endpoint: endpoint, Client: NewHTTPClientFunc()}
}

// RecorderConfig is the input to the default factory. The cmd
// package's PersistentPreRun sets this from the resolved
// TelemetryConfig (Phase 3) plus the two New Relic env vars
// (Phase 4). AccountID and InsertKey are env-only (no YAML) per
// the CONTEXT: secrets don't belong in the user's YAML file.
type RecorderConfig struct {
	Enabled   bool
	Endpoint  string
	AccountID string // SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID
	InsertKey string // SKILL_ORGANIZER_NEWRELIC_INSERT_KEY
}

// SetDefaultFactory swaps RecorderFactoryFunc to a 3-way closure:
//
//  1. NewRelicRecorder when telemetry is enabled AND both AccountID
//     and InsertKey are non-empty (the user has configured the New
//     Relic backend).
//  2. HTTPRecorder (the Phase 3 passthrough) when telemetry is
//     enabled AND a non-empty endpoint is set.
//  3. NoopRecorder otherwise (zero network egress).
//
// The "0 network egress when disabled" guarantee is preserved: the
// closure returns a NoopRecorder{} value when !cfg.Enabled, which
// has no methods that touch the network.
func SetDefaultFactory(cfg RecorderConfig) {
	RecorderFactoryFunc = func() Recorder {
		if !cfg.Enabled {
			return NoopRecorder{}
		}
		if cfg.AccountID != "" && cfg.InsertKey != "" {
			return NewNewRelicRecorder(cfg.AccountID, cfg.InsertKey, cfg.Endpoint)
		}
		if cfg.Endpoint != "" {
			return NewHTTPRecorder(cfg.Endpoint)
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

// NewNewRelicRecorder is the recorder surface for the New Relic
// Insights Events API (Phase 4). It wraps the project's flat 7-field
// Event in a backend-specific envelope: a JSON array of length 1
// with an `eventType: "skill_organizer_command"` prefix, the
// `timestamp` field renamed to `clientTime` in the envelope (New
// Relic reserves `timestamp` for Unix-epoch integers — see RESEARCH
// NP1), 413/429 responses hard-drop the event (return nil, no
// buffer fallback — RESEARCH NP4), and 503 responses get one
// context-aware 250ms retry (RESEARCH NP3). The X-Insert-Key
// header is the New Relic auth method (CONTEXT lock).
//
// The full struct + Record method is added in task 04-01-02. This
// stub returns a NoopRecorder so the factory's NewRelic branch
// compiles cleanly while the recorder is in flight.
func NewNewRelicRecorder(accountID, insertKey, endpointTemplate string) Recorder {
	// Placeholder body: returns a NoopRecorder until the
	// NewRelicRecorder struct and its Record method are added in
	// task 04-01-02. The endpoint-template substitution is computed
	// here so the helper is in place when the struct is wired.
	endpoint := newRelicEndpointTemplate
	if endpointTemplate != "" {
		endpoint = endpointTemplate
	}
	endpoint = strings.ReplaceAll(endpoint, "$ACCOUNT_ID", accountID)
	_ = endpoint
	_ = insertKey
	_ = accountID
	return NoopRecorder{}
}
