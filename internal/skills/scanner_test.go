package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSourceStopsAtTerminalSkills(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "personal", "alpha"), "alpha")
	createSkill(t, filepath.Join(root, "nested", "parent"), "parent")
	createSkill(t, filepath.Join(root, "sibling", "child"), "child")

	if err := os.MkdirAll(filepath.Join(root, "nested", "parent", "ignored", "child"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "parent", "ignored", "child", SkillFileName), []byte("---\nname: ignored\ndescription: ignored\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	scanned, err := ScanSource(root)
	if err != nil {
		t.Fatalf("ScanSource() error = %v", err)
	}

	if len(scanned) != 3 {
		t.Fatalf("ScanSource() len = %d, want 3", len(scanned))
	}

	if scanned[1].RelativePath != "personal/alpha" {
		t.Fatalf("unexpected scanned relative path: %q", scanned[1].RelativePath)
	}

	for _, skill := range scanned {
		if skill.RelativePath == "nested/parent/ignored/child" {
			t.Fatalf("ScanSource() unexpectedly included nested child skill")
		}
	}
}

func TestScanSourceDetectsFlattenCollisions(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "alpha", "beta"), "one")
	createSkill(t, filepath.Join(root, "alpha--beta"), "two")

	if _, err := ScanSource(root); err == nil {
		t.Fatalf("ScanSource() error = nil, want collision error")
	}
}

func TestResolveSourceSkill(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "personal", "example")
	createSkill(t, dir, "example")

	skill, err := ResolveSourceSkill(root, filepath.Join(dir, SkillFileName))
	if err != nil {
		t.Fatalf("ResolveSourceSkill() error = %v", err)
	}

	if skill.FlattenedName != "personal--example" {
		t.Fatalf("ResolveSourceSkill() flattened = %q, want %q", skill.FlattenedName, "personal--example")
	}
}

func createSkill(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: " + name + "\ndescription: test\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, SkillFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
