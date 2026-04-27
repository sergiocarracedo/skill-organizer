package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

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
