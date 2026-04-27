package cmd

import (
	"github.com/spf13/cobra"

	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show source, target, and sync status",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			report, err := statuspkg.Build(location)
			if err != nil {
				return err
			}

			return printStatusReport(configFile, location, report)
		},
	}
}
