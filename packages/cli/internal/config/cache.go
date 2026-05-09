package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type UpdateCache struct {
	LastCheckedAt string `yaml:"last-checked-at,omitempty"`
	LatestVersion string `yaml:"latest-version,omitempty"`
	LatestPageURL string `yaml:"latest-page-url,omitempty"`
}

func CachePath() (string, error) {
	registryPath, err := RegistryPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(registryPath), ".cache.yml"), nil
}

func (c *UpdateCache) Normalize() {
	c.LastCheckedAt = strings.TrimSpace(c.LastCheckedAt)
	c.LatestVersion = strings.TrimSpace(c.LatestVersion)
	c.LatestPageURL = strings.TrimSpace(c.LatestPageURL)
}

func LoadUpdateCache(path string) (UpdateCache, error) {
	var cache UpdateCache

	content, err := os.ReadFile(path)
	if err != nil {
		return cache, fmt.Errorf("read update cache: %w", err)
	}

	if err := yaml.Unmarshal(content, &cache); err != nil {
		return cache, fmt.Errorf("parse update cache: %w", err)
	}
	cache.Normalize()
	return cache, nil
}

func LoadUpdateCacheOrDefault(path string) (UpdateCache, error) {
	cache, err := LoadUpdateCache(path)
	if errors.Is(err, os.ErrNotExist) {
		cache.Normalize()
		return cache, nil
	}
	return cache, err
}

func SaveUpdateCache(path string, cache UpdateCache) error {
	cache.Normalize()

	content, err := yaml.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal update cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}

	return nil
}

func ClearUpdateCache(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove update cache: %w", err)
	}
	return nil
}
