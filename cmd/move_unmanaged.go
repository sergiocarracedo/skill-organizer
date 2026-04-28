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

type moveUnmanagedMode string

const (
	moveModeNavigate moveUnmanagedMode = "navigate"
	moveModeEdit     moveUnmanagedMode = "edit"
)

type moveUnmanagedSelector struct {
	location        configpkg.Location
	moves           []mover.Move
	suggestions     []string
	selected        map[string]bool
	parents         map[string]string
	activeIndex     int
	mode            moveUnmanagedMode
	editState       editableInputState
	editOriginal    string
	editEscapeSeq   string
	lastRenderLines int
	lastCursorLine  int
}

func newMoveUnmanagedSelector(location configpkg.Location, moves []mover.Move, defaultSelected []string, suggestions []string) (*moveUnmanagedSelector, error) {
	selected := make(map[string]bool, len(moves))
	for _, name := range defaultSelected {
		selected[name] = true
	}
	parents := make(map[string]string, len(moves))
	for _, move := range moves {
		defaultTarget, err := filepath.Rel(location.Source, move.Target)
		if err != nil {
			return nil, fmt.Errorf("compute default move target for %q: %w", move.Name, err)
		}
		defaultParent := filepath.ToSlash(filepath.Dir(defaultTarget))
		if defaultParent == "." {
			defaultParent = ""
		}
		parents[move.Name] = defaultParent
	}

	return &moveUnmanagedSelector{
		location:    location,
		moves:       moves,
		suggestions: suggestions,
		selected:    selected,
		parents:     parents,
		activeIndex: 0,
		mode:        moveModeNavigate,
	}, nil
}

func (s *moveUnmanagedSelector) Run() error {
	defer showTerminalCursor()

	if _, err := fmt.Fprintln(os.Stdout, "Select the skills to move. You can set the target path if you want."); err != nil {
		return fmt.Errorf("print selector intro: %w", err)
	}
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return fmt.Errorf("print selector spacing: %w", err)
	}

	s.render()

	err := keyboard.Listen(func(key keys.Key) (bool, error) {
		switch s.mode {
		case moveModeNavigate:
			return s.handleNavigateKey(key)
		case moveModeEdit:
			return s.handleEditKey(key)
		default:
			return false, nil
		}
	})
	if err != nil {
		return fmt.Errorf("select unmanaged moves: %w", err)
	}

	return nil
}

func (s *moveUnmanagedSelector) SelectedMoves() []mover.Move {
	filtered := make([]mover.Move, 0, len(s.moves))
	for _, move := range s.moves {
		if !s.selected[move.Name] {
			continue
		}
		parent := strings.TrimSpace(s.parents[move.Name])
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

func (s *moveUnmanagedSelector) handleNavigateKey(key keys.Key) (bool, error) {
	switch key.Code {
	case keys.CtrlC:
		_, _ = fmt.Fprintln(os.Stdout)
		return true, fmt.Errorf("interrupted")
	case keys.Enter:
		_, _ = fmt.Fprintln(os.Stdout)
		return true, nil
	case keys.Up:
		if s.activeIndex > 0 {
			s.activeIndex--
			s.render()
		}
		return false, nil
	case keys.Down:
		if s.activeIndex < len(s.moves) {
			s.activeIndex++
			s.render()
		}
		return false, nil
	case keys.Space:
		if s.activeIndex == 0 {
			enable := !allMovesSelected(s.selected, s.moves)
			setAllSelections(s.selected, s.moves, enable)
		} else {
			name := s.moves[s.activeIndex-1].Name
			s.selected[name] = !s.selected[name]
		}
		s.render()
		return false, nil
	case keys.Right:
		if s.activeIndex == 0 {
			return false, nil
		}
		move := s.moves[s.activeIndex-1]
		s.mode = moveModeEdit
		s.editOriginal = s.parents[move.Name]
		s.editState.setValue(s.editOriginal)
		s.render()
		return false, nil
	default:
		return false, nil
	}
}

func (s *moveUnmanagedSelector) handleEditKey(key keys.Key) (bool, error) {
	if handled, stop, err := s.handleEditEscapeKey(key); handled {
		return stop, err
	}

	move := s.moves[s.activeIndex-1]
	switch key.Code {
	case keys.CtrlC:
		_, _ = fmt.Fprintln(os.Stdout)
		return true, fmt.Errorf("interrupted")
	case keys.Up:
		s.moveEditSelection(-1)
		return false, nil
	case keys.Down:
		s.moveEditSelection(1)
		return false, nil
	case keys.Enter:
		s.parents[move.Name] = strings.TrimSpace(s.editState.String())
		s.mode = moveModeNavigate
		s.editEscapeSeq = ""
		s.render()
		return false, nil
	case keys.Escape:
		s.parents[move.Name] = s.editOriginal
		s.mode = moveModeNavigate
		s.editEscapeSeq = ""
		s.render()
		return false, nil
	case keys.Tab:
		s.editEscapeSeq = ""
		s.editState = autocompleteSuggestionAtCursor(s.editState, s.suggestions)
		s.render()
		return false, nil
	case keys.Left:
		s.editEscapeSeq = ""
		s.editState.moveLeft()
		s.render()
		return false, nil
	case keys.Right:
		s.editEscapeSeq = ""
		s.editState.moveRight()
		s.render()
		return false, nil
	case keys.Home:
		s.editEscapeSeq = ""
		s.editState.moveHome()
		s.render()
		return false, nil
	case keys.End:
		s.editEscapeSeq = ""
		s.editState.moveEnd()
		s.render()
		return false, nil
	case keys.Delete:
		s.editEscapeSeq = ""
		s.editState.deleteAtCursor()
		s.render()
		return false, nil
	case keys.Backspace:
		s.editEscapeSeq = ""
		s.editState.deleteBeforeCursor()
		s.render()
		return false, nil
	case keys.Space:
		s.editEscapeSeq = ""
		s.editState.insertRunes([]rune{' '})
		s.render()
		return false, nil
	case keys.RuneKey:
		s.editEscapeSeq = ""
		s.editState.insertRunes(key.Runes)
		s.render()
		return false, nil
	default:
		return false, nil
	}
}

func (s *moveUnmanagedSelector) handleEditEscapeKey(key keys.Key) (bool, bool, error) {
	if key.Code == keys.RuneKey && key.AltPressed && len(key.Runes) == 1 {
		switch key.Runes[0] {
		case '[', 'O':
			s.editEscapeSeq = string(key.Runes[0])
			return true, false, nil
		}
	}

	if s.editEscapeSeq == "" || key.Code != keys.RuneKey || len(key.Runes) != 1 {
		return false, false, nil
	}

	s.editEscapeSeq += string(key.Runes[0])
	switch s.editEscapeSeq {
	case "[A", "OA":
		s.editEscapeSeq = ""
		s.moveEditSelection(-1)
		return true, false, nil
	case "[B", "OB":
		s.editEscapeSeq = ""
		s.moveEditSelection(1)
		return true, false, nil
	case "[C", "OC":
		s.editEscapeSeq = ""
		s.editState.moveRight()
		s.render()
		return true, false, nil
	case "[D", "OD":
		s.editEscapeSeq = ""
		s.editState.moveLeft()
		s.render()
		return true, false, nil
	case "[H", "OH", "[1~", "[7~":
		s.editEscapeSeq = ""
		s.editState.moveHome()
		s.render()
		return true, false, nil
	case "[F", "OF", "[4~", "[8~":
		s.editEscapeSeq = ""
		s.editState.moveEnd()
		s.render()
		return true, false, nil
	}

	if isEditEscapeSequencePrefix(s.editEscapeSeq) {
		return true, false, nil
	}

	s.editEscapeSeq = ""
	return true, false, nil
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

func (s *moveUnmanagedSelector) moveEditSelection(delta int) {
	if s.activeIndex <= 0 || s.activeIndex > len(s.moves) {
		return
	}

	current := s.moves[s.activeIndex-1]
	s.parents[current.Name] = strings.TrimSpace(s.editState.String())

	next := s.activeIndex + delta
	if next < 1 {
		next = 1
	}
	if next > len(s.moves) {
		next = len(s.moves)
	}

	s.activeIndex = next
	nextMove := s.moves[s.activeIndex-1]
	s.editOriginal = s.parents[nextMove.Name]
	s.editState.setValue(s.editOriginal)
	s.editEscapeSeq = ""
	s.render()
}

func (s *moveUnmanagedSelector) render() {
	lines := s.lines()
	if s.mode == moveModeEdit {
		showTerminalCursor()
	} else {
		hideTerminalCursor()
	}
	if s.lastRenderLines > 0 {
		fmt.Printf("\033[%dA", s.lastCursorLine)
	}
	fmt.Print("\r\033[J")
	for i, line := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print(line)
	}
	if len(lines) > 0 {
		fmt.Print("\n")
	}
	if s.mode == moveModeEdit {
		activeLineIndex := s.activeIndex + 1
		linesUp := len(lines) - activeLineIndex
		if linesUp > 0 {
			fmt.Printf("\033[%dA", linesUp)
		}
		fmt.Print("\r")
		linePrefix := editLinePrefix(s.moves[s.activeIndex-1].Name, s.selected[s.moves[s.activeIndex-1].Name], true)
		lineValue := s.editState.String()
		line := linePrefix + lineValue
		fmt.Printf("\r\033[K%s", line)
		cursorDelta := visibleRuneWidth(lineValue) - s.editState.cursor
		if cursorDelta > 0 {
			fmt.Printf("\033[%dD", cursorDelta)
		}
		s.lastCursorLine = activeLineIndex
	} else {
		s.lastCursorLine = len(lines)
	}
	s.lastRenderLines = len(lines)
}

func (s *moveUnmanagedSelector) lines() []string {
	lines := []string{
		selectorHelpLine(s.mode),
		selectorToggleRow(s.activeIndex == 0),
	}
	for i, move := range s.moves {
		active := s.activeIndex == i+1
		if active && s.mode == moveModeEdit {
			lines = append(lines, editLinePrefix(move.Name, s.selected[move.Name], true)+s.editState.String())
			continue
		}
		lines = append(lines, navigationLine(move.Name, s.selected[move.Name], active, s.parents[move.Name]))
	}
	return lines
}
func selectorHelpLine(mode moveUnmanagedMode) string {
	if mode == moveModeEdit {
		return "Edit mode: Type, Tab autocomplete, <-/-> move cursor, Home/End, Enter save, Esc cancel"
	}
	return "Space: Toggle, Up/Down: Move, Right: Edit folder, Enter: Continue"
}

func selectorToggleRow(active bool) string {
	prefix := "  "
	if active {
		prefix = "> "
	}
	return prefix + "Toggle all"
}

func navigationLine(name string, selected bool, active bool, parent string) string {
	prefix := "  "
	if active {
		prefix = "> "
	}
	marker := styledSelectionMarker(selected)
	return fmt.Sprintf("%s%s %s -> %s", prefix, marker, name, renderOrganizedPath(parent))
}

func editLinePrefix(name string, selected bool, active bool) string {
	prefix := "  "
	if active {
		prefix = "> "
	}
	marker := styledSelectionMarker(selected)
	return fmt.Sprintf("%s%s %s -> %s", prefix, marker, name, "organized-skills/")
}

func styledSelectionMarker(selected bool) string {
	marker := "○"
	if selected {
		marker = "◉"
	}
	return pterm.NewStyle(pterm.FgGreen).Sprint(marker)
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
