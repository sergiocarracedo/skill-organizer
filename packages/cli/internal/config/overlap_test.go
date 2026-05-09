package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadOverlapConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	overlap := OverlapConfig{
		DefaultAgentTool:              "claude",
		AcknowledgedExternalToolCosts: true,
	}

	if err := SaveOverlapConfig(path, overlap); err != nil {
		t.Fatalf("SaveOverlapConfig() error = %v", err)
	}

	loaded, err := LoadOverlapConfig(path)
	if err != nil {
		t.Fatalf("LoadOverlapConfig() error = %v", err)
	}

	if loaded.DefaultAgentTool != overlap.DefaultAgentTool {
		t.Fatalf("LoadOverlapConfig().DefaultAgentTool = %q, want %q", loaded.DefaultAgentTool, overlap.DefaultAgentTool)
	}
	if loaded.AcknowledgedExternalToolCosts != overlap.AcknowledgedExternalToolCosts {
		t.Fatalf("LoadOverlapConfig().AcknowledgedExternalToolCosts = %v, want %v", loaded.AcknowledgedExternalToolCosts, overlap.AcknowledgedExternalToolCosts)
	}
}

func TestLoadAppConfigSupportsOverlapSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	content := []byte("watched:\n  - /tmp/a/.skill-organizer.yml\noverlap:\n  default-agent-tool: codex\n  acknowledged-external-tool-costs: true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if cfg.Overlap.DefaultAgentTool != "codex" {
		t.Fatalf("LoadAppConfig().Overlap.DefaultAgentTool = %q, want %q", cfg.Overlap.DefaultAgentTool, "codex")
	}
	if !cfg.Overlap.AcknowledgedExternalToolCosts {
		t.Fatalf("LoadAppConfig().Overlap.AcknowledgedExternalToolCosts = false, want true")
	}
}
