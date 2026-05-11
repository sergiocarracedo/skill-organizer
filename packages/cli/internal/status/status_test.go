package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func TestBuildReportsManagedAndUnmanagedEntries(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".agents", "skills-organized")
	target := filepath.Join(root, ".agents", "skills")
	createSkill(t, filepath.Join(source, "personal", "example"), "example", false)
	createSkill(t, filepath.Join(source, "personal", "disabled"), "disabled", true)

	if _, err := syncpkg.Run(configpkg.Location{Source: source, Target: target}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(target, "manual-skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "manual-skill", "SKILL.md"), []byte("---\nname: manual-skill\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "IMPORTANT.md"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(target, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", root); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	t.Cleanup(func() {
		if originalConfigHome == "" {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
			return
		}
		_ = os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
	})
	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		t.Fatalf("UpdatesPath() error = %v", err)
	}
	if err := configpkg.SaveUpdatesState(updatesPath, configpkg.UpdatesState{Pending: []configpkg.SkillUpdateRecord{{
		RelativePath:  "personal/example",
		FlattenedName: "example",
		LatestVersion: "9a8b7c6d5e4f3a2b1c0d",
		CheckedAt:     "2026-05-10T13:49:48Z",
	}}}); err != nil {
		t.Fatalf("SaveUpdatesState() error = %v", err)
	}

	report, err := Build(configpkg.Location{Source: source, Target: target})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(report.Unmanaged) != 1 || report.Unmanaged[0] != "manual-skill" {
		t.Fatalf("Build() unmanaged = %#v, want manual-skill", report.Unmanaged)
	}

	states := map[string]SkillState{}
	for _, entry := range report.Skills {
		states[entry.Skill.RelativePath] = entry.State
	}

	if states["personal/example"] != StateSynced {
		t.Fatalf("state for personal/example = %q, want %q", states["personal/example"], StateSynced)
	}
	if states["personal/disabled"] != StateDisabled {
		t.Fatalf("state for personal/disabled = %q, want %q", states["personal/disabled"], StateDisabled)
	}

	entries := map[string]SkillStatus{}
	for _, entry := range report.Skills {
		entries[entry.Skill.RelativePath] = entry
	}
	if entries["personal/example"].InstalledVersion != "1.0.0" {
		t.Fatalf("installed version = %q, want %q", entries["personal/example"].InstalledVersion, "1.0.0")
	}
	if entries["personal/example"].InstalledDate != "2026-05-01T12:00:00Z" {
		t.Fatalf("installed date = %q, want %q", entries["personal/example"].InstalledDate, "2026-05-01T12:00:00Z")
	}
	if entries["personal/example"].AvailableVersion != "9a8b7c6d5e4f3a2b1c0d" {
		t.Fatalf("available version = %q, want %q", entries["personal/example"].AvailableVersion, "9a8b7c6d5e4f3a2b1c0d")
	}
	if entries["personal/example"].AvailableCheckedDate != "2026-05-10T13:49:48Z" {
		t.Fatalf("available checked date = %q, want %q", entries["personal/example"].AvailableCheckedDate, "2026-05-10T13:49:48Z")
	}

	summary := report.Summary()
	if summary.TotalSkills != 2 {
		t.Fatalf("summary total skills = %d, want 2", summary.TotalSkills)
	}
	if summary.ManagedSkills != 1 {
		t.Fatalf("summary managed skills = %d, want 1", summary.ManagedSkills)
	}
	if summary.UnmanagedSkills != 1 {
		t.Fatalf("summary unmanaged skills = %d, want 1", summary.UnmanagedSkills)
	}
	if summary.Synced != 1 {
		t.Fatalf("summary synced = %d, want 1", summary.Synced)
	}
	if summary.Disabled != 1 {
		t.Fatalf("summary disabled = %d, want 1", summary.Disabled)
	}
}

func createSkill(t *testing.T, dir string, name string, disabled bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	metadataLines := []string{
		"metadata:",
		"  skill-organizer:",
		"    installed-version: 1.0.0",
		"    last-updated-at: \"2026-05-01T12:00:00Z\"",
	}
	if disabled {
		metadataLines = append(metadataLines, "    disabled: true")
	}
	content := "---\nname: " + name + "\ndescription: test\n" + strings.Join(metadataLines, "\n") + "\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
