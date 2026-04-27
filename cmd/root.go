package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"

	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "skill-organizer",
	Short: "Organize structured skill trees into flat tool-readable targets",
	Long:  "skill-organizer synchronizes organized source skill trees into flat target skills folders and manages watched skill projects.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to a project config file")
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("%s\n%s\ncommit %s, built %s\n", cliLogo(), version, commit, date))
	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cliHelpHeader())
		defaultHelpFunc(cmd, args)
	})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.AddCommand(newSyncCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newAboutCommand())
	rootCmd.AddCommand(newOnboardCommand())
	rootCmd.AddCommand(newProjectCommand())
	rootCmd.AddCommand(newSkillCommand())
	rootCmd.AddCommand(newWatchedCommand())
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newServiceCommand())

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}
