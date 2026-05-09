package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestRunCreatesManifestAndSymlinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".agents", "skills-organized")
	target := filepath.Join(root, ".agents", "skills")
	createSkill(t, filepath.Join(source, "personal", "example"), "example", false)
	createSkill(t, filepath.Join(source, "personal", "disabled"), "disabled", true)

	result, err := Run(configpkg.Location{Source: source, Target: target})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(result.Enabled) != 1 {
		t.Fatalf("Run() enabled len = %d, want 1", len(result.Enabled))
	}
	if len(result.Disabled) != 1 {
		t.Fatalf("Run() disabled len = %d, want 1", len(result.Disabled))
	}

	linkPath := filepath.Join(target, "personal--example")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target entry is not a symlink")
	}

	if _, err := os.Lstat(filepath.Join(target, "personal--disabled")); !os.IsNotExist(err) {
		t.Fatalf("disabled skill target should not exist")
	}

	manifest, err := LoadManifest(target)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if manifest.Managed["personal--example"] != "personal/example" {
		t.Fatalf("manifest managed entry = %q, want %q", manifest.Managed["personal--example"], "personal/example")
	}

	updated, err := os.ReadFile(filepath.Join(source, "personal", "example", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(updated), "name: personal--example") {
		t.Fatalf("sync did not rewrite flattened name\n%s", string(updated))
	}
}

func TestRunRemovesStaleManagedEntries(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".agents", "skills-organized")
	target := filepath.Join(root, ".agents", "skills")
	createSkill(t, filepath.Join(source, "personal", "example"), "example", false)

	if _, err := Run(configpkg.Location{Source: source, Target: target}); err != nil {
		t.Fatalf("Run() first error = %v", err)
	}

	if err := os.RemoveAll(filepath.Join(source, "personal", "example")); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	if _, err := Run(configpkg.Location{Source: source, Target: target}); err != nil {
		t.Fatalf("Run() second error = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(target, "personal--example")); !os.IsNotExist(err) {
		t.Fatalf("stale managed symlink was not removed")
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
