package telemetry

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// firstRunOnAnswer writes the sentinel (mimicking the cmd package's
// behavior) and tracks invocations.
type firstRunCall struct {
	yes   bool
	count int
}

func (c *firstRunCall) callback(sentinelPath string) func(yes bool) error {
	return func(yes bool) error {
		c.count++
		c.yes = yes
		return os.WriteFile(sentinelPath, []byte(yesStr(yes)), 0o644)
	}
}

func TestFirstRunPromptStickyYes(t *testing.T) {
	originalTTY := IsStdInTTYFunc
	originalConfirm := ConfirmFunc
	t.Cleanup(func() {
		IsStdInTTYFunc = originalTTY
		ConfirmFunc = originalConfirm
	})

	IsStdInTTYFunc = func() bool { return true }
	ConfirmFunc = func(prompt string, defaultValue bool) (bool, error) {
		return true, nil
	}

	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	sentinel := filepath.Join(appDir, promptSentinelFile)

	tracker := &firstRunCall{}

	// First call: TTY + no sentinel -> prompt fires, sentinel is
	// written, onAnswer is called.
	svc.MaybeRunFirstRunPrompt(context.Background(), io.Discard, nil, tracker.callback(sentinel))
	if tracker.count != 1 {
		t.Fatalf("onAnswer called %d times on first run, want 1", tracker.count)
	}
	if !tracker.yes {
		t.Fatalf("onAnswer yes = false, want true")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not created on first run: %v", err)
	}

	// Second call: sentinel exists -> prompt is skipped, onAnswer
	// is NOT called.
	svc.MaybeRunFirstRunPrompt(context.Background(), io.Discard, nil, tracker.callback(sentinel))
	if tracker.count != 1 {
		t.Fatalf("onAnswer called %d times on second run, want 1 (sentinel short-circuits)", tracker.count)
	}
}

func TestFirstRunPromptStickyNo(t *testing.T) {
	originalTTY := IsStdInTTYFunc
	originalConfirm := ConfirmFunc
	t.Cleanup(func() {
		IsStdInTTYFunc = originalTTY
		ConfirmFunc = originalConfirm
	})

	IsStdInTTYFunc = func() bool { return true }
	ConfirmFunc = func(prompt string, defaultValue bool) (bool, error) {
		return false, nil
	}

	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	sentinel := filepath.Join(appDir, promptSentinelFile)

	tracker := &firstRunCall{}
	svc.MaybeRunFirstRunPrompt(context.Background(), io.Discard, nil, tracker.callback(sentinel))
	if tracker.count != 1 {
		t.Fatalf("onAnswer called %d times, want 1", tracker.count)
	}
	if tracker.yes {
		t.Fatalf("onAnswer yes = true, want false")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not created: %v", err)
	}
}

func TestFirstRunPromptNonTTYSkippedAndNotPersisted(t *testing.T) {
	originalTTY := IsStdInTTYFunc
	originalConfirm := ConfirmFunc
	t.Cleanup(func() {
		IsStdInTTYFunc = originalTTY
		ConfirmFunc = originalConfirm
	})

	IsStdInTTYFunc = func() bool { return false } // non-TTY: CI / piped input

	confirmCalled := 0
	ConfirmFunc = func(prompt string, defaultValue bool) (bool, error) {
		confirmCalled++
		return true, nil
	}

	appDir := t.TempDir()
	svc, err := New(appDir, "1.0.0", TelemetryConfig{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	sentinel := filepath.Join(appDir, promptSentinelFile)

	tracker := &firstRunCall{}
	svc.MaybeRunFirstRunPrompt(context.Background(), io.Discard, nil, tracker.callback(sentinel))
	if confirmCalled != 0 {
		t.Fatalf("ConfirmFunc called %d times in non-TTY mode, want 0 (Pitfall P10: prompt is skipped)", confirmCalled)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("sentinel was created in non-TTY mode (err = %v) — Pitfall P10 violation: non-TTY must not write the answer", err)
	}
	if tracker.count != 0 {
		t.Fatalf("onAnswer called %d times in non-TTY mode, want 0", tracker.count)
	}
}
