package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newAboutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "about",
		Short: "Show project and build information",
		RunE: func(_ *cobra.Command, _ []string) error {
			pterm.Println(cliLogo())
			pterm.Println(cliHeader())
			pterm.Println("Organize structured skill trees into flat tool-readable targets")
			pterm.Println()
			pterm.Println("GitHub: https://github.com/sergiocarracedo/skill-organizer")
			pterm.Println()
			pterm.Println("Author: Sergio Carracedo https://sergiocarracedo.es/")
			pterm.Println()
			pterm.Println(fmt.Sprintf("Version: %s", version))
			pterm.Println(fmt.Sprintf("Build date: %s", date))

			return nil
		},
	}
}
