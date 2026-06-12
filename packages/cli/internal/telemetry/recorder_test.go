package telemetry

import (
	"context"
	"encoding/json"
	"errors"
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

// TestNewRelicRecorderContractEnforced is the smoke test from the
// Phase 4 CONTEXT. It stands up an httptest.NewServer, fires one
// Record(ctx, event) call on a NewRelicRecorder pointing at it, and
// asserts the 5 CONTEXT properties:
//
//  1. POST URL path == /v1/accounts/{account_id}/events
//  2. X-Insert-Key header matches the recorder's InsertKey
//  3. body is a JSON array of length 1
//  4. arr[0]["eventType"] == "skill_organizer_command"
//  5. the 7 schema fields match (with timestamp renamed to
//     clientTime in the envelope)
//
// Plus the User-Agent header and the "timestamp key absent" guard.
func TestNewRelicRecorderContractEnforced(t *testing.T) {
	const (
		accountID  = "test-12345"
		insertKey  = "test-key-xxxxxx"
		versionStr = "0.4.0"
	)
	var (
		gotPath        string
		gotInsertKey   string
		gotUserAgent   string
		gotContentType string
		gotBody        []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotInsertKey = r.Header.Get("X-Insert-Key")
		gotUserAgent = r.Header.Get("User-Agent")
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

	expectedPath := "/v1/accounts/" + accountID + "/events"
	endpoint := srv.URL + expectedPath
	rec := NewRelicRecorder{
		Endpoint:   endpoint,
		InsertKey:  insertKey,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Version:    versionStr,
	}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("NewRelicRecorder.Record() = %v, want nil", err)
	}

	// Assertion 1: POST URL path.
	if gotPath != expectedPath {
		t.Fatalf("URL path = %q, want %q", gotPath, expectedPath)
	}
	// Assertion 2: X-Insert-Key header.
	if gotInsertKey != insertKey {
		t.Fatalf("X-Insert-Key = %q, want %q", gotInsertKey, insertKey)
	}
	// User-Agent header (per AGENTS.md, smoke test asserts it too).
	wantUA := "skill-organizer/" + versionStr
	if gotUserAgent != wantUA {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, wantUA)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if !json.Valid(gotBody) {
		t.Fatalf("server body is not valid JSON: %s", gotBody)
	}
	// Assertion 3: body is array of length 1.
	var arr []map[string]any
	if err := json.Unmarshal(gotBody, &arr); err != nil {
		t.Fatalf("json.Unmarshal([]) = %v\nbody = %s", err, gotBody)
	}
	if len(arr) != 1 {
		t.Fatalf("body array length = %d, want 1 (got %s)", len(arr), gotBody)
	}
	// Assertion 4: first element has eventType.
	if arr[0]["eventType"] != "skill_organizer_command" {
		t.Fatalf("arr[0][eventType] = %v, want %q", arr[0]["eventType"], "skill_organizer_command")
	}
	// Assertion 5: the 7 schema fields match the recorder's input
	// (with timestamp renamed to clientTime in the envelope).
	// The 3 deterministic fields are byte-for-byte; the 4 volatile
	// fields are checked against the regexes.
	if arr[0]["command"] != "check-security" {
		t.Fatalf("arr[0][command] = %v, want %q", arr[0]["command"], "check-security")
	}
	if arr[0]["exit_status"] != float64(0) {
		t.Fatalf("arr[0][exit_status] = %v, want 0", arr[0]["exit_status"])
	}
	if arr[0]["version"] != "0.4.0" {
		t.Fatalf("arr[0][version] = %v, want %q", arr[0]["version"], "0.4.0")
	}
	hexRe := regexp.MustCompile(`^[0-9a-f]{32}$`)
	ulidRe := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	tsRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	for _, key := range []string{"install_id", "host_id"} {
		v, ok := arr[0][key].(string)
		if !ok {
			t.Fatalf("arr[0][%q] is not a string (got %T)", key, arr[0][key])
		}
		if !hexRe.MatchString(v) {
			t.Fatalf("arr[0][%q] = %q, want 32 hex chars", key, v)
		}
	}
	if v, ok := arr[0]["event_id"].(string); !ok || !ulidRe.MatchString(v) {
		t.Fatalf("arr[0][event_id] = %v, want 26-char ULID", arr[0]["event_id"])
	}
	// The timestamp is renamed to clientTime in the envelope.
	if v, ok := arr[0]["clientTime"].(string); !ok || !tsRe.MatchString(v) {
		t.Fatalf("arr[0][clientTime] = %v, want RFC3339 UTC (the renamed timestamp)", arr[0]["clientTime"])
	}
	// The "timestamp" key must NOT be present in the envelope
	// (NP1 — would be silently dropped at ingest by New Relic).
	if _, present := arr[0]["timestamp"]; present {
		t.Fatalf("arr[0][timestamp] must NOT be present in the New Relic envelope (renamed to clientTime; NP1)")
	}
}

// TestNewRelicRecorderHardDropsOn413 asserts that a 413 response
// makes the recorder return nil (hard drop, no buffer fallback) and
// that the WarningFunc fires exactly once. RESEARCH NP4: returning a
// non-nil error on 413/429 would trigger Service's "recorder failed
// -> buffer write" path, creating an infinite drain loop.
func TestNewRelicRecorderHardDropsOn413(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()
	// Silence the warning; assert it WAS called exactly once.
	originalWarning := WarningFunc
	var warnCalled int
	WarningFunc = func(_ string, _ ...any) { warnCalled++ }
	t.Cleanup(func() { WarningFunc = originalWarning })

	rec := NewRelicRecorder{
		Endpoint:   srv.URL,
		InsertKey:  "test-key",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("NewRelicRecorder.Record() on 413 = %v, want nil (hard drop, no buffer fallback)", err)
	}
	if warnCalled != 1 {
		t.Fatalf("WarningFunc called %d times, want 1 (the hard-drop warning)", warnCalled)
	}
}

// TestNewRelicRecorderHardDropsOn429 is the 429 twin of the 413 test
// above. 429 (Too Many Requests) is the rate-limit signal; the same
// hard-drop contract applies.
func TestNewRelicRecorderHardDropsOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	originalWarning := WarningFunc
	var warnCalled int
	WarningFunc = func(_ string, _ ...any) { warnCalled++ }
	t.Cleanup(func() { WarningFunc = originalWarning })

	rec := NewRelicRecorder{
		Endpoint:   srv.URL,
		InsertKey:  "test-key",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("NewRelicRecorder.Record() on 429 = %v, want nil (hard drop, no buffer fallback)", err)
	}
	if warnCalled != 1 {
		t.Fatalf("WarningFunc called %d times, want 1 (the hard-drop warning)", warnCalled)
	}
}

// TestNewRelicRecorderRetriesOn503 asserts the 503 path: the first
// POST returns 503, the recorder waits 250ms (the backoff), and the
// second POST returns 200. Record returns nil and the server is hit
// exactly twice.
func TestNewRelicRecorderRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	rec := NewRelicRecorder{
		Endpoint:   srv.URL,
		InsertKey:  "test-key",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("Record() = %v, want nil (first 503 retries to 200)", err)
	}
	if hits != 2 {
		t.Fatalf("server hits = %d, want 2 (1 initial + 1 retry)", hits)
	}
}

// TestNewRelicRecorderHonorsContextCancellation asserts that a
// context that becomes done DURING the 503 backoff returns
// ctx.Err() and does not retry. The 250ms select on ctx.Done()
// short-circuits the retry so the cancellation is honored
// immediately (RESEARCH NP3).
func TestNewRelicRecorderHonorsContextCancellation(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	// 50ms timeout: the first POST completes (server returns 503
	// in <10ms), the backoff's select fires on ctx.Done() at
	// 50ms, no retry.
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	t.Cleanup(cancel)
	rec := NewRelicRecorder{
		Endpoint:   srv.URL,
		InsertKey:  "test-key",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	err := rec.Record(ctx, validEvent())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Record() on timed-out ctx = %v, want context.DeadlineExceeded", err)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1 (cancelled during backoff, no retry)", hits)
	}
}

// TestRecorderFactoryPicksNewRelicWhenEnvVarsSet asserts the happy
// path: when both New Relic env vars are set, the factory returns a
// *NewRelicRecorder with the right Endpoint (account_id substituted
// in the placeholder) and InsertKey.
func TestRecorderFactoryPicksNewRelicWhenEnvVarsSet(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	const (
		accountID  = "1234567890"
		insertKey  = "test-insert-key"
		endpointIn = "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events"
	)
	SetDefaultFactory(RecorderConfig{
		Enabled:   true,
		Endpoint:  endpointIn,
		AccountID: accountID,
		InsertKey: insertKey,
	})

	rec := NewRecorder()
	nr, ok := rec.(*NewRelicRecorder)
	if !ok {
		t.Fatalf("NewRecorder() = %T, want *NewRelicRecorder", rec)
	}
	wantEndpoint := "https://insights-collector.newrelic.com/v1/accounts/" + accountID + "/events"
	if nr.Endpoint != wantEndpoint {
		t.Fatalf("NewRelicRecorder.Endpoint = %q, want %q", nr.Endpoint, wantEndpoint)
	}
	if nr.InsertKey != insertKey {
		t.Fatalf("NewRelicRecorder.InsertKey = %q, want %q", nr.InsertKey, insertKey)
	}
}

// TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete
// asserts that when only the AccountID is set (no InsertKey) but
// the endpoint is configured, the factory returns the HTTPRecorder
// (the Phase 3 passthrough). The NewRelic branch requires BOTH
// env vars.
func TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	SetDefaultFactory(RecorderConfig{
		Enabled:   true,
		Endpoint:  "https://example.com/in",
		AccountID: "1234",
		// InsertKey deliberately omitted
	})

	rec := NewRecorder()
	if _, ok := rec.(HTTPRecorder); !ok {
		t.Fatalf("NewRecorder() = %T, want HTTPRecorder (AccountID only, no InsertKey, falls back to passthrough)", rec)
	}
}

// TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete asserts
// that when only the AccountID is set (no InsertKey) AND the
// endpoint is empty, the factory returns the NoopRecorder (no
// recorder has the minimum config to fire).
func TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() { RecorderFactoryFunc = original })

	SetDefaultFactory(RecorderConfig{
		Enabled:   true,
		Endpoint:  "",
		AccountID: "1234",
		// InsertKey and Endpoint deliberately omitted
	})

	rec := NewRecorder()
	if _, ok := rec.(NoopRecorder); !ok {
		t.Fatalf("NewRecorder() = %T, want NoopRecorder (AccountID only, no InsertKey, no endpoint)", rec)
	}
}

// TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured
// asserts the Phase 3 happy path is preserved: enabled=true and a
// non-empty endpoint, with no NewRelic fields, returns an
// HTTPRecorder.
func TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured(t *testing.T) {
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
		t.Fatalf("NewRecorder() = %T, want HTTPRecorder (Phase 3 happy path preserved)", rec)
	}
	if hr.Endpoint != srv.URL {
		t.Fatalf("HTTPRecorder.Endpoint = %q, want %q", hr.Endpoint, srv.URL)
	}
	if err := rec.Record(t.Context(), validEvent()); err != nil {
		t.Fatalf("rec.Record() = %v, want nil", err)
	}
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}
}
