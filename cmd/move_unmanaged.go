package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/mover"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

const toggleAllOption = "[Toggle all]"

func newMoveUnmanagedCommand() *cobra.Command {
	var yes bool
	var to string

	cmd := &cobra.Command{
		Use:   "move-unmanaged",
		Short: "Move unmanaged target skills into the organized source tree",
		Long:  "Move unmanaged target skills into the organized source tree.\n\nIn interactive mode, use the arrow keys to move, space to toggle selection, enter to continue, and [Toggle all] to select or clear every unmanaged entry. You can accept the default destination or enter a nested path such as 3rdparty/asciinema/asciinema-recorder.",
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
				selectedMoves, err = chooseUnmanagedMoves(location, moves)
				if err != nil {
					return err
				}
				if len(selectedMoves) == 0 {
					pterm.Info.Println("No unmanaged target entries selected")
					return nil
				}
			}

			if to != "" {
				if len(selectedMoves) != 1 {
					return fmt.Errorf("--to requires exactly one unmanaged target entry")
				}
				selectedMoves[0], err = mover.SetRelativeTarget(location, selectedMoves[0], to)
				if err != nil {
					return err
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
	cmd.Flags().StringVar(&to, "to", "", "Move a single unmanaged skill to a custom path relative to the source root")
	return cmd
}

func chooseUnmanagedMoves(location configpkg.Location, moves []mover.Move) ([]mover.Move, error) {
	return chooseUnmanagedMovesWithDefaults(location, moves, nil)
}

func chooseUnmanagedMovesWithDefaults(location configpkg.Location, moves []mover.Move, defaultSelected []string) ([]mover.Move, error) {
	selected := make(map[string]bool, len(moves))
	for _, name := range defaultSelected {
		selected[name] = true
	}
	if err := selectUnmanagedMoves(fmt.Sprintf("Select unmanaged target entries to move (%d found)", len(moves)), moves, selected); err != nil {
		return nil, err
	}

	filtered := make([]mover.Move, 0, len(selected))
	for _, move := range moves {
		if selected[move.Name] {
			updatedMove, err := promptUnmanagedMoveTarget(location, move)
			if err != nil {
				return nil, err
			}
			filtered = append(filtered, updatedMove)
		}
	}

	return filtered, nil
}

func promptUnmanagedMoveTarget(location configpkg.Location, move mover.Move) (mover.Move, error) {
	defaultTarget, err := filepath.Rel(location.Source, move.Target)
	if err != nil {
		return mover.Move{}, fmt.Errorf("compute default move target for %q: %w", move.Name, err)
	}
	defaultParent := filepath.ToSlash(filepath.Dir(defaultTarget))
	if defaultParent == "." {
		defaultParent = ""
	}

	suggestions, err := sourceFolderSuggestions(location.Source)
	if err != nil {
		return mover.Move{}, err
	}

	prompt := fmt.Sprintf("Parent destination for %s (skill folder name is kept)", move.Name)
	value, err := promptTextWithSuggestionsBelow(prompt, defaultParent, suggestions)
	if err != nil {
		return mover.Move{}, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = move.Name
	} else {
		value = filepath.ToSlash(filepath.Join(value, move.Name))
	}

	return mover.SetRelativeTarget(location, move, value)
}

func sourceFolderSuggestions(sourceRoot string) ([]string, error) {
	entries := []string{""}
	err := filepath.WalkDir(sourceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Clean(path) == filepath.Clean(sourceRoot) {
			return nil
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list source folders: %w", err)
	}

	sort.Strings(entries)
	return entries, nil
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

func selectUnmanagedMoves(prompt string, moves []mover.Move, selected map[string]bool) error {
	if _, err := fmt.Fprintln(os.Stdout, prompt); err != nil {
		return fmt.Errorf("print prompt: %w", err)
	}

	index := 0
	renderUnmanagedSelection(index, moves, selected)

	err := keyboard.Listen(func(key keys.Key) (bool, error) {
		switch key.Code {
		case keys.CtrlC:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, fmt.Errorf("interrupted")
		case keys.Enter:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, nil
		case keys.Up:
			if index > 0 {
				index--
				renderUnmanagedSelection(index, moves, selected)
			}
			return false, nil
		case keys.Down:
			if index < len(moves) {
				index++
				renderUnmanagedSelection(index, moves, selected)
			}
			return false, nil
		case keys.Space:
			if index == 0 {
				enable := !allMovesSelected(selected, moves)
				setAllSelections(selected, moves, enable)
			} else {
				name := moves[index-1].Name
				selected[name] = !selected[name]
			}
			renderUnmanagedSelection(index, moves, selected)
			return false, nil
		default:
			return false, nil
		}
	})
	if err != nil {
		return fmt.Errorf("select unmanaged moves: %w", err)
	}

	return nil
}

func renderUnmanagedSelection(index int, moves []mover.Move, selected map[string]bool) {
	lines := []string{"> " + toggleAllOption + "  (space: toggle all, enter: continue)"}
	for _, move := range moves {
		marker := "[ ]"
		if selected[move.Name] {
			marker = "[x]"
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, move.Name))
	}

	for i := range lines {
		prefix := "  "
		if i == index {
			prefix = "> "
		}
		line := strings.TrimPrefix(lines[i], "> ")
		lines[i] = prefix + line
	}

	fmt.Print("\r\033[J")
	for i, line := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print(line)
	}
	if len(lines) > 0 {
		fmt.Printf("\033[%dA\r", len(lines)-1-index)
	}
}
