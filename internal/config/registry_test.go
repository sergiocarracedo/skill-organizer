package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatchRegistryNormalize(t *testing.T) {
	registry := WatchRegistry{
		Watched: []string{"/tmp/b", "/tmp/a", "/tmp/a", ""},
	}

	registry.Normalize()

	got := registry.Watched
	want := []string{"/tmp/a", "/tmp/b"}
	if len(got) != len(want) {
		t.Fatalf("Normalize() len = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Normalize()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadAppConfigOrDefaultUsesInfoLogLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")

	cfg, err := LoadAppConfigOrDefault(path)
	if err != nil {
		t.Fatalf("LoadAppConfigOrDefault() error = %v", err)
	}

	if cfg.Service.LogLevel != DefaultLogLevel {
		t.Fatalf("LoadAppConfigOrDefault().Service.LogLevel = %q, want %q", cfg.Service.LogLevel, DefaultLogLevel)
	}
}

func TestLoadAppConfigSupportsLegacyWatchedOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-organizer.yml")
	content := []byte("watched:\n  - /tmp/a/.skill-organizer.yml\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if len(cfg.Watched) != 1 || cfg.Watched[0] != "/tmp/a/.skill-organizer.yml" {
		t.Fatalf("LoadAppConfig().Watched = %#v", cfg.Watched)
	}
	if cfg.Service.LogLevel != DefaultLogLevel {
		t.Fatalf("LoadAppConfig().Service.LogLevel = %q, want %q", cfg.Service.LogLevel, DefaultLogLevel)
	}
}
