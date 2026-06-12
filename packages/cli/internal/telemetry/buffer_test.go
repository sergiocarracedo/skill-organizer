package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBufferAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(filepath.Join(dir, "buf.jsonl"))

	commands := []string{"alpha", "beta", "gamma"}
	for _, cmd := range commands {
		event := validEvent()
		event.Command = cmd
		if err := b.Append(event); err != nil {
			t.Fatalf("Append(%q) = %v", cmd, err)
		}
	}

	var got []Event
	if err := b.Drain(func(e Event) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatalf("Drain() = %v", err)
	}

	if len(got) != len(commands) {
		t.Fatalf("Drain captured %d events, want %d", len(got), len(commands))
	}
	for i, cmd := range commands {
		if got[i].Command != cmd {
			t.Fatalf("Drain[%d].Command = %q, want %q", i, got[i].Command, cmd)
		}
	}
}

func TestBufferFIFOEvictionAt1MB(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(filepath.Join(dir, "buf.jsonl"))

	// 12000 events with a valid event payload (~224 bytes each)
	// sums to ~2.6 MB, well over the 1 MB cap. Append auto-evicts
	// when the file exceeds the cap, so by the end of the loop the
	// file must be at or below 1 MB and the oldest events must be
	// gone.
	const count = 12000
	for i := 0; i < count; i++ {
		ev := validEvent()
		ev.Command = fmt.Sprintf("e-%05d", i)
		ev.Timestamp = "2026-06-11T00:00:00Z" // fixed timestamp
		if err := b.Append(ev); err != nil {
			t.Fatalf("Append(%d) = %v", i, err)
		}
	}

	post, err := os.Stat(b.Path)
	if err != nil {
		t.Fatalf("Stat(%q) = %v", b.Path, err)
	}
	if post.Size() > BufferMaxBytes {
		t.Fatalf("buffer size after appends = %d, want <= %d (1 MB cap; eviction should have run)", post.Size(), BufferMaxBytes)
	}

	var collected []Event
	if err := b.Drain(func(e Event) error {
		collected = append(collected, e)
		return nil
	}); err != nil {
		t.Fatalf("Drain() = %v", err)
	}

	if len(collected) == 0 {
		t.Fatalf("Drain collected 0 events, want at least 1 (the buffer was non-empty after appends)")
	}
	firstCmd := collected[0].Command
	if firstCmd == "e-00000" {
		t.Fatalf("oldest event 'e-00000' survived FIFO eviction (file size after appends = %d)", post.Size())
	}
	lastCmd := collected[len(collected)-1].Command
	if lastCmd != fmt.Sprintf("e-%05d", count-1) {
		t.Fatalf("newest event = %q, want %q (the newest must be preserved)", lastCmd, fmt.Sprintf("e-%05d", count-1))
	}
}

func TestBufferDrainIdempotent(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(filepath.Join(dir, "buf.jsonl"))

	for i := 0; i < 5; i++ {
		ev := validEvent()
		ev.Command = fmt.Sprintf("c-%d", i)
		if err := b.Append(ev); err != nil {
			t.Fatalf("Append(%d) = %v", i, err)
		}
	}

	var first []Event
	if err := b.Drain(func(e Event) error {
		first = append(first, e)
		return nil
	}); err != nil {
		t.Fatalf("first Drain() = %v", err)
	}
	if len(first) != 5 {
		t.Fatalf("first Drain captured %d events, want 5", len(first))
	}

	var second []Event
	if err := b.Drain(func(e Event) error {
		second = append(second, e)
		return nil
	}); err != nil {
		t.Fatalf("second Drain() = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second Drain captured %d events, want 0", len(second))
	}
}

func TestBufferDrainPreservesOnSendFailure(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(filepath.Join(dir, "buf.jsonl"))

	for i := 0; i < 3; i++ {
		ev := validEvent()
		ev.Command = fmt.Sprintf("c-%d", i)
		if err := b.Append(ev); err != nil {
			t.Fatalf("Append(%d) = %v", i, err)
		}
	}

	calls := 0
	sendErr := fmt.Errorf("synthetic send failure")
	if err := b.Drain(func(e Event) error {
		calls++
		if calls == 2 {
			return sendErr
		}
		return nil
	}); err == nil {
		t.Fatalf("Drain() = nil, want error")
	} else if !strings.Contains(err.Error(), "synthetic send failure") {
		t.Fatalf("Drain() = %v, want it to wrap the synthetic error", err)
	}
	if calls != 2 {
		t.Fatalf("send callback called %d times, want 2 (third was not attempted)", calls)
	}

	var round2 []Event
	if err := b.Drain(func(e Event) error {
		round2 = append(round2, e)
		return nil
	}); err != nil {
		t.Fatalf("second Drain() = %v", err)
	}
	if len(round2) != 3 {
		t.Fatalf("second Drain captured %d events, want 3 (the truncate did not happen on first failure)", len(round2))
	}
}

func TestBufferAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "subdir", "buf.jsonl")
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("setup: nested dir already exists")
	}
	b := NewBuffer(path)
	if err := b.Append(validEvent()); err != nil {
		t.Fatalf("Append() = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("buffer file was not created: %v", err)
	}
}

func TestHTTPRecorderSmokeOK(t *testing.T) {
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
	if len(gotBody) == 0 {
		t.Fatalf("server received empty body")
	}
	var asMap map[string]any
	if err := json.Unmarshal(gotBody, &asMap); err != nil {
		t.Fatalf("body is not valid JSON: %v\nbody = %s", err, gotBody)
	}
	wantKeys := []string{
		"command", "exit_status", "install_id", "host_id",
		"timestamp", "version", "event_id",
	}
	if len(asMap) != len(wantKeys) {
		t.Fatalf("json keys count = %d, want %d (got %v)", len(asMap), len(wantKeys), asMap)
	}
	for _, k := range wantKeys {
		if _, ok := asMap[k]; !ok {
			t.Fatalf("json missing key %q (body = %s)", k, gotBody)
		}
	}
}

func TestHTTPRecorderFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	rec := HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}
	err := rec.Record(t.Context(), validEvent())
	if err == nil {
		t.Fatalf("HTTPRecorder.Record() on 500 = nil, want error")
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("error = %q, want it to mention 'unexpected status 500'", err.Error())
	}
}
