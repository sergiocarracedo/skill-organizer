package cmd

import (
	"fmt"
	"sort"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/mover"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

const toggleAllOption = "[Toggle all]"

func newMoveUnmanagedCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "move-unmanaged",
		Short: "Move unmanaged target skills into the organized source tree",
		Long:  "Move unmanaged target skills into the organized source tree.\n\nIn interactive mode, use the arrow keys to move, space to toggle selection, enter to continue, and [Toggle all] to select or clear every unmanaged entry.",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			moves, err := mover.Plan(location)
			if err != nil {
				return err
			}
			if len(moves) == 0 {
				pterm.Info.Printfln("No unmanaged target entries found")
				return nil
			}

			selectedMoves := moves
			if !yes {
				selectedMoves, err = chooseUnmanagedMoves(moves)
				if err != nil {
					return err
				}
				if len(selectedMoves) == 0 {
					pterm.Info.Println("No unmanaged target entries selected")
					return nil
				}
			}

			if err := mover.Apply(selectedMoves); err != nil {
				return err
			}

			pterm.Success.Printfln("Moved %d unmanaged target entries", len(selectedMoves))

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}
			printSyncResult(configFile, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply unmanaged moves without confirmation")
	return cmd
}

func chooseUnmanagedMoves(moves []mover.Move) ([]mover.Move, error) {
	return chooseUnmanagedMovesWithDefaults(moves, nil)
}

func chooseUnmanagedMovesWithDefaults(moves []mover.Move, defaultSelected []string) ([]mover.Move, error) {
	selected := make(map[string]bool, len(moves))
	for _, name := range defaultSelected {
		selected[name] = true
	}
	options := make([]string, 0, len(moves)+1)
	options = append(options, toggleAllOption)
	for _, move := range moves {
		options = append(options, move.Name)
	}

	for {
		defaults := selectedMoveNames(selected)
		choices, err := selectMultiple(fmt.Sprintf("Select unmanaged target entries to move (%d found)", len(moves)), options, defaults)
		if err != nil {
			return nil, err
		}

		if includesOption(choices, toggleAllOption) {
			setAllSelections(selected, moves, !allMovesSelected(selected, moves))
			continue
		}

		selected = make(map[string]bool, len(moves))
		for _, choice := range choices {
			selected[choice] = true
		}
		break
	}

	filtered := make([]mover.Move, 0, len(selected))
	for _, move := range moves {
		if selected[move.Name] {
			filtered = append(filtered, move)
		}
	}

	return filtered, nil
}

func selectedMoveNames(selected map[string]bool) []string {
	if len(selected) == 0 {
		return nil
	}

	result := make([]string, 0, len(selected))
	for name, enabled := range selected {
		if enabled {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func includesOption(options []string, target string) bool {
	for _, option := range options {
		if option == target {
			return true
		}
	}
	return false
}

func allMovesSelected(selected map[string]bool, moves []mover.Move) bool {
	if len(moves) == 0 {
		return false
	}
	for _, move := range moves {
		if !selected[move.Name] {
			return false
		}
	}
	return true
}

func setAllSelections(selected map[string]bool, moves []mover.Move, enabled bool) {
	for key := range selected {
		delete(selected, key)
	}
	if !enabled {
		return
	}
	for _, move := range moves {
		selected[move.Name] = true
	}
}
