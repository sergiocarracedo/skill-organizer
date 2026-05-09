package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func newEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit a managed project configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			currentConfigPath, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			target, err := promptPath("Edit the target skills folder", location.Target)
			if err != nil {
				return err
			}
			source, err := promptPath("Edit the source skills-organized folder", location.Source)
			if err != nil {
				return err
			}

			updated := configpkg.Location{Source: source, Target: target}
			if err := updated.Validate(); err != nil {
				return err
			}

			newConfigPath := configpkg.ConfigPathForTarget(target)
			if err := configpkg.SaveLocation(newConfigPath, updated); err != nil {
				return err
			}

			if filepath.Clean(newConfigPath) != filepath.Clean(currentConfigPath) {
				if err := os.Remove(currentConfigPath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("remove old config file: %w", err)
				}

				registryPath, err := configpkg.RegistryPath()
				if err != nil {
					return err
				}
				registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
				if err != nil {
					return err
				}
				if registry.Remove(currentConfigPath) {
					registry.Add(newConfigPath)
					if err := configpkg.SaveRegistry(registryPath, registry); err != nil {
						return err
					}
				}
			}

			pterm.Success.Printfln("Updated project config: %s", newConfigPath)
			return nil
		},
	}
}
