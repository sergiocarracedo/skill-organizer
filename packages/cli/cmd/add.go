package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func newAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Interactively add a managed project",
		RunE: func(_ *cobra.Command, _ []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}

			candidates, err := configpkg.CandidateTargets(wd)
			if err != nil {
				return err
			}

			target, err := chooseTarget(wd, candidates)
			if err != nil {
				return err
			}

			if info, err := os.Stat(target); err != nil || !info.IsDir() {
				if err != nil {
					return fmt.Errorf("target folder is not accessible: %w", err)
				}
				return fmt.Errorf("target path %q is not a directory", target)
			}

			source, err := promptPath("Select the source skills-organized folder", configpkg.DefaultSourceForTarget(target))
			if err != nil {
				return err
			}

			location := configpkg.Location{Source: source, Target: target}
			if err := location.Validate(); err != nil {
				return err
			}

			configFile := configpkg.ConfigPathForTarget(target)
			if _, err := os.Stat(configFile); err == nil {
				overwrite, err := confirm(fmt.Sprintf("A project config already exists at %s. Overwrite it?", configFile), false)
				if err != nil {
					return err
				}
				if !overwrite {
					return fmt.Errorf("aborted")
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("check existing config: %w", err)
			}

			if err := configpkg.SaveLocation(configFile, location); err != nil {
				return err
			}

			pterm.Success.Printfln("Created project config: %s", configFile)

			watch, err := confirm("Watch this project?", true)
			if err != nil {
				return err
			}

			if watch {
				registryPath, err := configpkg.RegistryPath()
				if err != nil {
					return err
				}

				registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
				if err != nil {
					return err
				}

				registry.Add(configFile)
				if err := configpkg.SaveRegistry(registryPath, registry); err != nil {
					return err
				}

				pterm.Success.Printfln("Registered watched config: %s", configFile)
			}

			pterm.Info.Printfln("Source: %s", source)
			pterm.Info.Printfln("Target: %s", target)
			return nil
		},
	}
}
