package cmd

import (
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	loggingpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/logging"
	watchpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/watch"
)

func newWatchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch registered projects and keep them synchronized",
		RunE: func(_ *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			serviceConfig, err := configpkg.LoadServiceConfigOrDefault(registryPath)
			if err != nil {
				return err
			}

			runner, err := watchpkg.New(registryPath, loggingpkg.NewStd(serviceConfig.LogLevel))
			if err != nil {
				return err
			}
			defer runner.Close()

			pterm.Info.Printfln("Watching registered projects from %s", registryPath)
			return runner.Run()
		},
	}
}
