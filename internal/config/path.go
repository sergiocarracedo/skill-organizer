package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolvePath(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("path is required")
	}

	expanded, err := expandAliases(value)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", input, err)
	}

	return filepath.Clean(resolved), nil
}

func expandAliases(input string) (string, error) {
	value := os.Expand(input, func(name string) string {
		return os.Getenv(name)
	})

	if value == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return home, nil
	}

	if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, value[2:]), nil
	}

	return value, nil
}
