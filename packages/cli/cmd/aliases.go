package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func addRootAlias(use string, target *cobra.Command) {
	alias := &cobra.Command{
		Use:                aliasUse(use, target.Use),
		Short:              target.Short,
		Long:               target.Long,
		Example:            target.Example,
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

func aliasUse(alias string, targetUse string) string {
	fields := strings.Fields(targetUse)
	if len(fields) == 0 {
		return alias
	}
	fields[0] = alias
	return strings.Join(fields, " ")
}
