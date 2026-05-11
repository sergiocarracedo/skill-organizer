package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestRenderTryFindMetadataProgressTextIncludesSkippedCount(t *testing.T) {
	got := renderTryFindMetadataProgressText("checking", 2, 5, 1, 3)
	plain := stripANSI(got)
	for _, want := range []string{"checking 2/5", "Found: 1", "Skipped: 3"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderTryFindMetadataProgressText() missing %q in %q", want, plain)
		}
	}
}

func TestFindBestMetadataMatchRejectsAmbiguousCandidates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills-organized", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: demo-skill\ndescription: test\n---\n\n# Demo\n"
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	skill := skills.Skill{RelativePath: "demo", SkillFile: path}
	doc, err := skills.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	originalFind := findSkillMatchesFunc
	originalFetch := tryFindMetadataFetchBundleFunc
	findSkillMatchesFunc = func(_ context.Context, _ string) ([]remotepkg.SkillSearchResult, error) {
		return []remotepkg.SkillSearchResult{{Skill: remotepkg.SkillSummary{Name: "demo-skill", Source: "owner/one", SourceType: "github"}}, {Skill: remotepkg.SkillSummary{Name: "demo-skill", Source: "owner/two", SourceType: "github"}}}, nil
	}
	tryFindMetadataFetchBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		return remotepkg.SkillBundle{Root: dir, Skill: remotepkg.SkillSummary{Name: name, Source: source, SourceType: "github", RepoSkillPath: "skills/demo-skill"}, Files: []remotepkg.File{{Path: "SKILL.md", Contents: content}, {Path: "metadata.json", Contents: `{"version":"1.0.0"}`}}}, nil
	}
	t.Cleanup(func() {
		findSkillMatchesFunc = originalFind
		tryFindMetadataFetchBundleFunc = originalFetch
	})

	_, reason, err := findBestMetadataMatch(context.Background(), skill, doc, doc.ManagedMetadata())
	if err != nil {
		t.Fatalf("findBestMetadataMatch() error = %v", err)
	}
	if reason == "" {
		t.Fatalf("findBestMetadataMatch() reason = empty, want ambiguous match reason")
	}
}

func TestFindBestMetadataMatchUsesSkillContentWithoutManagedMetadataAsTieBreaker(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills-organized", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	localContent := "---\nname: demo-skill\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n    source: old/source\n---\n\n# Demo\n"
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(localContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	skill := skills.Skill{RelativePath: "demo", SkillFile: path}
	doc, err := skills.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	originalFind := findSkillMatchesFunc
	originalFetch := tryFindMetadataFetchBundleFunc
	findSkillMatchesFunc = func(_ context.Context, _ string) ([]remotepkg.SkillSearchResult, error) {
		return []remotepkg.SkillSearchResult{{Skill: remotepkg.SkillSummary{Name: "demo-skill", Source: "owner/right", SourceType: "github"}}, {Skill: remotepkg.SkillSummary{Name: "demo-skill", Source: "owner/wrong", SourceType: "github"}}}, nil
	}
	tryFindMetadataFetchBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		content := "---\nname: demo-skill\ndescription: test\n---\n\n# Different\n"
		if source == "owner/right" {
			content = "---\nname: demo-skill\ndescription: test\n---\n\n# Demo\n"
		}
		return remotepkg.SkillBundle{Root: dir, Skill: remotepkg.SkillSummary{Name: name, Source: source, SourceType: "github", RepoSkillPath: "skills/demo-skill"}, Files: []remotepkg.File{{Path: "SKILL.md", Contents: content}, {Path: "metadata.json", Contents: `{"version":"1.0.0"}`}}}, nil
	}
	t.Cleanup(func() {
		findSkillMatchesFunc = originalFind
		tryFindMetadataFetchBundleFunc = originalFetch
	})

	match, reason, err := findBestMetadataMatch(context.Background(), skill, doc, doc.ManagedMetadata())
	if err != nil {
		t.Fatalf("findBestMetadataMatch() error = %v", err)
	}
	if reason != "" {
		t.Fatalf("findBestMetadataMatch() reason = %q, want empty", reason)
	}
	if match.Result.Skill.Source != "owner/right" {
		t.Fatalf("match.Result.Skill.Source = %q, want %q", match.Result.Skill.Source, "owner/right")
	}
}

func TestTryRepairSkillMetadataWritesMissingFields(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills-organized", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: demo-skill\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n---\n\n# Demo\n"
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	skill := skills.Skill{RelativePath: "demo", FlattenedName: "demo-skill", SkillFile: path}
	originalFind := findSkillMatchesFunc
	originalFetch := tryFindMetadataFetchBundleFunc
	originalModTime := tryFindMetadataLatestModTimeFunc
	findSkillMatchesFunc = func(_ context.Context, _ string) ([]remotepkg.SkillSearchResult, error) {
		return []remotepkg.SkillSearchResult{{Skill: remotepkg.SkillSummary{Name: "demo-skill", Source: "owner/repo", SourceType: "github"}}}, nil
	}
	tryFindMetadataFetchBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		return remotepkg.SkillBundle{Root: dir, Skill: remotepkg.SkillSummary{Name: name, Source: source, SourceType: "github", RepoSkillPath: "skills/demo-skill"}, Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: demo-skill\ndescription: test\n---\n\n# Demo\n"}, {Path: "metadata.json", Contents: `{"version":"1.2.3"}`}}}, nil
	}
	tryFindMetadataLatestModTimeFunc = func(string) (time.Time, error) {
		return time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC), nil
	}
	t.Cleanup(func() {
		findSkillMatchesFunc = originalFind
		tryFindMetadataFetchBundleFunc = originalFetch
		tryFindMetadataLatestModTimeFunc = originalModTime
	})

	result, err := tryRepairSkillMetadata(context.Background(), skill)
	if err != nil {
		t.Fatalf("tryRepairSkillMetadata() error = %v", err)
	}
	if !result.Found {
		t.Fatalf("tryRepairSkillMetadata() Found = false, want true")
	}
	updated, err := skills.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() updated error = %v", err)
	}
	metadata := updated.ManagedMetadata()
	if metadata.Source != "owner/repo" {
		t.Fatalf("metadata.Source = %q, want %q", metadata.Source, "owner/repo")
	}
	if metadata.RepoSkillPath != "skills/demo-skill" {
		t.Fatalf("metadata.RepoSkillPath = %q, want %q", metadata.RepoSkillPath, "skills/demo-skill")
	}
	if metadata.InstalledVersion != "1.2.3" {
		t.Fatalf("metadata.InstalledVersion = %q, want %q", metadata.InstalledVersion, "1.2.3")
	}
	if metadata.LastUpdatedAt == "" {
		t.Fatalf("metadata.LastUpdatedAt = empty, want value")
	}
}
