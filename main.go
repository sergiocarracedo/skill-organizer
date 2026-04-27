package main

import (
	"os"

	"github.com/pterm/pterm"
	"github.com/sergiocarracedo/skill-organizer/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		pterm.Error.Printfln("%v", err)
		os.Exit(1)
	}
}
