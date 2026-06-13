package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
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

// TestTelemetryStatusSubcommand asserts the Phase 5 REQ-10 status
// output (3 lines). The factory is not pre-set in this test, so the
// build-time vars (NewRelicEndpoint, NewRelicAPIKey) are empty and
// the factory routes to NoopRecorder. The test asserts the relevant
// substrings and that the OLD (Phase 3/4) 8-line fields are NOT
// present.
func TestTelemetryStatusSubcommand(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalLoadAgentCfg := telemetryLoadAgentCfg
	originalFactory := telemetrypkg.RecorderFactoryFunc
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalEndpoint := telemetrypkg.NewRelicEndpoint
	originalKey := telemetrypkg.NewRelicAPIKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryLoadAgentCfg = originalLoadAgentCfg
		telemetrypkg.RecorderFactoryFunc = originalFactory
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetrypkg.NewRelicEndpoint = originalEndpoint
		telemetrypkg.NewRelicAPIKey = originalKey
	})

	appDir := t.TempDir()
	// Force NoopRecorder (no build-time creds in the test).
	telemetrypkg.NewRelicEndpoint = ""
	telemetrypkg.NewRelicAPIKey = ""

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true}, nil
	}
	telemetryLoadAgentCfg = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultModel: "test-model"}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }

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
		"true",
		"Recorder:",
		"NoopRecorder",
		"Default model:",
		"test-model",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q\noutput:\n%s", want, got)
		}
	}
	// The OLD (Phase 3/4) 8-line fields are NOT present.
	banned := []string{
		"Endpoint:",
		"Account ID:",
		"Insert key:",
		"Install ID:",
		"Host ID:",
		"Buffer file:",
	}
	for _, b := range banned {
		if strings.Contains(got, b) {
			t.Fatalf("status output must not contain %q (Phase 5 REQ-10 collapses status)\noutput:\n%s", b, got)
		}
	}
}

// TestTelemetryStatusSubcommand_NewRelicConfigured asserts the
// happy path: when both build-time vars are set and Enabled is
// true, the status output reports the NewRelicRecorder type.
func TestTelemetryStatusSubcommand_NewRelicConfigured(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalLoadAgentCfg := telemetryLoadAgentCfg
	originalFactory := telemetrypkg.RecorderFactoryFunc
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalEndpoint := telemetrypkg.NewRelicEndpoint
	originalKey := telemetrypkg.NewRelicAPIKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryLoadAgentCfg = originalLoadAgentCfg
		telemetrypkg.RecorderFactoryFunc = originalFactory
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetrypkg.NewRelicEndpoint = originalEndpoint
		telemetrypkg.NewRelicAPIKey = originalKey
	})

	appDir := t.TempDir()
	telemetrypkg.NewRelicEndpoint = "https://insights-collector.newrelic.com/v1/accounts/1234/events"
	telemetrypkg.NewRelicAPIKey = "test-insert-key"

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true}, nil
	}
	telemetryLoadAgentCfg = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultModel: "test-model"}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }

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
		"true",
		"Recorder:",
		"NewRelicRecorder",
		"Default model:",
		"test-model",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q\noutput:\n%s", want, got)
		}
	}
}

// TestTelemetryStatusSubcommand_NoBuildVars asserts the dev-build
// escape hatch: even when Enabled is true, the factory returns
// NoopRecorder if the build-time vars are empty.
func TestTelemetryStatusSubcommand_NoBuildVars(t *testing.T) {
	originalLoad := telemetryLoadConfig
	originalAppDir := telemetryAppDir
	originalLoadAgentCfg := telemetryLoadAgentCfg
	originalFactory := telemetrypkg.RecorderFactoryFunc
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	originalEndpoint := telemetrypkg.NewRelicEndpoint
	originalKey := telemetrypkg.NewRelicAPIKey
	t.Cleanup(func() {
		telemetryLoadConfig = originalLoad
		telemetryAppDir = originalAppDir
		telemetryLoadAgentCfg = originalLoadAgentCfg
		telemetrypkg.RecorderFactoryFunc = originalFactory
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
		telemetrypkg.NewRelicEndpoint = originalEndpoint
		telemetrypkg.NewRelicAPIKey = originalKey
	})

	appDir := t.TempDir()
	// Build-time vars empty (the dev-build path).
	telemetrypkg.NewRelicEndpoint = ""
	telemetrypkg.NewRelicAPIKey = ""

	telemetryLoadConfig = func(_ string) (telemetrypkg.TelemetryConfig, error) {
		return telemetrypkg.TelemetryConfig{Enabled: true}, nil
	}
	telemetryLoadAgentCfg = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultModel: ""}, nil
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }

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
	if !strings.Contains(got, "NoopRecorder") {
		t.Fatalf("status output missing NoopRecorder (build vars empty)\noutput:\n%s", got)
	}
	if !strings.Contains(got, "(none)") {
		t.Fatalf("status output should show (none) for empty DefaultModel\noutput:\n%s", got)
	}
}

// TestTelemetryWipeSubcommand asserts the Phase 5 REQ-10 right-to-
// erasure command deletes the on-disk buffer and is idempotent.
func TestTelemetryWipeSubcommand(t *testing.T) {
	originalAppDir := telemetryAppDir
	originalInfo := telemetryInfo
	originalSuccess := telemetrySuccess
	t.Cleanup(func() {
		telemetryAppDir = originalAppDir
		telemetryInfo = originalInfo
		telemetrySuccess = originalSuccess
	})

	appDir := t.TempDir()
	bufferPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
	if err := os.WriteFile(bufferPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed buffer = %v", err)
	}
	telemetryAppDir = func() (string, error) { return appDir, nil }
	telemetryInfo = func(_ string, _ ...any) {}
	telemetrySuccess = func(_ string, _ ...any) {}

	cmd := newTelemetryWipeCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() on existing buffer = %v, want nil", err)
	}
	if _, err := os.Stat(bufferPath); !os.IsNotExist(err) {
		t.Fatalf("buffer file still exists after wipe (err = %v)", err)
	}

	// Idempotent: second run on a clean app dir is a no-op.
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() on missing buffer = %v, want nil (wipe must be idempotent)", err)
	}
}

// TestRecorderTypeName asserts the 2-way switch (Phase 5 REQ-10
// drops HTTPRecorder from the recorder factory).
func TestRecorderTypeName(t *testing.T) {
	if got := recorderTypeName(telemetrypkg.NoopRecorder{}); got != "NoopRecorder" {
		t.Fatalf("recorderTypeName(NoopRecorder) = %q, want NoopRecorder", got)
	}
	if got := recorderTypeName(&telemetrypkg.NewRelicRecorder{Endpoint: "x"}); got != "NewRelicRecorder" {
		t.Fatalf("recorderTypeName(*NewRelicRecorder) = %q, want NewRelicRecorder", got)
	}
}
