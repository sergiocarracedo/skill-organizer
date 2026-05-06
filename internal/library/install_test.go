package library

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
)

func TestInstallFailsWhenDestinationAlreadyExists(t *testing.T) {
	root := t.TempDir()
	location := configpkg.Location{Source: root, Target: filepath.Join(root, "target")}
	existing := filepath.Join(root, "thirdparty", "demo")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	_, err := Install(InstallRequest{
		Location:         location,
		DestinationPaths: map[string]string{"demo": "thirdparty"},
		Bundles: []remote.SkillBundle{{
			Skill: remote.SkillSummary{ID: "demo", Name: "demo"},
			Files: []remote.File{{Path: "SKILL.md", Contents: "---\nname: demo\ndescription: test\n---\n"}},
		}},
	})
	if err == nil {
		t.Fatalf("Install() error = nil, want destination exists error")
	}
}
