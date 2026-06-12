package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
)

func TestTelemetryEnableSubcommand(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalSave := telemetrySaveConfig
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetrySaveConfig = originalSave
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
	})

	initial := telemetrypkg.TelemetryConfig{Enabled: false}
	var saved telemetrypkg.TelemetryConfig
	saveCalled := 0
	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return initial, nil
	}
	telemetrySaveConfig = func(_ string, cfg telemetrypkg.TelemetryConfig) error {
		saved = cfg
		saveCalled++
		return nil
	}
	// Silence pterm output during the test.
	telemetryInfo = func(_ string, _ ...any) {}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryEnableCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}
	if saveCalled != 1 {
		t.Fatalf("SaveTelemetryConfig called %d times, want 1", saveCalled)
	}
	if !saved.Enabled {
		t.Fatalf("Saved Enabled = false, want true (telemetry enable flips it)")
	}
}

func TestTelemetryDisableSubcommand(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalSave := telemetrySaveConfig
	originalAppDir := telemetryAppDir
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetrySaveConfig = originalSave
		telemetryAppDir = originalAppDir
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
	})

	appDir := t.TempDir()
	bufferPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
	if err := os.WriteFile(bufferPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed buffer = %v", err)
	}

	initial := telemetrypkg.TelemetryConfig{Enabled: true}
	var saved telemetrypkg.TelemetryConfig
	saveCalled := 0
	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return initial, nil
	}
	telemetrySaveConfig = func(_ string, cfg telemetrypkg.TelemetryConfig) error {
		saved = cfg
		saveCalled++
		return nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryInfo = func(_ string, _ ...any) {}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryDisableCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}
	if saveCalled != 1 {
		t.Fatalf("SaveTelemetryConfig called %d times, want 1", saveCalled)
	}
	if saved.Enabled {
		t.Fatalf("Saved Enabled = true, want false (telemetry disable flips it)")
	}
	if _, err := os.Stat(bufferPath); !os.IsNotExist(err) {
		t.Fatalf("buffer file still exists after disable (err = %v) — disable must remove it", err)
	}
}

func TestTelemetryStatusSubcommand(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalIdentity := telemetryIdentity
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalNRAcc := telemetryNewRelicAccountID
	originalNRKey := telemetryNewRelicInsertKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryIdentity = originalIdentity
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetryNewRelicAccountID = originalNRAcc
		telemetryNewRelicInsertKey = originalNRKey
	})

	appDir := t.TempDir()

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true, Endpoint: "https://example.com"}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryIdentity = func(_ string) (telemetrypkg.Identity, error) {
		return telemetrypkg.Identity{
			InstallID: "0123456789abcdef0123456789abcdef",
			HostID:    "fedcba9876543210fedcba9876543210",
		}, nil
	}
	// Phase 4: stub the New Relic env reads to ensure the test is
	// hermetic. With both empty, the factory returns HTTPRecorder
	// (endpoint is set, but no NewRelic creds).
	telemetryNewRelicAccountID = func() string { return "" }
	telemetryNewRelicInsertKey = func() string { return "" }

	var buf bytes.Buffer
	telemetryInfo = func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
		buf.WriteString("\n")
	}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryStatusCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}

	got := buf.String()
	wantSubstrings := []string{
		"Enabled:",
		"https://example.com",
		"Recorder:",
		"Account ID:",
		"Insert key:",
		"01234567", // short install_id prefix
		"fedcba98", // short host_id prefix
		"Buffer file:",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q\noutput:\n%s", want, got)
		}
	}
}

// TestTelemetryStatusSubcommand_NewRelicConfigured asserts that
// when both New Relic env vars are set, the status output reports
// the NewRelicRecorder type with a 4-char account-id prefix and
// "present" for the insert key.
func TestTelemetryStatusSubcommand_NewRelicConfigured(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalIdentity := telemetryIdentity
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalNRAcc := telemetryNewRelicAccountID
	originalNRKey := telemetryNewRelicInsertKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryIdentity = originalIdentity
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetryNewRelicAccountID = originalNRAcc
		telemetryNewRelicInsertKey = originalNRKey
	})

	appDir := t.TempDir()

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true, Endpoint: "https://insights-collector.newrelic.com/v1/accounts/1234/events"}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryIdentity = func(_ string) (telemetrypkg.Identity, error) { return telemetrypkg.Identity{}, nil }
	telemetryNewRelicAccountID = func() string { return "1234567890" }
	telemetryNewRelicInsertKey = func() string { return "test-insert-key" }

	var buf bytes.Buffer
	telemetryInfo = func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
		buf.WriteString("\n")
	}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryStatusCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}

	got := buf.String()
	wantSubstrings := []string{
		"NewRelicRecorder",
		"1234...",
		"present",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q\noutput:\n%s", want, got)
		}
	}
}

// TestTelemetryStatusSubcommand_NewRelicIncomplete asserts the
// fallback path: only AccountID is set (no InsertKey), so the
// factory picks HTTPRecorder (because the endpoint is configured)
// and the insert key is shown as "<not set>".
func TestTelemetryStatusSubcommand_NewRelicIncomplete(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalIdentity := telemetryIdentity
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalNRAcc := telemetryNewRelicAccountID
	originalNRKey := telemetryNewRelicInsertKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryIdentity = originalIdentity
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetryNewRelicAccountID = originalNRAcc
		telemetryNewRelicInsertKey = originalNRKey
	})

	appDir := t.TempDir()

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true, Endpoint: "https://example.com/in"}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryIdentity = func(_ string) (telemetrypkg.Identity, error) { return telemetrypkg.Identity{}, nil }
	telemetryNewRelicAccountID = func() string { return "1234" }
	telemetryNewRelicInsertKey = func() string { return "" }

	var buf bytes.Buffer
	telemetryInfo = func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
		buf.WriteString("\n")
	}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryStatusCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}

	got := buf.String()
	wantSubstrings := []string{
		"Recorder:",
		"HTTPRecorder",
		"<not set>",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q\noutput:\n%s", want, got)
		}
	}
}

func TestTelemetryRotateHostIDSubcommand(t *testing.T) {
	originalAppDir := telemetryAppDir
	originalRotate := telemetryRotate
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	t.Cleanup(func() {
		telemetryAppDir = originalAppDir
		telemetryRotate = originalRotate
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
	})

	appDir := t.TempDir()
	const fakeID = "abcdef0123456789abcdef0123456789"

	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryRotate = func(_ string) (string, error) { return fakeID, nil }

	var buf bytes.Buffer
	telemetryInfo = func(_ string, _ ...any) {}
	telemetrySuccess = func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
		buf.WriteString("\n")
	}

	cmd := newTelemetryRotateHostIDCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, fakeID) {
		t.Fatalf("rotate-host-id output missing new ID %q\noutput:\n%s", fakeID, got)
	}
}
