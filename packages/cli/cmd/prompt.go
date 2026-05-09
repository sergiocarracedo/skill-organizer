package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

const customPathOption = "Custom path"

func selectOption(prompt string, options []string, defaultOption string) (string, error) {
	printer := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithFilter(false)

	if defaultOption != "" {
		printer = printer.WithDefaultOption(defaultOption)
	}

	result, err := printer.Show(prompt)
	if err != nil {
		return "", fmt.Errorf("select option: %w", err)
	}

	return result, nil
}

func promptText(prompt string, defaultValue string) (string, error) {
	printer := pterm.DefaultInteractiveTextInput.WithDefaultValue(defaultValue)
	result, err := printer.Show(prompt)
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	return strings.TrimSpace(result), nil
}

func promptTextBelow(prompt string, defaultValue string) (string, error) {
	printer := pterm.DefaultInteractiveTextInput.WithDefaultValue(defaultValue)
	result, err := printer.Show(prompt + "\n")
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	return strings.TrimSpace(result), nil
}

func promptTextWithSuggestionsBelow(prompt string, defaultValue string, suggestions []string) (string, error) {
	cleanSuggestions := uniqueSortedSuggestions(suggestions)
	if len(cleanSuggestions) == 0 {
		return promptTextBelow(prompt, defaultValue)
	}

	if _, err := fmt.Fprintln(os.Stdout, prompt); err != nil {
		return "", fmt.Errorf("print prompt: %w", err)
	}

	state := editableInputState{value: []rune(strings.TrimSpace(defaultValue))}
	state.cursor = len(state.value)

	renderAutocompleteInput(state, cleanSuggestions)

	err := keyboard.Listen(func(key keys.Key) (bool, error) {
		switch key.Code {
		case keys.CtrlC:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, fmt.Errorf("interrupted")
		case keys.Enter:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, nil
		case keys.Tab:
			state.applyAutocomplete(cleanSuggestions)
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Left:
			state.moveLeft()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Right:
			state.moveRight()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Home:
			state.moveHome()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.End:
			state.moveEnd()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Delete:
			state.deleteAtCursor()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Backspace:
			state.deleteBeforeCursor()
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.Space:
			state.insertRunes([]rune{' '})
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		case keys.RuneKey:
			state.insertRunes(key.Runes)
			renderAutocompleteInput(state, cleanSuggestions)
			return false, nil
		default:
			return false, nil
		}
	})
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	return strings.TrimSpace(state.String()), nil
}

func confirm(prompt string, defaultValue bool) (bool, error) {
	result, err := pterm.DefaultInteractiveConfirm.WithDefaultValue(defaultValue).Show(prompt)
	if err != nil {
		return false, fmt.Errorf("confirm: %w", err)
	}

	return result, nil
}

func selectMultiple(prompt string, options []string, defaultOptions []string) ([]string, error) {
	printer := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultOptions(defaultOptions).
		WithFilter(false).
		WithKeySelect(keys.Space).
		WithKeyConfirm(keys.Enter)

	result, err := printer.Show(prompt)
	if err != nil {
		return nil, fmt.Errorf("select multiple options: %w", err)
	}

	return result, nil
}

func chooseTarget(initialRoot string, candidates []string) (string, error) {
	options := append([]string{}, candidates...)
	options = append(options, customPathOption)

	selection, err := selectOption("Select a target skills folder", options, "")
	if err != nil {
		return "", err
	}

	if selection == customPathOption {
		return promptPath("Enter the target skills folder", initialRoot)
	}

	return filepath.Clean(selection), nil
}

func promptPath(prompt string, defaultValue string) (string, error) {
	value, err := promptText(prompt, defaultValue)
	if err != nil {
		return "", err
	}

	return configpkg.ResolvePath(value)
}

func uniqueSortedSuggestions(suggestions []string) []string {
	seen := make(map[string]struct{}, len(suggestions))
	result := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		suggestion = strings.TrimSpace(suggestion)
		if suggestion == "" {
			continue
		}
		if _, ok := seen[suggestion]; ok {
			continue
		}
		seen[suggestion] = struct{}{}
		result = append(result, suggestion)
	}
	sort.Strings(result)
	return result
}

func firstSuggestion(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	return suggestions[0]
}

func autocompleteSuggestion(current string, suggestions []string) string {
	trimmed := strings.TrimSpace(current)
	if trimmed == "" {
		return firstSuggestion(suggestions)
	}

	for _, suggestion := range suggestions {
		if strings.HasPrefix(suggestion, trimmed) && suggestion != trimmed {
			return suggestion
		}
	}

	return current
}

func autocompleteSuggestionAtCursor(state editableInputState, suggestions []string) editableInputState {
	current := state.String()
	start, end := pathTokenBounds(current, state.cursor)
	prefix := strings.TrimSpace(string([]rune(current)[start:state.cursor]))
	if prefix == "" {
		if len(suggestions) == 0 {
			return state
		}
		prefix = firstSuggestion(suggestions)
	} else {
		prefix = autocompleteSuggestion(prefix, suggestions)
	}

	before := string([]rune(current)[:start])
	after := string([]rune(current)[end:])
	updated := editableInputState{}
	updated.setValue(before + prefix + after)
	updated.cursor = len([]rune(before + prefix))
	return updated
}

func pathTokenBounds(current string, cursor int) (int, int) {
	runes := []rune(current)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	start := cursor
	for start > 0 && runes[start-1] != ' ' {
		start--
	}

	end := cursor
	for end < len(runes) && runes[end] != ' ' {
		end++
	}

	return start, end
}

func renderAutocompleteInput(state editableInputState, suggestions []string) {
	hint := ""
	current := state.String()
	trimmed := strings.TrimSpace(current)
	for _, suggestion := range suggestions {
		if strings.HasPrefix(suggestion, trimmed) && suggestion != trimmed {
			hint = suggestion
			break
		}
	}

	line := "> " + current
	if hint != "" {
		line += pterm.NewStyle(pterm.FgDarkGray).Sprint("  tab -> " + hint)
		line += pterm.NewStyle(pterm.FgDefault).Sprint("")
	}
	fmt.Printf("\r\033[K%s", line)
	if hint != "" {
		fmt.Printf("\033[%dD", visibleRuneWidth(hint)+9)
	}
	cursorDelta := visibleRuneWidth(current) - state.cursor
	if cursorDelta > 0 {
		fmt.Printf("\033[%dD", cursorDelta)
	}
}

type editableInputState struct {
	value  []rune
	cursor int
}

func (s *editableInputState) String() string {
	return string(s.value)
}

func (s *editableInputState) setValue(value string) {
	s.value = []rune(value)
	s.cursor = len(s.value)
}

func (s *editableInputState) moveLeft() {
	if s.cursor > 0 {
		s.cursor--
	}
}

func (s *editableInputState) moveRight() {
	if s.cursor < len(s.value) {
		s.cursor++
	}
}

func (s *editableInputState) moveHome() {
	s.cursor = 0
}

func (s *editableInputState) moveEnd() {
	s.cursor = len(s.value)
}

func (s *editableInputState) insertRunes(runes []rune) {
	if len(runes) == 0 {
		return
	}
	head := append([]rune{}, s.value[:s.cursor]...)
	tail := append([]rune{}, s.value[s.cursor:]...)
	s.value = append(head, runes...)
	s.value = append(s.value, tail...)
	s.cursor += len(runes)
}

func (s *editableInputState) deleteBeforeCursor() {
	if s.cursor == 0 {
		return
	}
	s.value = append(append([]rune{}, s.value[:s.cursor-1]...), s.value[s.cursor:]...)
	s.cursor--
}

func (s *editableInputState) deleteAtCursor() {
	if s.cursor >= len(s.value) {
		return
	}
	s.value = append(append([]rune{}, s.value[:s.cursor]...), s.value[s.cursor+1:]...)
}

func (s *editableInputState) applyAutocomplete(suggestions []string) {
	s.setValue(autocompleteSuggestion(s.String(), suggestions))
}

func visibleRuneWidth(value string) int {
	return utf8.RuneCountInString(value)
}
