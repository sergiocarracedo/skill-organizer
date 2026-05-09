package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryAddAndRemove(t *testing.T) {
	registry := WatchRegistry{}
	first := filepath.Join(t.TempDir(), "a.yml")
	second := filepath.Join(t.TempDir(), "b.yml")

	registry.Add(first)
	registry.Add(second)
	registry.Add(first)

	if len(registry.Watched) != 2 {
		t.Fatalf("Add() len = %d, want 2", len(registry.Watched))
	}

	if !registry.Remove(first) {
		t.Fatalf("Remove() = false, want true")
	}

	if registry.Remove(first) {
		t.Fatalf("Remove() second call = true, want false")
	}
}

func TestSaveRegistryPreservesServiceConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	if err := SaveServiceConfig(path, ServiceConfig{LogLevel: "debug"}); err != nil {
		t.Fatalf("SaveServiceConfig() error = %v", err)
	}

	registry := WatchRegistry{}
	registry.Add(filepath.Join(t.TempDir(), "a.yml"))
	if err := SaveRegistry(path, registry); err != nil {
		t.Fatalf("SaveRegistry() error = %v", err)
	}

	serviceCfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatalf("LoadServiceConfig() error = %v", err)
	}
	if serviceCfg.LogLevel != "debug" {
		t.Fatalf("LoadServiceConfig().LogLevel = %q, want %q", serviceCfg.LogLevel, "debug")
	}
}

func TestSaveServiceConfigPreservesWatchedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	registry := WatchRegistry{}
	registry.Add(filepath.Join(t.TempDir(), "a.yml"))
	if err := SaveRegistry(path, registry); err != nil {
		t.Fatalf("SaveRegistry() error = %v", err)
	}

	if err := SaveServiceConfig(path, ServiceConfig{LogLevel: "warn"}); err != nil {
		t.Fatalf("SaveServiceConfig() error = %v", err)
	}

	loaded, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	if len(loaded.Watched) != 1 {
		t.Fatalf("LoadRegistry().Watched len = %d, want 1", len(loaded.Watched))
	}
}

func TestSaveAppConfigWritesMergedShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	cfg := AppConfig{
		Watched: []string{"/tmp/a/.skill-organizer.yml"},
		Service: ServiceConfig{LogLevel: "error"},
	}
	if err := SaveAppConfig(path, cfg); err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if text == "" {
		t.Fatalf("SaveAppConfig() wrote empty file")
	}
	if !strings.Contains(text, "watched:") || !strings.Contains(text, "service:") || !strings.Contains(text, "log-level: error") {
		t.Fatalf("SaveAppConfig() content = %q", text)
	}
}
