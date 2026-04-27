package watch

import (
	"path/filepath"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestAffectedProjectsMatchesOnlyTouchedProject(t *testing.T) {
	runner := &Runner{
		locations: map[string]configpkg.Location{
			"/tmp/a/.skill-organizer.yml": {Source: "/tmp/a/skills-organized", Target: "/tmp/a/skills"},
			"/tmp/b/.skill-organizer.yml": {Source: "/tmp/b/skills-organized", Target: "/tmp/b/skills"},
		},
	}

	affected := runner.affectedProjects([]string{filepath.Clean("/tmp/a/skills-organized/personal/example/SKILL.md")})
	if len(affected) != 1 {
		t.Fatalf("affectedProjects() len = %d, want 1", len(affected))
	}
	if _, ok := affected[filepath.Clean("/tmp/a/.skill-organizer.yml")]; !ok {
		t.Fatalf("affectedProjects() missing config for /tmp/a")
	}
}

func TestAffectedProjectsMatchesTargetChange(t *testing.T) {
	runner := &Runner{
		locations: map[string]configpkg.Location{
			"/tmp/a/.skill-organizer.yml": {Source: "/tmp/a/skills-organized", Target: "/tmp/a/skills"},
		},
	}

	affected := runner.affectedProjects([]string{filepath.Clean("/tmp/a/skills/manual-skill")})
	if len(affected) != 1 {
		t.Fatalf("affectedProjects() len = %d, want 1", len(affected))
	}
}
