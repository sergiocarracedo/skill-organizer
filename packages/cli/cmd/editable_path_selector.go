package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

const activeSelectorPrefix = "🡲 "

type editablePathSelectorMode string

const (
	editablePathModeNavigate editablePathSelectorMode = "navigate"
	editablePathModeEdit     editablePathSelectorMode = "edit"
)

type editablePathSelectorItem struct {
	Key            string
	Label          string
	Parent         string
	Selected       bool
	AlwaysSelected bool
}

type editablePathSelectorOptions struct {
	Intro          string
	BasePathLabel  string
	ShowToggleAll  bool
	ShowCheckboxes bool
	HelpNavigate   string
	HelpEdit       string
	ToggleAllLabel string
}

type editablePathSelector struct {
	items           []editablePathSelectorItem
	suggestions     []string
	activeIndex     int
	mode            editablePathSelectorMode
	editState       editableInputState
	editOriginal    string
	editEscapeSeq   string
	lastRenderLines int
	lastCursorLine  int
	options         editablePathSelectorOptions
	parents         map[string]string
	selected        map[string]bool
}

func newEditablePathSelector(items []editablePathSelectorItem, suggestions []string, options editablePathSelectorOptions) *editablePathSelector {
	selected := make(map[string]bool, len(items))
	parents := make(map[string]string, len(items))
	for _, item := range items {
		parents[item.Key] = strings.TrimSpace(item.Parent)
		selected[item.Key] = item.AlwaysSelected || item.Selected
	}
	if strings.TrimSpace(options.BasePathLabel) == "" {
		options.BasePathLabel = "organized-skills/"
	}
	if strings.TrimSpace(options.HelpNavigate) == "" {
		if options.ShowCheckboxes {
			options.HelpNavigate = "Space: Toggle, 🡩/🡫: Move, 🡪: Edit folder, Enter: Continue"
		} else {
			options.HelpNavigate = "🡩/🡫: Move, 🡪: Edit folder, Enter: Continue"
		}
	}
	if strings.TrimSpace(options.HelpEdit) == "" {
		options.HelpEdit = "Edit mode: Type, Tab autocomplete, 🡨/🡪 move cursor, Home/End, Enter save, Esc cancel"
	}
	if strings.TrimSpace(options.ToggleAllLabel) == "" {
		options.ToggleAllLabel = "Toggle all"
	}
	return &editablePathSelector{
		items:       items,
		suggestions: suggestions,
		activeIndex: 0,
		mode:        editablePathModeNavigate,
		options:     options,
		parents:     parents,
		selected:    selected,
	}
}

func (s *editablePathSelector) Run() error {
	defer showTerminalCursor()
	if strings.TrimSpace(s.options.Intro) != "" {
		if _, err := fmt.Fprintln(os.Stdout, s.options.Intro); err != nil {
			return fmt.Errorf("print selector intro: %w", err)
		}
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			return fmt.Errorf("print selector spacing: %w", err)
		}
	}
	s.render()
	err := keyboard.Listen(func(key keys.Key) (bool, error) {
		switch s.mode {
		case editablePathModeNavigate:
			return s.handleNavigateKey(key)
		case editablePathModeEdit:
			return s.handleEditKey(key)
		default:
			return false, nil
		}
	})
	if err != nil {
		return fmt.Errorf("run path selector: %w", err)
	}
	return nil
}

func (s *editablePathSelector) ParentFor(key string) string {
	return strings.TrimSpace(s.parents[key])
}

func (s *editablePathSelector) SelectedKeys() []string {
	results := make([]string, 0, len(s.items))
	for _, item := range s.items {
		if item.AlwaysSelected || s.selected[item.Key] {
			results = append(results, item.Key)
		}
	}
	return results
}

func (s *editablePathSelector) handleNavigateKey(key keys.Key) (bool, error) {
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
		if s.activeIndex < s.maxActiveIndex() {
			s.activeIndex++
			s.render()
		}
		return false, nil
	case keys.Space:
		if !s.options.ShowCheckboxes {
			return false, nil
		}
		if s.options.ShowToggleAll && s.activeIndex == 0 {
			enable := !s.allSelected()
			s.setAllSelections(enable)
		} else {
			item := s.activeItem()
			if item == nil || item.AlwaysSelected {
				return false, nil
			}
			s.selected[item.Key] = !s.selected[item.Key]
		}
		s.render()
		return false, nil
	case keys.Right:
		item := s.activeItem()
		if item == nil {
			return false, nil
		}
		s.mode = editablePathModeEdit
		s.editOriginal = s.parents[item.Key]
		s.editState.setValue(s.editOriginal)
		s.render()
		return false, nil
	default:
		return false, nil
	}
}

func (s *editablePathSelector) handleEditKey(key keys.Key) (bool, error) {
	if handled, stop, err := s.handleEditEscapeKey(key); handled {
		return stop, err
	}
	item := s.activeItem()
	if item == nil {
		return false, nil
	}
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
		s.parents[item.Key] = strings.TrimSpace(s.editState.String())
		s.mode = editablePathModeNavigate
		s.editEscapeSeq = ""
		s.render()
		return false, nil
	case keys.Escape:
		s.parents[item.Key] = s.editOriginal
		s.mode = editablePathModeNavigate
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

func (s *editablePathSelector) handleEditEscapeKey(key keys.Key) (bool, bool, error) {
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

func (s *editablePathSelector) moveEditSelection(delta int) {
	current := s.activeItem()
	if current == nil {
		return
	}
	s.parents[current.Key] = strings.TrimSpace(s.editState.String())
	next := s.activeIndex + delta
	minIndex := 0
	if s.options.ShowToggleAll {
		minIndex = 1
	}
	if next < minIndex {
		next = minIndex
	}
	if next > s.maxActiveIndex() {
		next = s.maxActiveIndex()
	}
	s.activeIndex = next
	nextItem := s.activeItem()
	if nextItem == nil {
		return
	}
	s.editOriginal = s.parents[nextItem.Key]
	s.editState.setValue(s.editOriginal)
	s.editEscapeSeq = ""
	s.render()
}

func (s *editablePathSelector) render() {
	lines := s.lines()
	if s.mode == editablePathModeEdit {
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
	if s.mode == editablePathModeEdit {
		activeLineIndex := s.activeLineIndex()
		linesUp := len(lines) - activeLineIndex
		if linesUp > 0 {
			fmt.Printf("\033[%dA", linesUp)
		}
		fmt.Print("\r")
		item := s.activeItem()
		linePrefix := s.editLinePrefix(item, true)
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

func (s *editablePathSelector) lines() []string {
	lines := []string{selectorHelpLine(s.mode, s.options)}
	if s.options.ShowToggleAll {
		lines = append(lines, selectorToggleRow(s.activeIndex == 0, s.options.ToggleAllLabel))
	}
	for i := range s.items {
		item := &s.items[i]
		active := s.activeLineIndexForItem(i) == s.activeIndex
		if active && s.mode == editablePathModeEdit {
			lines = append(lines, s.editLinePrefix(item, true)+s.editState.String())
			continue
		}
		lines = append(lines, s.navigationLine(item, active))
	}
	return lines
}

func selectorHelpLine(mode editablePathSelectorMode, options editablePathSelectorOptions) string {
	baseStyle := pterm.NewStyle(pterm.FgDarkGray)
	keyStyle := pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	replacer := strings.NewReplacer(
		"🡨/🡪", keyStyle.Sprint("🡨/🡪"),
		"🡩/🡫", keyStyle.Sprint("🡩/🡫"),
		"🡨", keyStyle.Sprint("🡨"),
		"🡪", keyStyle.Sprint("🡪"),
		"🡩", keyStyle.Sprint("🡩"),
		"🡫", keyStyle.Sprint("🡫"),
		"🡬", keyStyle.Sprint("🡬"),
		"🡭", keyStyle.Sprint("🡭"),
		"🡮", keyStyle.Sprint("🡮"),
		"🡯", keyStyle.Sprint("🡯"),
		"Right", keyStyle.Sprint("Right"),
		"Enter", keyStyle.Sprint("Enter"),
		"Space", keyStyle.Sprint("Space"),
		"Tab", keyStyle.Sprint("Tab"),
		"Home/End", keyStyle.Sprint("Home/End"),
		"Esc", keyStyle.Sprint("Esc"),
	)
	if mode == editablePathModeEdit {
		return replacer.Replace(baseStyle.Sprint(options.HelpEdit))
	}
	return replacer.Replace(baseStyle.Sprint(options.HelpNavigate))
}

func selectorToggleRow(active bool, label string) string {
	prefix := "  "
	if active {
		prefix = activeSelectorPrefix
	}
	return prefix + pterm.NewStyle(pterm.FgLightWhite, pterm.Bold).Sprint(label)
}

func (s *editablePathSelector) navigationLine(item *editablePathSelectorItem, active bool) string {
	prefix := "  "
	if active {
		prefix = activeSelectorPrefix
	}
	marker := s.selectionMarker(item)
	label := pterm.NewStyle(pterm.FgLightMagenta, pterm.Bold).Sprint(item.Label)
	path := s.renderBasePath(s.parents[item.Key])
	if marker != "" {
		return fmt.Sprintf("%s%s %s -> %s", prefix, marker, label, path)
	}
	return fmt.Sprintf("%s%s -> %s", prefix, label, path)
}

func (s *editablePathSelector) editLinePrefix(item *editablePathSelectorItem, active bool) string {
	prefix := "  "
	if active {
		prefix = activeSelectorPrefix
	}
	marker := s.selectionMarker(item)
	label := pterm.NewStyle(pterm.FgLightMagenta, pterm.Bold).Sprint(item.Label)
	base := pterm.NewStyle(pterm.FgDarkGray).Sprint(s.options.BasePathLabel)
	if marker != "" {
		return fmt.Sprintf("%s%s %s -> %s", prefix, marker, label, base)
	}
	return fmt.Sprintf("%s%s -> %s", prefix, label, base)
}

func (s *editablePathSelector) selectionMarker(item *editablePathSelectorItem) string {
	if !s.options.ShowCheckboxes {
		return ""
	}
	return styledSelectionMarker(item.AlwaysSelected || s.selected[item.Key])
}

func styledSelectionMarker(selected bool) string {
	marker := "○"
	if selected {
		marker = "◉"
	}
	return pterm.NewStyle(pterm.FgGreen).Sprint(marker)
}

func (s *editablePathSelector) renderBasePath(parent string) string {
	parent = strings.TrimSpace(parent)
	base := s.options.BasePathLabel
	baseStyled := pterm.NewStyle(pterm.FgDarkGray).Sprint(base)
	if parent == "" {
		return baseStyled
	}
	child := pterm.NewStyle(pterm.FgLightWhite).Sprint(filepath.ToSlash(parent) + "/")
	if strings.HasSuffix(base, "/") {
		return baseStyled + child
	}
	return pterm.NewStyle(pterm.FgDarkGray).Sprint(base+"/") + child
}

func (s *editablePathSelector) activeItem() *editablePathSelectorItem {
	index := s.activeItemIndex()
	if index < 0 || index >= len(s.items) {
		return nil
	}
	return &s.items[index]
}

func (s *editablePathSelector) activeItemIndex() int {
	index := s.activeIndex
	if s.options.ShowToggleAll {
		index--
	}
	return index
}

func (s *editablePathSelector) activeLineIndex() int {
	return s.activeIndex + 1
}

func (s *editablePathSelector) activeLineIndexForItem(itemIndex int) int {
	index := itemIndex
	if s.options.ShowToggleAll {
		index++
	}
	return index
}

func (s *editablePathSelector) maxActiveIndex() int {
	if len(s.items) == 0 {
		return 0
	}
	max := len(s.items) - 1
	if s.options.ShowToggleAll {
		max++
	}
	return max
}

func (s *editablePathSelector) allSelected() bool {
	if len(s.items) == 0 {
		return false
	}
	for _, item := range s.items {
		if item.AlwaysSelected {
			continue
		}
		if !s.selected[item.Key] {
			return false
		}
	}
	return true
}

func (s *editablePathSelector) setAllSelections(enabled bool) {
	for _, item := range s.items {
		if item.AlwaysSelected {
			continue
		}
		s.selected[item.Key] = enabled
	}
}
