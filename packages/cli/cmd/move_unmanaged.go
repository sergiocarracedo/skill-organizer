package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	suggestions, err := sourceFolderSuggestions(location.Source)
	if err != nil {
		return nil, err
	}
	selector, err := newMoveUnmanagedSelector(location, moves, defaultSelected, suggestions)
	if err != nil {
		return nil, err
	}
	if err := selector.Run(); err != nil {
		return nil, err
	}
	return selector.SelectedMoves(), nil
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

type moveUnmanagedSelector struct {
	location configpkg.Location
	moves    []mover.Move
	selector *editablePathSelector
}

func newMoveUnmanagedSelector(location configpkg.Location, moves []mover.Move, defaultSelected []string, suggestions []string) (*moveUnmanagedSelector, error) {
	defaultMap := make(map[string]bool, len(defaultSelected))
	for _, name := range defaultSelected {
		defaultMap[name] = true
	}
	items := make([]editablePathSelectorItem, 0, len(moves))
	for _, move := range moves {
		defaultTarget, err := filepath.Rel(location.Source, move.Target)
		if err != nil {
			return nil, fmt.Errorf("compute default move target for %q: %w", move.Name, err)
		}
		defaultParent := filepath.ToSlash(filepath.Dir(defaultTarget))
		if defaultParent == "." {
			defaultParent = ""
		}
		items = append(items, editablePathSelectorItem{
			Key:      move.Name,
			Label:    move.Name,
			Parent:   defaultParent,
			Selected: defaultMap[move.Name],
		})
	}

	return &moveUnmanagedSelector{
		location:    location,
		moves:       moves,
		selector: newEditablePathSelector(items, suggestions, editablePathSelectorOptions{
			Intro:          "Select the skills to move. You can set the target path if you want.",
			BasePathLabel:  "organized-skills/",
			ShowToggleAll:  true,
			ShowCheckboxes: true,
			ToggleAllLabel: "Toggle all",
		}),
	}, nil
}

func (s *moveUnmanagedSelector) Run() error {
	if err := s.selector.Run(); err != nil {
		return fmt.Errorf("select unmanaged moves: %w", err)
	}
	return nil
}

func (s *moveUnmanagedSelector) SelectedMoves() []mover.Move {
	filtered := make([]mover.Move, 0, len(s.moves))
	selectedKeys := make(map[string]struct{}, len(s.selector.SelectedKeys()))
	for _, key := range s.selector.SelectedKeys() {
		selectedKeys[key] = struct{}{}
	}
	for _, move := range s.moves {
		if _, ok := selectedKeys[move.Name]; !ok {
			continue
		}
		parent := s.selector.ParentFor(move.Name)
		relative := move.Name
		if parent != "" {
			relative = filepath.ToSlash(filepath.Join(parent, move.Name))
		}
		updatedMove, err := mover.SetRelativeTarget(s.location, move, relative)
		if err != nil {
			continue
		}
		filtered = append(filtered, updatedMove)
	}
	return filtered
}

func isEditEscapeSequencePrefix(value string) bool {
	known := []string{"[A", "[B", "[C", "[D", "[H", "[F", "[1~", "[4~", "[7~", "[8~", "OA", "OB", "OC", "OD", "OH", "OF"}
	for _, candidate := range known {
		if strings.HasPrefix(candidate, value) {
			return true
		}
	}
	return false
}

func hideTerminalCursor() {
	fmt.Fprint(os.Stdout, "\033[?25l")
}

func showTerminalCursor() {
	fmt.Fprint(os.Stdout, "\033[?25h")
}

func renderOrganizedPath(parent string) string {
	parent = strings.TrimSpace(parent)
	if parent == "" {
		return "organized-skills/"
	}
	return "organized-skills/" + filepath.ToSlash(parent) + "/"
}
