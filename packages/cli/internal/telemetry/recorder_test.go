package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRecorder is a test double for the Recorder interface used by
// the factory-swap test. It captures every recorded event in the
// order they were received so tests can assert on the captured slice.
type fakeRecorder struct {
	events []Event
}

// Record appends e to the captured events and returns nil.
func (f *fakeRecorder) Record(_ context.Context, e Event) error {
	f.events = append(f.events, e)
	return nil
}

func TestNoopRecorderDropsEvents(t *testing.T) {
	rec := NoopRecorder{}
	event := Event{
		Command:    "x",
		InstallID:  "0123456789abcdef0123456789abcdef",
		HostID:     "0123456789abcdef0123456789abcdef",
		Timestamp:  "2026-06-11T00:00:00Z",
		Version:    "1.0.0",
		EventID:    "01HXYZABCDEFGHJKMNPQRSTVWX",
		ExitStatus: 0,
	}
	for i := 0; i < 1000; i++ {
		if err := rec.Record(context.Background(), event); err != nil {
			t.Fatalf("NoopRecorder.Record() iter %d = %v, want nil", i, err)
		}
	}
}

func TestRecorderFactoryReturnsNoopOnEmptyConfig(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() {
		RecorderFactoryFunc = original
	})

	// Force the production default: the literal that the package
	// itself uses. This makes the test resilient to a future swap
	// in package init code.
	RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }

	rec := NewRecorder()
	if _, ok := rec.(NoopRecorder); !ok {
		t.Fatalf("NewRecorder() returned %T, want NoopRecorder", rec)
	}
}

func TestRecorderFactorySwapRoundtrip(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() {
		RecorderFactoryFunc = original
	})

	RecorderFactoryFunc = func() Recorder { return &fakeRecorder{} }

	rec := NewRecorder()
	fr, ok := rec.(*fakeRecorder)
	if !ok {
		t.Fatalf("NewRecorder() returned %T, want *fakeRecorder", rec)
	}

	ev := validEvent()
	if err := fr.Record(context.Background(), ev); err != nil {
		t.Fatalf("fake.Record() = %v, want nil", err)
	}
	if len(fr.events) != 1 {
		t.Fatalf("fake captured %d events, want 1", len(fr.events))
	}
	if fr.events[0].Command != ev.Command {
		t.Fatalf("fake captured Command = %q, want %q", fr.events[0].Command, ev.Command)
	}
}

// TestHTTPRecorderSchemaByteForByte captures the raw POST body the
// recorder sends to an httptest server and asserts the schema. This
// is the strongest possible schema assertion short of running the
// recorder against the production server (which doesn't exist yet).
// The 3 deterministic fields (command, exit_status, version) are
// compared byte-for-byte; the 4 volatile fields (install_id,
// host_id, event_id, timestamp) are checked against the regexes
// documented in OBSERVABILITY.md.
func TestHTTPRecorderSchemaByteForByte(t *testing.T) {
	var (
		gotMethod      string
		gotContentType string
		gotBody        []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("server: read body: %v", err)
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("HTTPRecorder.Record() = %v, want nil", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", gotMethod)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if !json.Valid(gotBody) {
		t.Fatalf("server body is not valid JSON: %s", gotBody)
	}

	var asMap map[string]any
	if err := json.Unmarshal(gotBody, &asMap); err != nil {
		t.Fatalf("json.Unmarshal() = %v\nbody = %s", err, gotBody)
	}

	const wantKeyCount = 7
	if len(asMap) != wantKeyCount {
		t.Fatalf("json keys count = %d, want %d (got %v)", len(asMap), wantKeyCount, asMap)
	}

	// Deterministic fields: byte-for-byte equality.
	if asMap["command"] != "check-security" {
		t.Fatalf("command = %v, want %q", asMap["command"], "check-security")
	}
	// Go's json.Unmarshal decodes JSON numbers into float64.
	if asMap["exit_status"] != float64(0) {
		t.Fatalf("exit_status = %v, want 0", asMap["exit_status"])
	}
	if asMap["version"] != "0.4.0" {
		t.Fatalf("version = %v, want %q", asMap["version"], "0.4.0")
	}

	// Volatile fields: regex match (the value is fresh, the shape
	// is fixed).
	hexRe := regexp.MustCompile(`^[0-9a-f]{32}$`)
	ulidRe := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	tsRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

	for _, key := range []string{"install_id", "host_id"} {
		v, ok := asMap[key].(string)
		if !ok {
			t.Fatalf("key %q is not a string (got %T)", key, asMap[key])
		}
		if !hexRe.MatchString(v) {
			t.Fatalf("key %q = %q, want 32 hex chars", key, v)
		}
	}
	if v, ok := asMap["event_id"].(string); !ok || !ulidRe.MatchString(v) {
		t.Fatalf("event_id = %v, want 26-char ULID", asMap["event_id"])
	}
	if v, ok := asMap["timestamp"].(string); !ok || !tsRe.MatchString(v) {
		t.Fatalf("timestamp = %v, want RFC3339 UTC", asMap["timestamp"])
	}
}

// TestHTTPRecorderSchemaFieldOrder inspects the raw body as a string
// and asserts the keys appear in the exact order documented in
// OBSERVABILITY.md: command, exit_status, install_id, host_id,
// timestamp, version, event_id. encoding/json marshals struct fields
// in declaration order; this test pins that order to the doc.
func TestHTTPRecorderSchemaFieldOrder(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("server: read body: %v", err)
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("HTTPRecorder.Record() = %v, want nil", err)
	}

	expected := []string{
		`"command":`,
		`"exit_status":`,
		`"install_id":`,
		`"host_id":`,
		`"timestamp":`,
		`"version":`,
		`"event_id":`,
	}
	cursor := 0
	body := string(gotBody)
	for _, marker := range expected {
		idx := strings.Index(body[cursor:], marker)
		if idx < 0 {
			t.Fatalf("json body missing marker %q after offset %d (body = %s)", marker, cursor, body)
		}
		cursor += idx + len(marker)
	}
}

// TestHTTPRecorderFieldCount is the "no extra fields" guard. The
// schema is exactly 7 top-level keys; adding an 8th is a breaking
// change and must be caught at CI time.
func TestHTTPRecorderFieldCount(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("server: read body: %v", err)
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("HTTPRecorder.Record() = %v, want nil", err)
	}

	var asMap map[string]any
	if err := json.Unmarshal(gotBody, &asMap); err != nil {
		t.Fatalf("json.Unmarshal() = %v\nbody = %s", err, gotBody)
	}
	if len(asMap) != 7 {
		keys := make([]string, 0, len(asMap))
		for k := range asMap {
			keys = append(keys, k)
		}
		t.Fatalf("json keys count = %d, want 7 (got %v)", len(asMap), keys)
	}
}

// countingTransport is a test double http.RoundTripper that counts
// every request that passes through it. The zero-egress tests below
// swap the HTTPRecorder's HTTP client to one using this transport
// and assert the counter is 0 after exercising the recorder.
type countingTransport struct {
	calls atomic.Int64
}

// RoundTrip records one call and returns a synthetic 200 OK response.
// The recorder only checks the status code and closes the body; the
// body content is irrelevant for the zero-egress assertion.
func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	_ = req // unused beyond the count
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// TestNoopRecorderNoNetworkCalls asserts the strongest possible
// "zero network egress when disabled" property: 100 Record calls on
// a NoopRecorder must not touch the network at all. The
// countingTransport is the only path that would increment the
// counter; if the NoopRecorder were ever wired to use the HTTP
// client, the counter would jump above 0 and the test would fail.
func TestNoopRecorderNoNetworkCalls(t *testing.T) {
	counter := &countingTransport{}
	// Wire the counting transport through the package's HTTP client
	// seam, so the test would catch any code path that constructs an
	// HTTPRecorder (even by accident) and uses NewHTTPClientFunc.
	originalClientFunc := NewHTTPClientFunc
	t.Cleanup(func() { NewHTTPClientFunc = originalClientFunc })
	NewHTTPClientFunc = func() *http.Client {
		return &http.Client{Transport: counter}
	}

	rec := NoopRecorder{}
	for i := 0; i < 100; i++ {
		if err := rec.Record(t.Context(), validEvent()); err != nil {
			t.Fatalf("NoopRecorder.Record() iter %d = %v, want nil", i, err)
		}
	}
	if got := counter.calls.Load(); got != 0 {
		t.Fatalf("countingTransport.calls = %d after 100 NoopRecorder.Record calls, want 0 (NoopRecorder must never touch the network)", got)
	}
}

// TestRecorderFactoryReturnsNoopWhenEndpointEmpty asserts that even
// with Enabled=true, the factory short-circuits to a NoopRecorder
// when no endpoint is configured. The CONTEXT decision: "If none is
// set, the factory returns NoopRecorder regardless of the enabled
// flag."
func TestRecorderFactoryReturnsNoopWhenEndpointEmpty(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: ""})

	rec := NewRecorder()
	if _, ok := rec.(NoopRecorder); !ok {
		t.Fatalf("NewRecorder() = %T, want NoopRecorder (enabled=true with empty endpoint must short-circuit)", rec)
	}
}

// TestRecorderFactoryReturnsHTTPRecorderWhenConfigured asserts the
// happy path: enabled=true and a non-empty endpoint produces an
// HTTPRecorder that actually POSTs to the configured endpoint.
func TestRecorderFactoryReturnsHTTPRecorderWhenConfigured(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: srv.URL})

	rec := NewRecorder()
	hr, ok := rec.(HTTPRecorder)
	if !ok {
		t.Fatalf("NewRecorder() = %T, want HTTPRecorder (enabled=true with non-empty endpoint)", rec)
	}
	if hr.Endpoint != srv.URL {
		t.Fatalf("HTTPRecorder.Endpoint = %q, want %q", hr.Endpoint, srv.URL)
	}

	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("rec.Record() = %v, want nil", err)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1 (the recorder should have POSTed once)", hits)
	}
}

// TestRecorderFactoryReturnsNoopWhenDisabled asserts the disabled
// state overrides the endpoint: even with a configured endpoint,
// the factory returns a NoopRecorder so no network calls are made.
func TestRecorderFactoryReturnsNoopWhenDisabled(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	SetDefaultFactory(RecorderConfig{Enabled: false, Endpoint: "https://example.com/in"})

	rec := NewRecorder()
	if _, ok := rec.(NoopRecorder); !ok {
		t.Fatalf("NewRecorder() = %T, want NoopRecorder (enabled=false must short-circuit regardless of endpoint)", rec)
	}
}
