package watch

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestCollectDirWatchPathsIncludesNestedDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	paths, err := collectDirWatchPaths(root)
	if err != nil {
		t.Fatalf("collectDirWatchPaths() error = %v", err)
	}

	want := map[string]struct{}{
		filepath.Clean(root):                          {},
		filepath.Clean(filepath.Join(root, "a")):      {},
		filepath.Clean(filepath.Join(root, "a", "b")): {},
		nested: {},
	}

	for _, path := range paths {
		delete(want, path)
	}

	if len(want) != 0 {
		t.Fatalf("collectDirWatchPaths() missing paths: %#v", want)
	}
}

func TestAffectedProjectsUsesReloadedProjectSet(t *testing.T) {
	runner := &Runner{
		locations: map[string]configpkg.Location{
			"/tmp/a/.skill-organizer.yml": {Source: "/tmp/a/skills-organized", Target: "/tmp/a/skills"},
		},
	}

	affected := runner.affectedProjects([]string{filepath.Clean("/tmp/a/skills/manual")})
	if len(affected) != 1 {
		t.Fatalf("affectedProjects() len = %d, want 1", len(affected))
	}
}
