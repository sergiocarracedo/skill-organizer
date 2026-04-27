package cmd

import (
	"fmt"
	"os"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func resolveConfigPath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	path, err := configpkg.DiscoverFrom(wd)
	if err != nil {
		return "", fmt.Errorf("resolve project config: %w", err)
	}

	return path, nil
}

func loadResolvedLocation() (string, configpkg.Location, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return "", configpkg.Location{}, err
	}

	location, err := configpkg.LoadLocation(path)
	if err != nil {
		return "", configpkg.Location{}, err
	}

	return path, location, nil
}
