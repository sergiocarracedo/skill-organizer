package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atomicgo.dev/keyboard/keys"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestDiffLinesShowsAddedRemovedAndChangedFiles(t *testing.T) {
	oldFiles := []remotepkg.File{{Path: "SKILL.md", Contents: "old"}, {Path: "notes.txt", Contents: "gone"}}
	newFiles := []remotepkg.File{{Path: "SKILL.md", Contents: "new"}, {Path: "extra.txt", Contents: "added"}}

	lines := diffLines(oldFiles, newFiles)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"--- a/SKILL.md", "+++ b/SKILL.md", "+new", "-old", "--- a/notes.txt", "+++ b/extra.txt"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("diff output missing %q\n%s", want, joined)
		}
	}
}

func TestToggleAndAbortKeyHelpersRecognizeExpectedKeys(t *testing.T) {
	if !isToggleKey(keys.Key{Code: keys.Space}) {
		t.Fatalf("expected Space key to toggle")
	}
	if !isToggleKey(keys.Key{Code: keys.RuneKey, Runes: []rune{' '}}) {
		t.Fatalf("expected rune space key to toggle")
	}
	if !isInterruptKey(keys.Key{Code: keys.CtrlC}) {
		t.Fatalf("expected Ctrl+C to interrupt")
	}
	if !isBackKey(keys.Key{Code: keys.Escape}) {
		t.Fatalf("expected Escape to go back")
	}
	if !isPageUpKey(keys.Key{Code: keys.PgUp}) {
		t.Fatalf("expected PgUp to page up")
	}
	if !isPageDownKey(keys.Key{Code: keys.PgDown}) {
		t.Fatalf("expected PgDown to page down")
	}
}

func TestDisplayVersionShortensHashes(t *testing.T) {
	if got := displayVersion("006f8413941b59eff54a7ce64851b8a2fb79e7a3a5f1a895e97a48f01482553d"); got != "006f841" {
		t.Fatalf("displayVersion() = %q, want %q", got, "006f841")
	}
	if got := displayVersion("0.22.5"); got != "0.22.5" {
		t.Fatalf("displayVersion() = %q, want %q", got, "0.22.5")
	}
}

func TestDisplayUpdateDateFormatsVersionDate(t *testing.T) {
	parsed := time.Date(2026, time.May, 10, 13, 49, 48, 0, time.UTC)
	if got := displayUpdateDate(parsed); got != "2026-05-10" {
		t.Fatalf("displayUpdateDate() = %q, want %q", got, "2026-05-10")
	}
	if got := displayUpdateDate(time.Time{}); got != "" {
		t.Fatalf("displayUpdateDate() = %q, want empty", got)
	}
}

func TestRenderCheckUpdatesProgressTextIncludesSkippedCount(t *testing.T) {
	got := renderCheckUpdatesProgressText("checking", 1, 4, 2, 3, "demo-skill")
	plain := stripANSI(got)
	for _, want := range []string{"checking 1/4", "Found: 2", "Skipped: 3", "demo-skill"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderCheckUpdatesProgressText() missing %q in %q", want, plain)
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
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		if source != "terrylica/cc-skills" {
			t.Fatalf("source = %q, want %q", source, "terrylica/cc-skills")
		}
		if name != "asciinema-recorder" {
			t.Fatalf("name = %q, want %q", name, "asciinema-recorder")
		}
		return remotepkg.SkillBundle{
			Root: dir,
			Skill: remotepkg.SkillSummary{
				Source:        source,
				Hash:          "new-hash",
				RepoSkillPath: "skills/asciinema-recorder",
			},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: asciinema-recorder\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	candidates := scan.Candidates
	if len(candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(candidates))
	}
	if candidates[0].Latest != "new-hash" {
		t.Fatalf("Latest = %q, want %q", candidates[0].Latest, "new-hash")
	}
}

func TestCollectUpdateCandidatesUsesInstalledVersionMetadataWhenPresent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "google", "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: google--reports\ndescription: test\nmetadata:\n  version: 999\n  skill-organizer:\n    original-name: gws-admin-reports\n    source: owner/repo\n    repo-skill-path: skills/gws-admin-reports\n    installed-version: old-installed\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		return remotepkg.SkillBundle{
			Root:  dir,
			Skill: remotepkg.SkillSummary{Source: source, Hash: "new-installed", RepoSkillPath: "skills/gws-admin-reports"},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: gws-admin-reports\nmetadata:\n  version: 1\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(scan.Candidates))
	}
	if scan.Candidates[0].Installed != "old-installed" {
		t.Fatalf("Installed = %q, want %q", scan.Candidates[0].Installed, "old-installed")
	}
}

func TestCollectUpdateCandidatesUsesSkillFileVersionWhenPresent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "google", "gws-admin-reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: google--gws-admin-reports\ndescription: test\nmetadata:\n  version: 0.22.3\n  skill-organizer:\n    original-name: gws-admin-reports\n    source: owner/repo\n    repo-skill-path: skills/gws-admin-reports\n    installed-version: 0.22.3\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		return remotepkg.SkillBundle{
			Root:  dir,
			Skill: remotepkg.SkillSummary{Source: source, Hash: "opaque-hash", RepoSkillPath: "skills/gws-admin-reports"},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: gws-admin-reports\nmetadata:\n  version: 0.22.4\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(scan.Candidates))
	}
	if scan.Candidates[0].Installed != "0.22.3" {
		t.Fatalf("Installed = %q, want %q", scan.Candidates[0].Installed, "0.22.3")
	}
	if scan.Candidates[0].Latest != "0.22.4" {
		t.Fatalf("Latest = %q, want %q", scan.Candidates[0].Latest, "0.22.4")
	}
}

func TestCollectUpdateCandidatesNormalizesGitHubSourceURL(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "google", "gws-admin-reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: google--gws-admin-reports\ndescription: test\nmetadata:\n  version: 0.22.3\n  skill-organizer:\n    original-name: gws-admin-reports\n    source: https://github.com/googleworkspace/cli.git\n    repo-skill-path: skills/gws-admin-reports\n    installed-version: 0.22.3\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		if source != "googleworkspace/cli" {
			t.Fatalf("source = %q, want %q", source, "googleworkspace/cli")
		}
		if name != "gws-admin-reports" {
			t.Fatalf("name = %q, want %q", name, "gws-admin-reports")
		}
		return remotepkg.SkillBundle{
			Root:  dir,
			Skill: remotepkg.SkillSummary{Source: source, Hash: "opaque-hash", RepoSkillPath: "skills/gws-admin-reports"},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: gws-admin-reports\nmetadata:\n  version: 0.22.5\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(scan.Candidates))
	}
	if scan.Candidates[0].Source != "googleworkspace/cli" {
		t.Fatalf("Source = %q, want %q", scan.Candidates[0].Source, "googleworkspace/cli")
	}
	if scan.Candidates[0].Latest != "0.22.5" {
		t.Fatalf("Latest = %q, want %q", scan.Candidates[0].Latest, "0.22.5")
	}
}

func TestCollectUpdateCandidatesDoesNotRequireRepoSkillPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "google", "gws-admin-reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: google--gws-admin-reports\ndescription: test\nmetadata:\n  version: 0.22.3\n  skill-organizer:\n    original-name: gws-admin-reports\n    source: https://github.com/googleworkspace/cli\n    installed-version: 0.22.3\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		if source != "googleworkspace/cli" {
			t.Fatalf("source = %q, want %q", source, "googleworkspace/cli")
		}
		if name != "gws-admin-reports" {
			t.Fatalf("name = %q, want %q", name, "gws-admin-reports")
		}
		return remotepkg.SkillBundle{
			Root:  dir,
			Skill: remotepkg.SkillSummary{Source: source, Hash: "opaque-hash"},
			Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: gws-admin-reports\nmetadata:\n  version: 0.22.5\n---\n"}},
		}, nil
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(scan.Candidates))
	}
	if scan.Candidates[0].Latest != "0.22.5" {
		t.Fatalf("Latest = %q, want %q", scan.Candidates[0].Latest, "0.22.5")
	}
}

func TestCollectUpdateCandidatesReportsFetchFailuresAndProgress(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir1 := filepath.Join(source, "google", "reports")
	dir2 := filepath.Join(source, "google", "docs")
	for _, dir := range []string{dir1, dir2} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
	}
	content1 := "---\nname: google--reports\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: gws-admin-reports\n    source: owner/repo\n    repo-skill-path: skills/gws-admin-reports\n    installed-version: old-reports\n---\n\n# Test\n"
	content2 := "---\nname: google--docs\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: gws-docs\n    source: owner/repo\n    repo-skill-path: skills/gws-docs\n    installed-version: old-docs\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(dir1, "SKILL.md"), []byte(content1), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "SKILL.md"), []byte(content2), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalFetch := fetchSkillBundleFunc
	fetchSkillBundleFunc = func(_ context.Context, source string, name string) (remotepkg.SkillBundle, error) {
		switch name {
		case "gws-admin-reports":
			return remotepkg.SkillBundle{
				Root:  dir1,
				Skill: remotepkg.SkillSummary{Source: source, Hash: "new-reports", RepoSkillPath: "skills/gws-admin-reports"},
				Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: gws-admin-reports\n---\n"}},
			}, nil
		case "gws-docs":
			return remotepkg.SkillBundle{}, fmt.Errorf("network down")
		default:
			return remotepkg.SkillBundle{}, fmt.Errorf("unexpected skill %s", name)
		}
	}
	t.Cleanup(func() { fetchSkillBundleFunc = originalFetch })

	var progress []string
	var active []string
	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, func(checked int, total int, found int, skipped int) {
		progress = append(progress, fmt.Sprintf("%d/%d/%d/%d", checked, total, found, skipped))
	}, func(checked int, total int, found int, skipped int, relativePath string) {
		active = append(active, fmt.Sprintf("%d/%d/%d/%d:%s", checked, total, found, skipped, relativePath))
	})
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 1 {
		t.Fatalf("collectUpdateCandidates() len = %d, want 1", len(scan.Candidates))
	}
	if len(scan.Failures) != 1 {
		t.Fatalf("Failures len = %d, want 1", len(scan.Failures))
	}
	if scan.Failures[0].RelativePath != "google/docs" || !strings.Contains(scan.Failures[0].Reason, "network down") {
		t.Fatalf("Failures = %#v", scan.Failures)
	}
	if len(progress) == 0 || progress[0] != "0/2/0/0" || progress[len(progress)-1] != "2/2/1/0" {
		t.Fatalf("progress = %#v", progress)
	}
	if len(active) == 0 || active[0] != "0/2/0/0:google/docs" && active[0] != "0/2/0/0:google/reports" {
		t.Fatalf("active = %#v", active)
	}
}

func TestCollectUpdateCandidatesReportsSkippedMissingMetadata(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	dir := filepath.Join(source, "thirdparty", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: thirdparty--demo\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	scan, err := collectUpdateCandidates(context.Background(), configpkg.Location{Source: source, Target: target}, nil, nil)
	if err != nil {
		t.Fatalf("collectUpdateCandidates() error = %v", err)
	}
	if len(scan.Candidates) != 0 {
		t.Fatalf("Candidates len = %d, want 0", len(scan.Candidates))
	}
	if len(scan.Skipped) != 1 {
		t.Fatalf("Skipped len = %d, want 1", len(scan.Skipped))
	}
	if scan.Skipped[0].RelativePath != "thirdparty/demo" {
		t.Fatalf("Skipped relative path = %q, want %q", scan.Skipped[0].RelativePath, "thirdparty/demo")
	}
	if !strings.Contains(scan.Skipped[0].Reason, "metadata.skill-organizer.source") {
		t.Fatalf("Skipped reason = %q", scan.Skipped[0].Reason)
	}
	if got := updateSkipReason(skills.ManagedMetadata{Source: "owner/repo"}); got != "" {
		t.Fatalf("updateSkipReason() = %q, want empty", got)
	}
}

func TestCheckUpdatesHelpLineStylesKeys(t *testing.T) {
	line := checkUpdatesHelpLine("Space: Toggle, 🡹/🡻: Move, PgUp/PgDown: Page, d: Diff, Enter: Continue, Ctrl+C: Abort")
	for _, want := range []string{"Space", "🡹/🡻", "PgUp/PgDown", "d", "Enter", "Ctrl+C"} {
		if !strings.Contains(line, want) {
			t.Fatalf("help line missing %q: %q", want, line)
		}
	}
}

func TestRefreshSkillUpdateCacheWritesPendingEntries(t *testing.T) {
	updatesPath := filepath.Join(t.TempDir(), ".updates")
	original := updatesPathFunc
	updatesPathFunc = func() (string, error) { return updatesPath, nil }
	t.Cleanup(func() { updatesPathFunc = original })

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
	content, err := os.ReadFile(updatesPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	for _, want := range []string{"update-count: 1", "relative-path: thirdparty/example", "installed-version: old", "latest-version: new", "source: owner/repo"} {
		if !strings.Contains(text, want) {
			t.Fatalf("updates content missing %q\n%s", want, text)
		}
	}
}

func TestExistingSkillNamesIncludesOriginalAndManagedNames(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "thirdparty", "asciinema")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: thirdparty--asciinema\ndescription: test\nmetadata:\n  skill-organizer:\n    original-name: asciinema-recorder\n---\n\n# Test\n"
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	items := existingSkillsByName([]skills.Skill{{RelativePath: "thirdparty/asciinema", SkillFile: path}})
	for _, key := range []string{"asciinema", "asciinema-recorder", "thirdparty--asciinema"} {
		if _, ok := items[key]; !ok {
			t.Fatalf("existingSkillNames() missing %q", key)
		}
	}
}
