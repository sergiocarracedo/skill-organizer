package telemetry

import (
	"context"
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
