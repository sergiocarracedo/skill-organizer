package cmd

import (
	selfupdatepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the skill-organizer CLI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return selfupdatepkg.Run(cmd.Context(), version, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}
