package cmd

import (
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	loggingpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/logging"
	serviceinternal "github.com/sergiocarracedo/skill-organizer/cli/internal/service"
)

func newServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the background watcher service",
	}

	for _, name := range []string{"install", "start", "stop", "restart", "status", "uninstall"} {
		serviceName := name
		cmd.AddCommand(&cobra.Command{
			Use:   serviceName,
			Short: serviceCommandDescription(serviceName),
			RunE: func(_ *cobra.Command, _ []string) error {
				registryPath, err := configpkg.RegistryPath()
				if err != nil {
					return err
				}

				status, err := serviceinternal.Control(registryPath, serviceName)
				if err != nil {
					return err
				}

				pterm.Info.Printfln("Service %s: %s", serviceName, status)
				if serviceName == "stop" || serviceName == "restart" || serviceName == "uninstall" {
					serviceinternal.WaitForStopDelay()
				}
				return nil
			},
		})
	}

	logLevelCmd := &cobra.Command{
		Use:   "log-level",
		Short: "Show or update the service log level",
		RunE: func(_ *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			cfg, err := configpkg.LoadServiceConfigOrDefault(registryPath)
			if err != nil {
				return err
			}

			pterm.Info.Printfln("Service log level: %s", cfg.LogLevel)
			return nil
		},
	}

	logLevelCmd.AddCommand(&cobra.Command{
		Use:   "set <level>",
		Short: "Set the service log level",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := loggingpkg.ValidateLevel(args[0]); err != nil {
				return err
			}

			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			cfg := configpkg.ServiceConfig{LogLevel: loggingpkg.NormalizeLevel(args[0])}
			if err := configpkg.SaveServiceConfig(registryPath, cfg); err != nil {
				return err
			}

			pterm.Success.Printfln("Updated service log level: %s", cfg.LogLevel)
			pterm.Info.Println("Restart the service to apply the new log level.")
			return nil
		},
	})

	cmd.AddCommand(logLevelCmd)

	return cmd
}

func serviceCommandDescription(name string) string {
	switch name {
	case "install":
		return "Install the background watcher service"
	case "start":
		return "Start the background watcher service"
	case "stop":
		return "Stop the background watcher service"
	case "restart":
		return "Restart the background watcher service"
	case "status":
		return "Show background watcher service status"
	case "uninstall":
		return "Uninstall the background watcher service"
	default:
		return "Manage the background watcher service"
	}
}
