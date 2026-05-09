package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long:  "Generate shell completion scripts for bash, zsh, fish, or powershell.",
	}

	cmd.AddCommand(newCompletionBashCommand())
	cmd.AddCommand(newCompletionZshCommand())
	cmd.AddCommand(newCompletionFishCommand())
	cmd.AddCommand(newCompletionPowerShellCommand())

	return cmd
}

func newCompletionBashCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !noDescriptions)
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Disable completion descriptions")
	return cmd
}

func newCompletionZshCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				cmd.Root().CompletionOptions.DisableDescriptions = true
				defer func() {
					cmd.Root().CompletionOptions.DisableDescriptions = false
				}()
			}
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
		Example: "skill-organizer completion zsh > ~/.zsh/completions/_skill-organizer",
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Disable completion descriptions")
	return cmd
}

func newCompletionFishCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !noDescriptions)
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Disable completion descriptions")
	return cmd
}

func newCompletionPowerShellCommand() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:     "powershell",
		Aliases: []string{"ps"},
		Short:   "Generate PowerShell completions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				cmd.Root().CompletionOptions.DisableDescriptions = true
				defer func() {
					cmd.Root().CompletionOptions.DisableDescriptions = false
				}()
			}
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		},
		Example: fmt.Sprintf("%s\n%s",
			"skill-organizer completion powershell > skill-organizer.ps1",
			". ./skill-organizer.ps1",
		),
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Disable completion descriptions")
	return cmd
}
