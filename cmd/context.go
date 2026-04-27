package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

type resolvedProject struct {
	ConfigPath string
	Location   configpkg.Location
	Fallback   bool
}

func resolveConfigPath() (string, bool, error) {
	if configPath != "" {
		return configPath, false, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("resolve working directory: %w", err)
	}

	path, err := configpkg.DiscoverFrom(wd)
	if err == nil {
		return path, false, nil
	}
	if !errors.Is(err, configpkg.ErrConfigNotFound) {
		return "", false, fmt.Errorf("resolve project config: %w", err)
	}

	target, fallbackErr := configpkg.HomeFallbackTarget()
	if fallbackErr == nil {
		return configpkg.ConfigPathForTarget(target), true, nil
	}
	if !errors.Is(fallbackErr, configpkg.ErrConfigNotFound) {
		return "", false, fmt.Errorf("resolve fallback project config: %w", fallbackErr)
	}

	return "", false, fmt.Errorf("resolve project config: %w", err)
}
func loadResolvedProject() (resolvedProject, error) {
	path, fallback, err := resolveConfigPath()
	if err != nil {
		return resolvedProject{}, err
	}

	if fallback {
		target := filepath.Join(filepath.Dir(path), "skills")
		return resolvedProject{
			ConfigPath: path,
			Location: configpkg.Location{
				Source: configpkg.DefaultSourceForTarget(target),
				Target: target,
			},
			Fallback: true,
		}, nil
	}

	location, err := configpkg.LoadLocation(path)
	if err != nil {
		return resolvedProject{}, err
	}

	return resolvedProject{ConfigPath: path, Location: location}, nil
}

func loadResolvedLocation() (string, configpkg.Location, error) {
	project, err := loadResolvedProject()
	if err != nil {
		return "", configpkg.Location{}, err
	}

	return project.ConfigPath, project.Location, nil
}
