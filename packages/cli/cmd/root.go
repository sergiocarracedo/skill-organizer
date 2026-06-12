package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	maintenancepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/maintenance"
	selfupdatepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/selfupdate"
	servicepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/service"
	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"

	configPath        string
	telemetryEndpoint string
)

var rootCmd = &cobra.Command{
	Use:   "skill-organizer",
	Short: "Organize structured skill trees into flat tool-readable targets",
	Long:  "skill-organizer synchronizes organized source skill trees into flat target skills folders and manages watched skill projects.",
}

func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return nil
	}
	if shouldPrintCommandHelp(err) {
		_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "ERROR   %v\n\n", err)
		cmd, _, findErr := rootCmd.Find(os.Args[1:])
		if findErr == nil && cmd != nil {
			_ = cmd.Help()
			return nil
		}
	}
	return err
}

func shouldPrintCommandHelp(err error) bool {
	if err == nil {
		return false
	}
	text := strings.TrimSpace(err.Error())
	if strings.Contains(text, "accepts ") && strings.Contains(text, " arg(s), received ") {
		return true
	}
	return false
}

func init() {
	maintenancepkg.IsServiceRunningFunc = func() (bool, error) {
		registryPath, err := configpkg.RegistryPath()
		if err != nil {
			return false, err
		}
		return servicepkg.IsRunning(registryPath)
	}
	// Wire the prompt's func-var seam to the local confirm helper so
	// the telemetry package doesn't import the cmd package.
	telemetrypkg.ConfirmFunc = confirm
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to a project config file")
	rootCmd.PersistentFlags().StringVar(&telemetryEndpoint, "telemetry-endpoint", "", "Endpoint URL for telemetry events (env: SKILL_ORGANIZER_TELEMETRY_ENDPOINT, yaml: telemetry.endpoint)")
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("%s\n%s\ncommit %s, built %s\n", cliLogo(), version, commit, date))
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		if cmd == rootCmd {
			return
		}
		if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "telemetry" {
			return
		}
		printCLIHeader(cmd.OutOrStdout())
		maintenancepkg.MaybeRunBackupGC(cmd.Context())
		selfupdatepkg.MaybeNotify(cmd.Context(), version, cmd.OutOrStdout())
		if cmd.Name() != "check-updates" {
			maintenancepkg.MaybeNotifySkillUpdates(cmd.Context(), cmd.OutOrStdout())
		}

		// ---- Telemetry (REQ-8) ----
		// Resolve the AppDir and the final endpoint value (flag > env > YAML).
		appDir, appDirErr := configpkg.AppDir()
		if appDirErr == nil {
			registryPath, _ := configpkg.RegistryPath()
			cfg, _ := configpkg.LoadTelemetryConfigOrDefault(registryPath)
			resolvedEndpoint := telemetrypkg.ResolveEndpoint(
				telemetryEndpoint,
				os.Getenv("SKILL_ORGANIZER_TELEMETRY_ENDPOINT"), // env override (flag > env > YAML precedence)
				cfg.Endpoint,
			)
			// The Service is constructed and stored on the command's
			// Context so the PersistentPostRun can pick it up. We use
			// a custom context-key type to avoid collisions.
			svc, svcErr := telemetrypkg.New(appDir, version, telemetrypkg.TelemetryConfig{Enabled: cfg.Enabled, Endpoint: resolvedEndpoint})
			if svcErr == nil {
				svc.MaybeRunFirstRunPrompt(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), func(yes bool) error {
					return configpkg.SaveTelemetryConfig(registryPath, telemetrypkg.TelemetryConfig{Enabled: yes, Endpoint: resolvedEndpoint})
				})
				_ = svc.DrainBuffer(cmd.Context())
				cmd.SetContext(withTelemetryService(cmd.Context(), svc))
			}
		}
	}
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		if cmd == rootCmd {
			return
		}
		if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "telemetry" {
			return
		}
		svc, ok := telemetryServiceFromContext(cmd.Context())
		if !ok {
			return
		}
		// Cobra does not call PersistentPostRun when RunE returns an
		// error (it short-circuits the post-run path on error). For
		// the success case the exit status is 0; for the error case
		// PersistentPostRun is skipped entirely, so this hook fires
		// only on the success path. exit_status = 0 is correct for
		// the success case.
		exitStatus := 0
		_ = svc.RecordEvent(cmd.Context(), telemetrypkg.NormalizeCommandName(cmd.Name()), exitStatus)
	}
	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cliHelpHeader())
		defaultHelpFunc(cmd, args)
	})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.AddCommand(newSyncCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newAboutCommand())
	rootCmd.AddCommand(newCompletionCommand())
	rootCmd.AddCommand(newOnboardCommand())
	rootCmd.AddCommand(newProjectCommand())
	skillCmd := newSkillCommand()
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(newWatchedCommand())
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newServiceCommand())
	rootCmd.AddCommand(newSelfUpdateCommand())
	rootCmd.AddCommand(newTelemetryCommand())

	for _, child := range skillCmd.Commands() {
		switch child.Name() {
		case "add", "delete", "enable", "disable", "check-updates":
			use := child.Use
			if fields := strings.Fields(use); len(fields) > 0 {
				addRootAlias(fields[0], child)
			}
		}
	}

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}

// ctxKey is the unexported type used for the PersistentPreRun/PostRun
// context plumbing. The two keys (telemetry service, run error) avoid
// collisions with other packages that may also use context values.
type ctxKey int

const (
	ctxKeyTelemetry ctxKey = iota
)

// withTelemetryService attaches a *telemetrypkg.Service to the given
// context so PersistentPostRun can pick it up.
func withTelemetryService(ctx context.Context, svc *telemetrypkg.Service) context.Context {
	return context.WithValue(ctx, ctxKeyTelemetry, svc)
}

// telemetryServiceFromContext returns the *telemetrypkg.Service
// previously attached via withTelemetryService, or (nil, false) if
// no Service is in the context.
func telemetryServiceFromContext(ctx context.Context) (*telemetrypkg.Service, bool) {
	svc, ok := ctx.Value(ctxKeyTelemetry).(*telemetrypkg.Service)
	return svc, ok
}
