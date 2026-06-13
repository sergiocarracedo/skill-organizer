package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadAgentSelectionConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	as := AgentSelectionConfig{
		DefaultAgentTool:              "claude",
		AcknowledgedExternalToolCosts: true,
	}

	if err := SaveAgentSelectionConfig(path, as); err != nil {
		t.Fatalf("SaveAgentSelectionConfig() error = %v", err)
	}

	loaded, err := LoadAgentSelectionConfig(path)
	if err != nil {
		t.Fatalf("LoadAgentSelectionConfig() error = %v", err)
	}

	if loaded.DefaultAgentTool != as.DefaultAgentTool {
		t.Fatalf("LoadAgentSelectionConfig().DefaultAgentTool = %q, want %q", loaded.DefaultAgentTool, as.DefaultAgentTool)
	}
	if loaded.AcknowledgedExternalToolCosts != as.AcknowledgedExternalToolCosts {
		t.Fatalf("LoadAgentSelectionConfig().AcknowledgedExternalToolCosts = %v, want %v", loaded.AcknowledgedExternalToolCosts, as.AcknowledgedExternalToolCosts)
	}
}

func TestLoadAppConfigWithAgentSelectionMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	content := []byte("watched:\n  - /tmp/a/.skill-organizer.yml\noverlap:\n  default-agent-tool: codex\n  acknowledged-external-tool-costs: true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if cfg.AgentSelection.DefaultAgentTool != "codex" {
		t.Fatalf("LoadAppConfig().AgentSelection.DefaultAgentTool = %q, want %q", cfg.AgentSelection.DefaultAgentTool, "codex")
	}
	if !cfg.AgentSelection.AcknowledgedExternalToolCosts {
		t.Fatalf("LoadAppConfig().AgentSelection.AcknowledgedExternalToolCosts = false, want true")
	}
}

func TestAgentSelectionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	as := AgentSelectionConfig{
		DefaultAgentTool:              "opencode",
		AcknowledgedExternalToolCosts: true,
	}

	if err := SaveAgentSelectionConfig(path, as); err != nil {
		t.Fatalf("SaveAgentSelectionConfig() error = %v", err)
	}

	loaded, err := LoadAgentSelectionConfig(path)
	if err != nil {
		t.Fatalf("LoadAgentSelectionConfig() error = %v", err)
	}

	if loaded.DefaultAgentTool != "opencode" {
		t.Fatalf("LoadAgentSelectionConfig().DefaultAgentTool = %q, want %q", loaded.DefaultAgentTool, "opencode")
	}
	if loaded.AcknowledgedExternalToolCosts != true {
		t.Fatalf("LoadAgentSelectionConfig().AcknowledgedExternalToolCosts = %v, want true", loaded.AcknowledgedExternalToolCosts)
	}
}

func TestSaveWritesAgentSelectionKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	as := AgentSelectionConfig{
		DefaultAgentTool:              "claude",
		AcknowledgedExternalToolCosts: false,
	}

	if err := SaveAgentSelectionConfig(path, as); err != nil {
		t.Fatalf("SaveAgentSelectionConfig() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !containsLine(string(content), "agent-selection:") {
		t.Fatalf("YAML content missing agent-selection key: %q", string(content))
	}
}

func TestAgentSelectionConfigDefaultModelRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	as := AgentSelectionConfig{
		DefaultAgentTool: "opencode",
		DefaultModel:     "anthropic/claude-sonnet-4",
	}

	if err := SaveAgentSelectionConfig(path, as); err != nil {
		t.Fatalf("SaveAgentSelectionConfig() error = %v", err)
	}

	loaded, err := LoadAgentSelectionConfig(path)
	if err != nil {
		t.Fatalf("LoadAgentSelectionConfig() error = %v", err)
	}

	if loaded.DefaultModel != "anthropic/claude-sonnet-4" {
		t.Fatalf("LoadAgentSelectionConfig().DefaultModel = %q, want %q", loaded.DefaultModel, "anthropic/claude-sonnet-4")
	}
}

func TestAgentSelectionConfigKnownModelsNotPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	as := AgentSelectionConfig{
		DefaultAgentTool: "opencode",
		DefaultModel:     "anthropic/claude-sonnet-4",
		KnownModels:      []string{"model1", "model2"},
	}

	if err := SaveAgentSelectionConfig(path, as); err != nil {
		t.Fatalf("SaveAgentSelectionConfig() error = %v", err)
	}

	loaded, err := LoadAgentSelectionConfig(path)
	if err != nil {
		t.Fatalf("LoadAgentSelectionConfig() error = %v", err)
	}

	if len(loaded.KnownModels) != 0 {
		t.Fatalf("LoadAgentSelectionConfig().KnownModels = %v, want empty (not persisted)", loaded.KnownModels)
	}
}

func containsLine(s, substr string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == substr {
			return true
		}
	}
	return false
}
