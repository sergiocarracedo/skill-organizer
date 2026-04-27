package cmd

import (
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <source-path>",
		Short: "Disable a source skill by source path",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			skill, err := skills.ResolveSourceSkill(location.Source, args[0])
			if err != nil {
				return err
			}

			if err := skills.RewriteManagedFields(skill, false, true); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Disabled skill: %s", skill.RelativePath)
			printSyncResult(configFile, result)
			return nil
		},
	}
}
