package cmd

import (
	"github.com/spf13/cobra"

	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

func newStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
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
			if err := statuspkg.AttachUpdates(&report); err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(statusJSON{ConfigPath: configFile, Source: location.Source, Target: location.Target, Report: report})
			}

			return printStatusReport(configFile, location, report)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print status as JSON")
	return cmd
}
