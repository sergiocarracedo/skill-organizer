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

type AgentSelectionConfig struct {
	DefaultAgentTool              string `yaml:"default-agent-tool,omitempty"`
	AcknowledgedExternalToolCosts bool   `yaml:"acknowledged-external-tool-costs,omitempty"`
}

type OverlapConfig = AgentSelectionConfig

type BackupConfig struct {
	RetentionDays int `yaml:"retention-days,omitempty"`
}

// TelemetryConfig holds the opt-in telemetry settings. The Enabled
// flag is the only user-facing key (Phase 5 REQ-10). The endpoint
// and API key are build-time vars set via -ldflags on the
// `telemetry.NewRelicEndpoint` and `telemetry.NewRelicAPIKey` vars
// in the internal/telemetry package — they never appear in YAML
// (the user never configures them).
type TelemetryConfig struct {
	Enabled bool `yaml:"enabled"`
}

type AppConfig struct {
	Watched        []string             `yaml:"watched"`
	Service        ServiceConfig        `yaml:"service"`
	AgentSelection AgentSelectionConfig `yaml:"agent-selection,omitempty"`
	Backup         BackupConfig         `yaml:"backup,omitempty"`
	Telemetry      TelemetryConfig      `yaml:"telemetry,omitempty"`
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
	c.AgentSelection.Normalize()
	c.Backup.Normalize()
	c.Telemetry.Normalize()
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
	c.AgentSelection.Normalize()
	c.Backup.Normalize()
	c.Telemetry.Normalize()
}

func (c *ServiceConfig) Normalize() {
	if !IsValidLogLevel(c.LogLevel) {
		c.LogLevel = DefaultLogLevel
	}
}

func (c *AgentSelectionConfig) Normalize() {
	c.DefaultAgentTool = filepath.Clean(c.DefaultAgentTool)
	if c.DefaultAgentTool == "." {
		c.DefaultAgentTool = ""
	}
}

func (c *BackupConfig) Normalize() {
	if c.RetentionDays <= 0 {
		c.RetentionDays = DefaultBackupRetentionDays
	}
}

func (c *TelemetryConfig) Normalize() {
	// Phase 5 REQ-10: TelemetryConfig is a single bool. No
	// normalization needed; method kept for symmetry with the
	// other config types (Normalize is called by AppConfig.Normalize).
	_ = c
}

const DefaultLogLevel = "info"
const DefaultBackupRetentionDays = 10

func IsValidLogLevel(level string) bool {
	switch level {
	case "error", "warn", "info", "debug":
		return true
	default:
		return false
	}
}
