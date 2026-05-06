package cmd

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/library"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSkillRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <source-relative-path>",
		Short: "Remove an installed skill by source-relative path",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			_, appConfig, err := loadAppConfigForSkills()
			if err != nil {
				return err
			}

			skill, _, err := resolveSkillBySourcePath(location, args[0])
			if err != nil {
				return err
			}

			ok, err := confirm(fmt.Sprintf("Move %s to organized-skills/.old?", skill.RelativePath), true)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("aborted")
			}

			if _, err := library.MoveToBackup(location.Source, skill, time.Now().UTC()); err != nil {
				return err
			}
			if err := library.GarbageCollectBackups(location.Source, appConfig.Backups.RetentionDays, time.Now().UTC()); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Moved %s to organized-skills/.old", skill.RelativePath)
			printSyncResult(configFile, result)
			return nil
		},
	}
}
