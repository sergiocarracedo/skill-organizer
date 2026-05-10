package cmd

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSkillDeleteCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "delete <source-path>",
		Aliases: []string{"remove", "rm"},
		Short:   "Delete a managed source skill by moving it into the .old backup area",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			skill, err := skills.ResolveSourceSkill(location.Source, args[0])
			if err != nil {
				return err
			}

			if !yes {
				ok, err := confirm(fmt.Sprintf("Move skill %s to the .old backup area?", skill.RelativePath), false)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			backupPath, err := backuppkg.MoveSkill(skill.Dir, skill.FlattenedName, backuppkg.Metadata{
				OriginalPath:  skill.RelativePath,
				FlattenedName: skill.FlattenedName,
				DeletedAt:     time.Now().UTC().Format(time.RFC3339),
			}, time.Now().UTC())
			if err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Moved skill to backup: %s", backupPath)
			printSyncResult(configFile, result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Move the skill to backup without confirmation")
	return cmd
}
