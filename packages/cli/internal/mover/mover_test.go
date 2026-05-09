package mover

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func TestPlanAndApplyMovesUnmanagedEntries(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".agents", "skills-organized")
	target := filepath.Join(root, ".agents", "skills")
	createSkill(t, filepath.Join(source, "personal", "example"), "example")

	if _, err := syncpkg.Run(configpkg.Location{Source: source, Target: target}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	manual := filepath.Join(target, "manual-skill")
	if err := os.MkdirAll(manual, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(manual, "SKILL.md"), []byte("---\nname: manual-skill\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	moves, err := Plan(configpkg.Location{Source: source, Target: target})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(moves) != 1 {
		t.Fatalf("Plan() len = %d, want 1", len(moves))
	}

	if err := Apply(moves); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(source, "manual-skill")); err != nil {
		t.Fatalf("moved skill missing from source: %v", err)
	}
	if _, err := os.Stat(manual); !os.IsNotExist(err) {
		t.Fatalf("manual skill still exists in target")
	}
}

func TestSetRelativeTargetOverridesDefaultDestination(t *testing.T) {
	location := configpkg.Location{
		Source: filepath.Join(t.TempDir(), ".agents", "skills-organized"),
		Target: filepath.Join(t.TempDir(), ".agents", "skills"),
	}
	move := Move{
		Name:   "manual-skill",
		Source: filepath.Join(location.Target, "manual-skill"),
		Target: filepath.Join(location.Source, "manual-skill"),
	}

	updated, err := SetRelativeTarget(location, move, "3rdparty/asciinema/asciinema-recorder")
	if err != nil {
		t.Fatalf("SetRelativeTarget() error = %v", err)
	}

	want := filepath.Join(location.Source, "3rdparty", "asciinema", "asciinema-recorder")
	if updated.Target != want {
		t.Fatalf("SetRelativeTarget() target = %q, want %q", updated.Target, want)
	}
}

func TestSetRelativeTargetRejectsEscapingSourceRoot(t *testing.T) {
	location := configpkg.Location{Source: filepath.Join(t.TempDir(), ".agents", "skills-organized")}
	move := Move{Name: "manual-skill"}

	if _, err := SetRelativeTarget(location, move, "../outside"); err == nil {
		t.Fatalf("SetRelativeTarget() error = nil, want error")
	}
}

func createSkill(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: " + name + "\ndescription: test\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
