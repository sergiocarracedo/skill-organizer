package cmd

import "github.com/spf13/cobra"

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage source skills and remote skill installs.",
	}

	cmd.AddCommand(newSkillAddCommand())
	cmd.AddCommand(newSkillAuditCommand())
	cmd.AddCommand(newSkillUpdateCommand())
	cmd.AddCommand(newSkillRemoveCommand())
	cmd.AddCommand(newEnableCommand())
	cmd.AddCommand(newDisableCommand())
	cmd.AddCommand(newMoveUnmanagedCommand())
	cmd.AddCommand(newCheckOverlapCommand())

	return cmd
}
