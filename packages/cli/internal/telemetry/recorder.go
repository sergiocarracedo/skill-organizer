package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// RecorderConfig is the input to the default factory. Plan 02 sets
// this from Service; the cmd package's PersistentPostRun is the only
// caller.
type RecorderConfig struct {
	Enabled  bool
	Endpoint string
}

// SetDefaultFactory swaps RecorderFactoryFunc to a closure that returns
// an HTTPRecorder when both Enabled and Endpoint are set, else a
// NoopRecorder. Idempotent: calling it with the same cfg is a no-op.
//
// The "0 network egress when disabled" guarantee is preserved: the
// closure returns a NoopRecorder{} value, which has no methods that
// touch the network.
func SetDefaultFactory(cfg RecorderConfig) {
	RecorderFactoryFunc = func() Recorder {
		if !cfg.Enabled || cfg.Endpoint == "" {
			return NoopRecorder{}
		}
		return NewHTTPRecorder(cfg.Endpoint)
	}
}
