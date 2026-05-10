package cmd

import "github.com/spf13/cobra"

func addRootAlias(use string, target *cobra.Command) {
	alias := &cobra.Command{
		Use:                use,
		Short:              target.Short,
		Args:               target.Args,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			target.SetArgs(args)
			target.SetContext(cmd.Context())
			target.SetIn(cmd.InOrStdin())
			target.SetOut(cmd.OutOrStdout())
			target.SetErr(cmd.ErrOrStderr())
			return target.RunE(target, args)
		},
	}
	alias.Flags().AddFlagSet(target.Flags())
	alias.PersistentFlags().AddFlagSet(target.PersistentFlags())
	rootCmd.AddCommand(alias)
}
