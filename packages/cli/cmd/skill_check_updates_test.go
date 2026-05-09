package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestDiffLinesShowsAddedRemovedAndChangedFiles(t *testing.T) {
	oldFiles := []remotepkg.File{{Path: "SKILL.md", Contents: "old"}, {Path: "notes.txt", Contents: "gone"}}
	newFiles := []remotepkg.File{{Path: "SKILL.md", Contents: "new"}, {Path: "extra.txt", Contents: "added"}}

	lines := diffLines(oldFiles, newFiles)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"--- SKILL.md", "+ new", "- old", "--- notes.txt", "+++ extra.txt"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("diff output missing %q\n%s", want, joined)
		}
	}
}

func TestCollectUpdateCandidatesUsesOriginalNameWhenPresent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "thirdparty", "asciinema")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: thirdparty--asciinema\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: asciinema-recorder\n    source: terrylica/cc-skills\n    repo-skill-path: skills/asciinema-recorder\n    installed-version: old-hash\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(source string, name string) (remotepkg.SkillBundle, error) {
		if source != "terrylica/cc-skills" {
			t.Fatalf("source = %q, want %q", source, "terrylica/cc-skills")
		}
		if name != "asciinema-recorder" {
			t.Fatalf("name = %q, want %q", name, "asciinema-recorder")
		}
		return remotepkg.SkillBundle{
			Root: dir,
			Skill: remotepkg.SkillSummary{
				Source:       source,
				Hash:         "new-hash",
				RepoSkillPath: "skills/asciinema-recorder",
			},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: asciinema-recorder\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	candidates, err := collectUpdateCandidates(configpkg.Location{Source: source, Target: target})
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(candidates))
	}
	if candidates[0].Latest != "new-hash" {
		t.Fatalf("Latest = %q, want %q", candidates[0].Latest, "new-hash")
	}
}

func TestRefreshSkillUpdateCacheWritesPendingEntries(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), ".cache.yml")
	original := cachePathFunc
	cachePathFunc = func() (string, error) { return cachePath, nil }
	t.Cleanup(func() { cachePathFunc = original })

	err := refreshSkillUpdateCache([]skillUpdateCandidate{{
		Skill:     skills.Skill{RelativePath: "thirdparty/example", FlattenedName: "thirdparty--example"},
		Installed: "old",
		Latest:    "new",
		Source:    "owner/repo",
		Metadata:  skills.ManagedMetadata{RepoSkillPath: "skills/example"},
	}})
	if err != nil {
		t.Fatalf("refreshSkillUpdateCache() error = %v", err)
	}
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	for _, want := range []string{"relative-path: thirdparty/example", "installed-version: old", "latest-version: new", "source: owner/repo"} {
		if !strings.Contains(text, want) {
			t.Fatalf("cache content missing %q\n%s", want, text)
		}
	}
}
