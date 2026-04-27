package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func newWatchedCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watched",
		Short: "Manage watched project config paths",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List watched project config paths",
		RunE: func(_ *cobra.Command, _ []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
			if err != nil {
				return err
			}

			if len(registry.Watched) == 0 {
				pterm.Info.Printfln("No watched project configs registered")
				return nil
			}

			pterm.DefaultSection.Println("Watched project configs")
			for _, watched := range registry.Watched {
				pterm.Println(watched)
			}

			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add [config-path]",
		Short: "Register a project config path for watching",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var path string
			var err error
			if len(args) == 1 {
				path, err = configpkg.ResolvePath(args[0])
				if err != nil {
					return fmt.Errorf("resolve config path: %w", err)
				}
			} else {
				path, err = promptPath("Enter the project config path to watch", configPath)
				if err != nil {
					return err
				}
			}

			path = filepath.Clean(path)
			if _, err := configpkg.LoadLocation(path); err != nil {
				return fmt.Errorf("validate watched config path: %w", err)
			}

			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
			if err != nil {
				return err
			}

			registry.Add(path)
			if err := configpkg.SaveRegistry(registryPath, registry); err != nil {
				return err
			}

			pterm.Success.Printfln("Registered watched project config: %s", path)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove [config-path]",
		Short: "Remove a watched project config path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
			if err != nil {
				return err
			}

			if len(registry.Watched) == 0 {
				return fmt.Errorf("no watched configs are registered")
			}

			var path string
			if len(args) == 1 {
				path, err = configpkg.ResolvePath(args[0])
				if err != nil {
					return fmt.Errorf("resolve config path: %w", err)
				}
				path = filepath.Clean(path)
			} else {
				selection, err := selectOption("Select a watched project config to remove", registry.Watched, "")
				if err != nil {
					return err
				}
				path = filepath.Clean(selection)
			}

			ok, err := confirm(fmt.Sprintf("Remove watched project config %s?", path), false)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("aborted")
			}

			if !registry.Remove(path) {
				return fmt.Errorf("watched project config not found: %s", path)
			}

			if err := configpkg.SaveRegistry(registryPath, registry); err != nil {
				return err
			}

			pterm.Success.Printfln("Removed watched project config: %s", path)
			return nil
		},
	})

	return cmd
}
