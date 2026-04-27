package cmd

import (
	"github.com/spf13/cobra"

	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Synchronize managed skills into the target folder",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			printSyncResult(configFile, result)
			return nil
		},
	}
}
