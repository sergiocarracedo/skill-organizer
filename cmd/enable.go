package cmd

import (
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <source-path>",
		Short: "Enable a source skill by source path",
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

			if err := skills.RewriteManagedFields(skill, false, false); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Enabled skill: %s", skill.RelativePath)
			printSyncResult(configFile, result)
			return nil
		},
	}
}
