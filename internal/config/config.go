package config

import (
	"fmt"
	"path/filepath"
	"sort"
)

type Location struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

func (l Location) Validate() error {
	if l.Source == "" {
		return fmt.Errorf("project config source is required")
	}
	if l.Target == "" {
		return fmt.Errorf("project config target is required")
	}
	if filepath.Clean(l.Source) == filepath.Clean(l.Target) {
		return fmt.Errorf("project config source and target must be different")
	}

	return nil
}

type WatchRegistry struct {
	Watched []string `yaml:"watched"`
}

type ServiceConfig struct {
	LogLevel string `yaml:"log-level"`
}

type AppConfig struct {
	Watched []string      `yaml:"watched"`
	Service ServiceConfig `yaml:"service"`
}

func (r *WatchRegistry) Normalize() {
	seen := make(map[string]struct{}, len(r.Watched))
	unique := make([]string, 0, len(r.Watched))

	for _, watched := range r.Watched {
		if watched == "" {
			continue
		}

		normalized := filepath.Clean(watched)
		if _, ok := seen[normalized]; ok {
			continue
		}

		seen[normalized] = struct{}{}
		unique = append(unique, normalized)
	}

	sort.Strings(unique)
	r.Watched = unique
}

func (c *AppConfig) Normalize() {
	registry := WatchRegistry{Watched: c.Watched}
	registry.Normalize()
	c.Watched = registry.Watched
	c.Service.Normalize()
}

func (c AppConfig) WatchRegistry() WatchRegistry {
	registry := WatchRegistry{Watched: append([]string{}, c.Watched...)}
	registry.Normalize()
	return registry
}

func (c *AppConfig) SetWatchRegistry(registry WatchRegistry) {
	registry.Normalize()
	c.Watched = append([]string{}, registry.Watched...)
	c.Service.Normalize()
}

func (c *ServiceConfig) Normalize() {
	if !IsValidLogLevel(c.LogLevel) {
		c.LogLevel = DefaultLogLevel
	}
}

const DefaultLogLevel = "info"

func IsValidLogLevel(level string) bool {
	switch level {
	case "error", "warn", "info", "debug":
		return true
	default:
		return false
	}
}
