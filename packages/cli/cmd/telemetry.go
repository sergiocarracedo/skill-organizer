package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
)

// Package-level func-vars for test injection. Mirrors the
// skill_overlap.go pattern: production calls the real configpkg /
// telemetrypkg functions, tests reassign in t.Cleanup.
var (
	telemetryLoadConfig = configpkg.LoadTelemetryConfigOrDefault
	telemetrySaveConfig = configpkg.SaveTelemetryConfig
	telemetryAppDir     = configpkg.AppDir
	telemetryRotate     = telemetrypkg.RotateHostID
	telemetryIdentity   = func(appDir string) (telemetrypkg.Identity, error) {
		return telemetrypkg.LoadOrCreate(appDir)
	}
	telemetryInfo    = func(format string, args ...any) { pterm.Info.Printfln(format, args...) }
	telemetrySuccess = func(format string, args ...any) { pterm.Success.Printfln(format, args...) }
)

// newTelemetryCommand is the parent `telemetry` cobra subcommand. It
// owns enable / disable / status / rotate-host-id.
func newTelemetryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage anonymous, opt-in telemetry",
		Long:  "telemetry enable|disable|status|rotate-host-id — see OBSERVABILITY.md for the full schema and opt-in flow.",
	}
	cmd.AddCommand(newTelemetryEnableCommand())
	cmd.AddCommand(newTelemetryDisableCommand())
	cmd.AddCommand(newTelemetryStatusCommand())
	cmd.AddCommand(newTelemetryRotateHostIDCommand())
	return cmd
}

// newTelemetryEnableCommand flips the YAML to telemetry.enabled=true.
// The user still has to set telemetry.endpoint (or the env / flag)
// before events leave the buffer.
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
			telemetrySuccess("Telemetry enabled. Edit telemetry.endpoint in %s to set the URL.", registryPath)
			return nil
		},
	}
}

// newTelemetryDisableCommand flips the YAML to telemetry.enabled=false
// AND best-effort removes the on-disk buffer so the next run cannot
// accidentally send any leftover events.
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
// stdout: enabled flag, endpoint, install/host ID prefixes, and the
// current buffer file size. The format matches the OBSERVABILITY.md
// "How to inspect" section.
func newTelemetryStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current telemetry state (enabled, endpoint, install_id, host_id, buffer size)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			cfg, err := telemetryLoadConfig(registryPath)
			if err != nil {
				return err
			}
			appDir, _ := telemetryAppDir()
			identity, _ := telemetryIdentity(appDir)
			bufferPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
			var bufferBytes int64
			if info, statErr := os.Stat(bufferPath); statErr == nil {
				bufferBytes = info.Size()
			}
			telemetryInfo("Enabled:      %v", cfg.Enabled)
			telemetryInfo("Endpoint:     %s", emptyAsNone(cfg.Endpoint))
			telemetryInfo("Install ID:   %s", shortID(identity.InstallID))
			telemetryInfo("Host ID:      %s", shortID(identity.HostID))
			telemetryInfo("Buffer file:  %s (%d bytes)", bufferPath, bufferBytes)
			return nil
		},
	}
}

// newTelemetryRotateHostIDCommand generates a new host_id (the
// install_id is preserved per CONTEXT) and prints it.
func newTelemetryRotateHostIDCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate-host-id",
		Short: "Rotate the host_id (the install_id is preserved)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			appDir, err := telemetryAppDir()
			if err != nil {
				return err
			}
			newID, err := telemetryRotate(appDir)
			if err != nil {
				return fmt.Errorf("rotate host id: %w", err)
			}
			telemetrySuccess("New host ID: %s", newID)
			return nil
		},
	}
}

// emptyAsNone returns "<none>" for empty input so the status output
// is unambiguous when the user has not set an endpoint.
func emptyAsNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// shortID returns the first 8 characters + "..." or "<unset>" if
// the input is empty. The full ID is 32 hex chars; the prefix is
// enough to disambiguate users in the status output.
func shortID(s string) string {
	if s == "" {
		return "<unset>"
	}
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "..."
}
