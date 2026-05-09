package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const FileName = ".skill-organizer.yml"

var ErrConfigNotFound = errors.New("project config not found")

func LoadLocation(path string) (Location, error) {
	var cfg Location

	content, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read project config: %w", err)
	}

	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return cfg, fmt.Errorf("parse project config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func SaveLocation(path string, cfg Location) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}

func DiscoverFrom(start string) (string, error) {
	current, err := ResolvePath(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	info, err := os.Stat(current)
	if err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		candidate := filepath.Join(current, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat project config: %w", err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrConfigNotFound
		}

		current = parent
	}
}

func DefaultSourceForTarget(target string) string {
	return filepath.Join(filepath.Dir(target), "skills-organized")
}

func ConfigPathForTarget(target string) string {
	return filepath.Join(filepath.Dir(target), FileName)
}

func CandidateTargets(root string) ([]string, error) {
	var matches []string
	patterns := candidateTargetPatterns()

	for _, pattern := range patterns {
		candidate := filepath.Join(root, pattern)
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat candidate target: %w", err)
		}
		if info.IsDir() {
			matches = append(matches, candidate)
		}
	}

	sort.Strings(matches)
	return matches, nil
}

func HomeFallbackTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	for _, pattern := range candidateTargetPatterns() {
		candidate := filepath.Join(home, pattern)
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("stat home fallback target: %w", err)
		}
		if info.IsDir() {
			return candidate, nil
		}
	}

	return "", ErrConfigNotFound
}

func candidateTargetPatterns() []string {
	return []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".agent/skills",
	}
}
