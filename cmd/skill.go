package cmd

import "github.com/spf13/cobra"

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage source skills. Enable, disable, or move unmanaged skills.",
	}

	cmd.AddCommand(newEnableCommand())
	cmd.AddCommand(newDisableCommand())
	cmd.AddCommand(newMoveUnmanagedCommand())

	return cmd
}
