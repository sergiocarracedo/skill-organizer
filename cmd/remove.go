package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove a managed project configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, _, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			ok, err := confirm(fmt.Sprintf("Remove project config %s?", configFile), false)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("aborted")
			}

			if err := os.Remove(configFile); err != nil {
				return fmt.Errorf("remove project config: %w", err)
			}

			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}
			registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
			if err != nil {
				return err
			}
			if registry.Remove(configFile) {
				if err := configpkg.SaveRegistry(registryPath, registry); err != nil {
					return err
				}
			}

			pterm.Success.Printfln("Removed project config: %s", configFile)
			return nil
		},
	}
}
