package cmd

import "github.com/spf13/cobra"

func newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage skill-organizer projects",
	}

	cmd.AddCommand(newAddCommand())
	cmd.AddCommand(newEditCommand())
	cmd.AddCommand(newRemoveCommand())

	return cmd
}
