package cmd

import "github.com/spf13/cobra"

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage source skills. Enable, disable, or move unmanaged skills.",
	}

	cmd.AddCommand(newEnableCommand())
	cmd.AddCommand(newDisableCommand())
	cmd.AddCommand(newSkillAddCommand())
	cmd.AddCommand(newSkillDeleteCommand())
	cmd.AddCommand(newMoveUnmanagedCommand())
	cmd.AddCommand(newCheckUpdatesCommand())
	cmd.AddCommand(newTryFindMetadataCommand())
	cmd.AddCommand(newCheckOverlapCommand())
	cmd.AddCommand(newCheckSecurityCommand())

	return cmd
}
