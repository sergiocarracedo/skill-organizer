package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSkillAuditCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "audit <source-relative-path>",
		Short: "Show audit results for an installed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			_, doc, err := resolveSkillBySourcePath(location, args[0])
			if err != nil {
				return err
			}

			metadata := doc.RemoteMetadata()
			if metadata.Provider == "" {
				return fmt.Errorf("skill %q is not managed by a remote provider", args[0])
			}

			service, err := newRemoteService()
			if err != nil {
				return err
			}
			provider, err := service.Manager().Provider(metadata.Provider)
			if err != nil {
				return err
			}

			report, err := service.Audit(provider, remoteSummaryFromMetadata(doc.Name(), metadata))
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(report)
			}
			renderAuditReport(report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print audit results as JSON")
	return cmd
}
