package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func TestResolveSkillsForDeleteSupportsWildcards(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	for _, rel := range []string{"google/gws-admin-reports", "google/gws-docs", "personal/demo"} {
		dir := filepath.Join(source, filepath.FromSlash(rel))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\n---\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	matched, err := resolveSkillsForDelete(source, "google/*")
	if err != nil {
		t.Fatalf("resolveSkillsForDelete() error = %v", err)
	}
	if len(matched) != 2 {
		t.Fatalf("resolveSkillsForDelete() len = %d, want 2", len(matched))
	}
	got := []string{matched[0].RelativePath, matched[1].RelativePath}
	joined := strings.Join(got, ",")
	for _, want := range []string{"google/gws-admin-reports", "google/gws-docs"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("matched skills missing %q in %q", want, joined)
		}
	}
}

func TestSkillDeleteNoBackupRemovesSkillDirectory(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	skillDir := filepath.Join(source, "google", "gws-admin-reports")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: google--gws-admin-reports\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalLoadResolvedLocation := skillDeleteLoadResolvedLocation
	originalConfirm := skillDeleteConfirm
	originalSync := runSkillDeleteSync
	originalRegistryPath := skillDeleteRegistryPath
	originalLoadBackupConfig := skillDeleteLoadBackupConfig
	originalMoveToBackup := skillDeleteMoveToBackup
	skillDeleteLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return filepath.Join(root, ".skill-organizer.yml"), configpkg.Location{Source: source, Target: target}, nil
	}
	skillDeleteConfirm = func(prompt string, defaultValue bool) (bool, error) {
		return true, nil
	}
	runSkillDeleteSync = func(location configpkg.Location) (syncpkg.Result, error) {
		return syncpkg.Result{}, nil
	}
	skillDeleteRegistryPath = func() (string, error) {
		return filepath.Join(configRoot, "skill-organizer.yml"), nil
	}
	skillDeleteLoadBackupConfig = func(path string) (configpkg.BackupConfig, error) {
		return configpkg.BackupConfig{RetentionDays: 10}, nil
	}
	skillDeleteMoveToBackup = func(sourcePath string, flattenedName string, metadata backuppkg.Metadata, now time.Time) (string, error) {
		return backuppkg.MoveSkill(sourcePath, flattenedName, metadata, now)
	}
	t.Cleanup(func() {
		skillDeleteLoadResolvedLocation = originalLoadResolvedLocation
		skillDeleteConfirm = originalConfirm
		runSkillDeleteSync = originalSync
		skillDeleteRegistryPath = originalRegistryPath
		skillDeleteLoadBackupConfig = originalLoadBackupConfig
		skillDeleteMoveToBackup = originalMoveToBackup
	})

	cmd := newSkillDeleteCommand()
	if err := cmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("Flags().Set(yes) error = %v", err)
	}
	if err := cmd.Flags().Set("no-backup", "true"); err != nil {
		t.Fatalf("Flags().Set(no-backup) error = %v", err)
	}
	if err := cmd.RunE(cmd, []string{"google/gws-admin-reports"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("skill dir still exists, stat err = %v", err)
	}
	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		t.Fatalf("UpdatesPath() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(updatesPath), ".old")); !os.IsNotExist(err) {
		t.Fatalf(".old directory should not exist for --no-backup, stat err = %v", err)
	}
}
