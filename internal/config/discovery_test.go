package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFromFindsNearestConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".agents", FileName)
	deepDir := filepath.Join(tempDir, ".agents", "skills-organized", "personal", "example")

	if err := SaveLocation(configPath, Location{Source: "/tmp/source", Target: "/tmp/target"}); err != nil {
		t.Fatalf("SaveLocation() error = %v", err)
	}

	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, err := DiscoverFrom(deepDir)
	if err != nil {
		t.Fatalf("DiscoverFrom() error = %v", err)
	}

	if got != configPath {
		t.Fatalf("DiscoverFrom() = %q, want %q", got, configPath)
	}
}

func TestDiscoverFromReturnsNotFound(t *testing.T) {
	_, err := DiscoverFrom(t.TempDir())
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("DiscoverFrom() error = %v, want %v", err, ErrConfigNotFound)
	}
}

func TestDefaultSourceForTarget(t *testing.T) {
	target := filepath.Join("/repo", ".agents", "skills")
	want := filepath.Join("/repo", ".agents", "skills-organized")

	if got := DefaultSourceForTarget(target); got != want {
		t.Fatalf("DefaultSourceForTarget() = %q, want %q", got, want)
	}
}

func TestHomeFallbackTargetPrefersAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeTarget := filepath.Join(home, ".claude", "skills")
	agentsTarget := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(claudeTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(agentsTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, err := HomeFallbackTarget()
	if err != nil {
		t.Fatalf("HomeFallbackTarget() error = %v", err)
	}

	if got != agentsTarget {
		t.Fatalf("HomeFallbackTarget() = %q, want %q", got, agentsTarget)
	}
}

func TestHomeFallbackTargetUsesOtherSupportedHomes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	codexTarget := filepath.Join(home, ".codex", "skills")
	if err := os.MkdirAll(codexTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, err := HomeFallbackTarget()
	if err != nil {
		t.Fatalf("HomeFallbackTarget() error = %v", err)
	}

	if got != codexTarget {
		t.Fatalf("HomeFallbackTarget() = %q, want %q", got, codexTarget)
	}
}

func TestHomeFallbackTargetReturnsNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := HomeFallbackTarget()
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("HomeFallbackTarget() error = %v, want %v", err, ErrConfigNotFound)
	}
}
