package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
)

func TestRootPersistentPreRun_SkipsTelemetryCommand(t *testing.T) {
	originalConfirm := telemetrypkg.ConfirmFunc
	t.Cleanup(func() {
		telemetrypkg.ConfirmFunc = originalConfirm
	})

	confirmCalls := 0
	telemetrypkg.ConfirmFunc = func(_ string, _ bool) (bool, error) {
		confirmCalls++
		return true, nil
	}

	// Pre-create the sentinel so the prompt would otherwise
	// short-circuit on the next call (irrelevant here, but
	// keeps the test isolated from prior runs).
	appDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", appDir)
	// Force the real AppDir() to use this XDG_CONFIG_HOME so the
	// in-PreRun code can resolve it.
	if err := os.MkdirAll(filepath.Join(appDir, "skill-organizer"), 0o755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	sentinel := filepath.Join(appDir, "skill-organizer", "telemetry-prompted")
	if err := os.WriteFile(sentinel, []byte("yes"), 0o644); err != nil {
		t.Fatalf("WriteFile(sentinel) = %v", err)
	}

	telemetryCmd := newTelemetryCommand()
	// Call PersistentPreRun with the PARENT telemetryCmd (Name()
	// is "telemetry"), which is in the skip set.
	rootCmd.PersistentPreRun(telemetryCmd, nil)

	if confirmCalls != 0 {
		t.Fatalf("ConfirmFunc called %d times for telemetryCmd, want 0 (PreRun guard skips telemetry)", confirmCalls)
	}
}

func TestRootPersistentPostRun_EmitsEvent(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", appDir)

	originalFactory := telemetrypkg.RecorderFactoryFunc
	originalEndpoint := telemetrypkg.NewRelicEndpoint
	originalKey := telemetrypkg.NewRelicAPIKey
	t.Cleanup(func() {
		telemetrypkg.RecorderFactoryFunc = originalFactory
		telemetrypkg.NewRelicEndpoint = originalEndpoint
		telemetrypkg.NewRelicAPIKey = originalKey
	})

	// Initialize the default factory (Enabled=true) and THEN
	// swap RecorderFactoryFunc for a closure that returns our
	// capturing recorder. The order matters: SetDefaultFactory
	// replaces RecorderFactoryFunc, so the custom swap must come
	// after. (Build-time vars are empty in this test so the
	// factory would otherwise route to NoopRecorder, but we
	// override the closure directly.)
	telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{Enabled: true})

	captured := &eventCapture{}
	telemetrypkg.RecorderFactoryFunc = func() telemetrypkg.Recorder {
		return captured
	}

	svc, err := telemetrypkg.New(appDir, "1.0.0", telemetrypkg.TelemetryConfig{Enabled: true})
	if err != nil {
		t.Fatalf("telemetrypkg.New() = %v", err)
	}

	// Build a stand-alone cobra command (not registered on rootCmd)
	// so the PersistentPostRun can be exercised with a synthetic
	// Command whose Name() is "test".
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(withTelemetryService(context.Background(), svc))

	rootCmd.PersistentPostRun(cmd, nil)

	if len(captured.events) != 1 {
		t.Fatalf("captured %d events, want 1", len(captured.events))
	}
	got := captured.events[0]
	if got.Command != "test" {
		t.Fatalf("captured event Command = %q, want %q", got.Command, "test")
	}
	if got.ExitStatus != 0 {
		t.Fatalf("captured event ExitStatus = %d, want 0", got.ExitStatus)
	}
	if got.Version != "1.0.0" {
		t.Fatalf("captured event Version = %q, want %q", got.Version, "1.0.0")
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("captured event Validate() = %v", err)
	}
}

func TestRootPersistentPreRun_FiresFirstRunPrompt_OnFirstRun(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", appDir)

	// Make sure the registry path resolves under the temp dir.
	if err := os.MkdirAll(filepath.Join(appDir, "skill-organizer"), 0o755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}

	originalTTY := telemetrypkg.IsStdInTTYFunc
	originalConfirm := telemetrypkg.ConfirmFunc
	originalFactory := telemetrypkg.RecorderFactoryFunc
	t.Cleanup(func() {
		telemetrypkg.IsStdInTTYFunc = originalTTY
		telemetrypkg.ConfirmFunc = originalConfirm
		telemetrypkg.RecorderFactoryFunc = originalFactory
	})

	telemetrypkg.IsStdInTTYFunc = func() bool { return true }
	telemetrypkg.ConfirmFunc = func(_ string, _ bool) (bool, error) { return true, nil }
	telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{Enabled: false})

	cmd := &cobra.Command{Use: "sync"}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.SetContext(context.Background())

	rootCmd.PersistentPreRun(cmd, nil)

	sentinel := filepath.Join(appDir, "skill-organizer", "telemetry-prompted")
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not created after first run: %v", err)
	}

	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		t.Fatalf("RegistryPath() = %v", err)
	}
	cfg, err := configpkg.LoadTelemetryConfigOrDefault(registryPath)
	if err != nil {
		t.Fatalf("LoadTelemetryConfigOrDefault() = %v", err)
	}
	if !cfg.Enabled {
		t.Fatalf("TelemetryConfig.Enabled = false, want true (onAnswer wrote yes)")
	}

	// Confirm the YAML file actually contains the telemetry section.
	content, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v", registryPath, err)
	}
	if !strings.Contains(string(content), "telemetry") {
		t.Fatalf("registry YAML missing 'telemetry' key\ncontent:\n%s", content)
	}
	// Sanity: round-trip through yaml.Unmarshal to confirm the
	// schema is well-formed.
	var raw map[string]any
	if err := yaml.Unmarshal(content, &raw); err != nil {
		t.Fatalf("yaml.Unmarshal() = %v\ncontent:\n%s", err, content)
	}
}

// eventCapture is a test-only Recorder implementation that records
// every event it sees. Used by the root.go integration tests.
type eventCapture struct {
	events []telemetrypkg.Event
}

func (c *eventCapture) Record(_ context.Context, e telemetrypkg.Event) error {
	c.events = append(c.events, e)
	return nil
}
