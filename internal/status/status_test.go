package status

import (
	"os"
	"path/filepath"
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
	disabledLine := ""
	if disabled {
		disabledLine = "metadata:\n  skill-organizer:\n    disabled: true\n"
	}
	content := "---\nname: " + name + "\ndescription: test\n" + disabledLine + "---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
