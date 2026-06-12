package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandNameNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"on", "enable"},
		{"off", "disable"},
		{"install", "add"},
		{"rm", "delete"},
		{"add", "add"},                       // passthrough
		{"delete", "delete"},                 // passthrough
		{"check-overlap", "check-overlap"},   // passthrough (no alias)
		{"unknown-command", "unknown-command"}, // passthrough
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := NormalizeCommandName(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizeCommandName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveEndpointPrecedence(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		yaml string
		want string
	}{
		{"flag wins over env and yaml", "https://flag", "https://env", "https://yaml", "https://flag"},
		{"env wins over yaml", "", "https://env", "https://yaml", "https://env"},
		{"yaml when flag and env empty", "", "", "https://yaml", "https://yaml"},
		{"empty when all empty", "", "", "", ""},
		{"whitespace flag is unset", "  ", "", "https://yaml", "https://yaml"},
		{"whitespace env is unset", "", "   ", "https://yaml", "https://yaml"},
		{"flag wins even if env is set", "https://flag", "https://env", "", "https://flag"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveEndpoint(tc.flag, tc.env, tc.yaml)
			if got != tc.want {
				t.Fatalf("ResolveEndpoint(%q, %q, %q) = %q, want %q", tc.flag, tc.env, tc.yaml, got, tc.want)
			}
		})
	}
}

// failingRecorder returns an error on every Record call.
type failingRecorder struct{}

func (failingRecorder) Record(_ context.Context, _ Event) error {
	return errors.New("synthetic recorder failure")
}

func TestService_RecordEvent_WritesToBufferOnFailure(t *testing.T) {
	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{Enabled: true, Endpoint: "https://example.com/in"})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	// Replace the recorder with one that always fails.
	svc.Recorder = failingRecorder{}

	if err := svc.RecordEvent(context.Background(), "test", 0); err == nil {
		t.Fatalf("RecordEvent() = nil, want error from failing recorder")
	}

	// Buffer file should now exist with exactly one event.
	info, err := os.Stat(svc.Buffer.Path)
	if err != nil {
		t.Fatalf("Stat(%q) = %v", svc.Buffer.Path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("buffer file is empty; expected one event line")
	}

	// Drain the buffer with a custom send callback (the default
	// DrainBuffer would use the failing recorder and error out).
	collected := 0
	if err := svc.Buffer.Drain(func(_ Event) error {
		collected++
		return nil
	}); err != nil {
		t.Fatalf("Drain() = %v", err)
	}
	if collected != 1 {
		t.Fatalf("Drain collected %d events, want 1", collected)
	}

	post, err := os.Stat(svc.Buffer.Path)
	if err != nil {
		t.Fatalf("Stat(%q) after drain = %v", svc.Buffer.Path, err)
	}
	if post.Size() != 0 {
		t.Fatalf("buffer size after drain = %d, want 0 (truncated)", post.Size())
	}
}

func TestService_RecordEvent_NoEgressWhenDisabled(t *testing.T) {
	// CRITICAL (BUG #2): SetDefaultFactory must be called BEFORE
	// telemetry.New — the Service captures the Recorder inside
	// New by calling NewRecorder() (which calls
	// RecorderFactoryFunc at construction time). Swapping the
	// factory after New returns is a no-op for this Service.
	originalFactory := RecorderFactoryFunc
	t.Cleanup(func() {
		RecorderFactoryFunc = originalFactory
	})
	SetDefaultFactory(RecorderConfig{Enabled: false, Endpoint: ""})

	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{Enabled: false, Endpoint: ""})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	if _, ok := svc.Recorder.(NoopRecorder); !ok {
		t.Fatalf("svc.Recorder = %T, want NoopRecorder (SetDefaultFactory must run BEFORE New)", svc.Recorder)
	}

	if err := svc.RecordEvent(context.Background(), "test", 0); err != nil {
		t.Fatalf("RecordEvent() = %v, want nil (noop recorder)", err)
	}

	// Buffer file should NOT exist: the noop path didn't write.
	if _, err := os.Stat(svc.Buffer.Path); !os.IsNotExist(err) {
		t.Fatalf("buffer file exists; the noop path should not have written (stat err = %v)", err)
	}
}

// countingRecorder records every event it sees and returns nil.
type countingRecorder struct {
	calls int
}

func (c *countingRecorder) Record(_ context.Context, _ Event) error {
	c.calls++
	return nil
}

func TestService_DrainBuffer_SendsAndTruncates(t *testing.T) {
	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{Enabled: true, Endpoint: "https://example.com/in"})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	counter := &countingRecorder{}
	svc.Recorder = counter

	for i := 0; i < 3; i++ {
		ev := validEvent()
		ev.Command = fmt.Sprintf("c-%d", i)
		if err := svc.Buffer.Append(ev); err != nil {
			t.Fatalf("Buffer.Append(%d) = %v", i, err)
		}
	}

	if err := svc.DrainBuffer(context.Background()); err != nil {
		t.Fatalf("DrainBuffer() = %v", err)
	}
	if counter.calls != 3 {
		t.Fatalf("counter.calls = %d, want 3", counter.calls)
	}
	info, err := os.Stat(svc.Buffer.Path)
	if err != nil {
		t.Fatalf("Stat(%q) after drain = %v", svc.Buffer.Path, err)
	}
	if info.Size() != 0 {
		t.Fatalf("buffer size after drain = %d, want 0 (truncated)", info.Size())
	}
}

func TestService_New_CreatesAppDir(t *testing.T) {
	nested := filepath.Join(t.TempDir(), "nested", "config")
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Fatalf("setup: nested dir already exists")
	}

	svc, err := New(nested, "1.0.0", TelemetryConfig{})
	if err != nil {
		t.Fatalf("New() on missing dir = %v", err)
	}
	if svc == nil {
		t.Fatalf("New() returned nil Service")
	}
	if _, err := os.Stat(filepath.Join(nested, installIDFile)); err != nil {
		t.Fatalf("install_id file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, hostIDFile)); err != nil {
		t.Fatalf("host_id file missing: %v", err)
	}
}
