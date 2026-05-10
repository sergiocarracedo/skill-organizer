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
		Backup:  BackupConfig{RetentionDays: 14},
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
	if !strings.Contains(text, "watched:") || !strings.Contains(text, "service:") || !strings.Contains(text, "log-level: error") || !strings.Contains(text, "backup:") || !strings.Contains(text, "retention-days: 14") {
		t.Fatalf("SaveAppConfig() content = %q", text)
	}
}

func TestSaveBackupConfigPreservesWatchedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	registry := WatchRegistry{}
	registry.Add(filepath.Join(t.TempDir(), "a.yml"))
	if err := SaveRegistry(path, registry); err != nil {
		t.Fatalf("SaveRegistry() error = %v", err)
	}

	if err := SaveBackupConfig(path, BackupConfig{RetentionDays: 21}); err != nil {
		t.Fatalf("SaveBackupConfig() error = %v", err)
	}

	loaded, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	if len(loaded.Watched) != 1 {
		t.Fatalf("LoadRegistry().Watched len = %d, want 1", len(loaded.Watched))
	}

	backup, err := LoadBackupConfig(path)
	if err != nil {
		t.Fatalf("LoadBackupConfig() error = %v", err)
	}
	if backup.RetentionDays != 21 {
		t.Fatalf("LoadBackupConfig().RetentionDays = %d, want 21", backup.RetentionDays)
	}
}

func TestSaveAndLoadUpdatesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".updates")
	state := UpdatesState{
		LastCheckedAt:  "2026-05-10T13:49:48Z",
		UpdateCount:    2,
		LastRemindedAt: "2026-05-11T13:49:48Z",
		Pending: []SkillUpdateRecord{{
			RelativePath:     "google/gws-admin-reports",
			InstalledVersion: "0.22.3",
			LatestVersion:    "0.22.5",
		}},
	}
	if err := SaveUpdatesState(path, state); err != nil {
		t.Fatalf("SaveUpdatesState() error = %v", err)
	}
	loaded, err := LoadUpdatesState(path)
	if err != nil {
		t.Fatalf("LoadUpdatesState() error = %v", err)
	}
	if loaded.UpdateCount != 2 {
		t.Fatalf("LoadUpdatesState().UpdateCount = %d, want 2", loaded.UpdateCount)
	}
	if len(loaded.Pending) != 1 || loaded.Pending[0].RelativePath != "google/gws-admin-reports" {
		t.Fatalf("LoadUpdatesState().Pending = %#v", loaded.Pending)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	for _, want := range []string{"update-count: 2", "last-reminded-at: \"2026-05-11T13:49:48Z\"", "relative-path: google/gws-admin-reports"} {
		if !strings.Contains(text, want) {
			t.Fatalf("updates file missing %q\n%s", want, text)
		}
	}
}
