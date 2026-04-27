package cmd

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestLoadResolvedProjectFallsBackToAgentsHome(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	agentsTarget := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(agentsTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	previousConfigPath := configPath
	configPath = ""
	t.Cleanup(func() { configPath = previousConfigPath })

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	project, err := loadResolvedProject()
	if err != nil {
		t.Fatalf("loadResolvedProject() error = %v", err)
	}

	if !project.Fallback {
		t.Fatalf("loadResolvedProject() fallback = false, want true")
	}
	if project.ConfigPath != filepath.Join(home, ".agents", configpkg.FileName) {
		t.Fatalf("loadResolvedProject() configPath = %q", project.ConfigPath)
	}
	if project.Location.Target != agentsTarget {
		t.Fatalf("loadResolvedProject() target = %q, want %q", project.Location.Target, agentsTarget)
	}
	if project.Location.Source != filepath.Join(home, ".agents", "skills-organized") {
		t.Fatalf("loadResolvedProject() source = %q", project.Location.Source)
	}
}

func TestLoadResolvedProjectPrefersNearestConfigOverHomeFallback(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	agentsTarget := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(agentsTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	projectConfig := filepath.Join(root, ".agents", configpkg.FileName)
	deepDir := filepath.Join(root, ".agents", "skills-organized", "nested", "example")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	location := configpkg.Location{
		Source: filepath.Join(root, ".agents", "skills-organized"),
		Target: filepath.Join(root, ".agents", "skills"),
	}
	if err := configpkg.SaveLocation(projectConfig, location); err != nil {
		t.Fatalf("SaveLocation() error = %v", err)
	}

	previousConfigPath := configPath
	configPath = ""
	t.Cleanup(func() { configPath = previousConfigPath })

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(deepDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	project, err := loadResolvedProject()
	if err != nil {
		t.Fatalf("loadResolvedProject() error = %v", err)
	}

	if project.Fallback {
		t.Fatalf("loadResolvedProject() fallback = true, want false")
	}
	if project.ConfigPath != projectConfig {
		t.Fatalf("loadResolvedProject() configPath = %q, want %q", project.ConfigPath, projectConfig)
	}
	if project.Location != location {
		t.Fatalf("loadResolvedProject() location = %#v, want %#v", project.Location, location)
	}
}
