package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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

// TelemetryConfig holds the opt-in telemetry settings. The Enabled flag
// controls whether the first-run prompt is sticky-yes; the Endpoint
// field is the URL the HTTPRecorder POSTs events to. An empty Endpoint
// forces the recorder to NoopRecorder regardless of Enabled (per CONTEXT).
type TelemetryConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint,omitempty"`
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
	c.Endpoint = strings.TrimSpace(c.Endpoint)
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
