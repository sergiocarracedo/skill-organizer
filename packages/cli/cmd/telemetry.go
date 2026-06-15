package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
)

// Package-level func-vars for test injection. Mirrors the
// skill_overlap.go pattern: production calls the real configpkg
// functions, tests reassign in t.Cleanup.
var (
	telemetryLoadConfig      = configpkg.LoadTelemetryConfigOrDefault
	telemetrySaveConfig      = configpkg.SaveTelemetryConfig
	telemetryLoadAgentCfg    = configpkg.LoadAgentSelectionConfigOrDefault
	telemetryAppDir          = configpkg.AppDir
	telemetryInfo            = func(format string, args ...any) { pterm.Info.Printfln(format, args...) }
	telemetrySuccess         = func(format string, args ...any) { pterm.Success.Printfln(format, args...) }
	telemetryWarning         = func(format string, args ...any) { pterm.Warning.Printfln(format, args...) }
)

// newTelemetryCommand is the parent `telemetry` cobra subcommand. It
// owns enable / disable / status / wipe. (Phase 5 REQ-10: the
// Phase 3 `rotate-host-id` subcommand is removed — there are no
// pseudonymous IDs to rotate.)
func newTelemetryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage anonymous, opt-in telemetry",
		Long:  "telemetry enable|disable|status|wipe — see OBSERVABILITY.md for the full schema and opt-in flow.",
	}
	cmd.AddCommand(newTelemetryEnableCommand())
	cmd.AddCommand(newTelemetryDisableCommand())
	cmd.AddCommand(newTelemetryStatusCommand())
	cmd.AddCommand(newTelemetryWipeCommand())
	return cmd
}

// newTelemetryEnableCommand flips the YAML to telemetry.enabled=true.
func newTelemetryEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable anonymous telemetry (writes telemetry.enabled: true to YAML)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			cfg, err := telemetryLoadConfig(registryPath)
			if err != nil {
				return err
			}
			cfg.Enabled = true
			if err := telemetrySaveConfig(registryPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			telemetrySuccess("Telemetry enabled.")
			return nil
		},
	}
}

// newTelemetryDisableCommand flips the YAML to telemetry.enabled=false
// AND best-effort removes the on-disk buffer so the next run cannot
// accidentally send any leftover events. (Phase 5 REQ-10: the
// previous wording mentioned the buffer "to make sure no events
// leave the device"; the new wording is shorter and matches the
// `telemetry wipe` semantics.)
func newTelemetryDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable anonymous telemetry (writes telemetry.enabled: false to YAML; clears the buffer)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			cfg, err := telemetryLoadConfig(registryPath)
			if err != nil {
				return err
			}
			cfg.Enabled = false
			if err := telemetrySaveConfig(registryPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			// Best-effort: clear the buffer file (zero network
			// egress on next run).
			appDir, _ := telemetryAppDir()
			if appDir != "" {
				_ = os.Remove(filepath.Join(appDir, telemetrypkg.BufferFileName))
			}
			telemetrySuccess("Telemetry disabled. The on-disk buffer has been cleared.")
			return nil
		},
	}
}

// newTelemetryStatusCommand prints the current telemetry state to
// stdout: three lines. Shows enabled state, recorder type, and
// the configured default model (if set). The endpoint, account id,
// insert key, install id, host id, and buffer size are no longer
// surfaced — they were either build-time (the recorder creds) or
// removed (the identity fields). The full schema and posture is
// documented in OBSERVABILITY.md and PRIVACY.md.
func newTelemetryStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current telemetry state (enabled, recorder type, default model)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			cfg, err := telemetryLoadConfig(registryPath)
			if err != nil {
				return err
			}
			agentCfg, err := telemetryLoadAgentCfg(registryPath)
			if err != nil {
				return err
			}
			telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{Enabled: cfg.Enabled})
			rec := telemetrypkg.NewRecorder()
			recType := recorderTypeName(rec)

			modelValue := agentCfg.DefaultModel
			if modelValue == "" {
				modelValue = "(none)"
			}

			telemetryInfo("Enabled:        %v", cfg.Enabled)
			telemetryInfo("Recorder:       %s", recType)

			if cfg.Enabled && recType == "NoopRecorder" &&
				(telemetrypkg.NewRelicEndpoint == "" || telemetrypkg.NewRelicAPIKey == "") {
				telemetryWarning("Misconfiguration: telemetry is enabled but this binary was built without New Relic credentials. Events are being dropped. See RELEASING.md for the fix.")
			}

			if nr, ok := rec.(*telemetrypkg.NewRelicRecorder); ok {
				telemetryInfo("Endpoint:       %s", nr.Endpoint)
				credStatus := "yes"
				if nr.InsertKey == "" {
					credStatus = "no"
				}
				telemetryInfo("Credentials:    %s", credStatus)
			}
			telemetryInfo("Default model:  %s", modelValue)

			appDir, err := telemetryAppDir()
			if err == nil && appDir != "" {
				bufPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
				info, statErr := os.Stat(bufPath)
				if statErr == nil {
					eventCount, _ := countBufferLines(bufPath)
					telemetryInfo("Buffer:         %s (%d bytes, %d events)", bufPath, info.Size(), eventCount)
				}
			}
			return nil
		},
	}
}

// countBufferLines returns the number of non-empty lines in the file,
// which for a JSONL buffer equals the number of buffered events.
func countBufferLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

// newTelemetryWipeCommand is the Phase 5 REQ-10 GDPR right-to-erasure
// command. It deletes the on-disk buffer (the only persistent data
// on the device). Since there are no IDs, that's all there is to
// delete. Idempotent: running on a clean app dir is a no-op.
func newTelemetryWipeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "wipe",
		Short: "Delete the on-disk telemetry buffer (idempotent, GDPR right-to-erasure)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			appDir, err := telemetryAppDir()
			if err != nil {
				return err
			}
			path := filepath.Join(appDir, telemetrypkg.BufferFileName)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("wipe buffer: %w", err)
			}
			return nil
		},
	}
}

// recorderTypeName maps the 2 known Recorder implementations (Phase
// 5 REQ-10) to a short, human-friendly name for the status output.
// Unknown types fall back to "%T" (the standard Go type-string) so
// a future custom recorder is still observable.
func recorderTypeName(r telemetrypkg.Recorder) string {
	switch r.(type) {
	case *telemetrypkg.NewRelicRecorder:
		return "NewRelicRecorder"
	case telemetrypkg.NoopRecorder:
		return "NoopRecorder"
	default:
		return fmt.Sprintf("%T", r)
	}
}
