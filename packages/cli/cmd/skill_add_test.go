package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type fakeSkillAddSandbox struct {
	before       []remotepkg.InstalledSkill
	after        []remotepkg.InstalledSkill
	bundles      map[string]remotepkg.SkillBundle
	runArgs      []string
	runCalls     int
	closed       bool
	installedRun int
}

func (f *fakeSkillAddSandbox) Close() {
	f.closed = true
}

func (f *fakeSkillAddSandbox) InstalledSkills() ([]remotepkg.InstalledSkill, error) {
	f.installedRun++
	if f.installedRun == 1 {
		return f.before, nil
	}
	return f.after, nil
}

func (f *fakeSkillAddSandbox) RunInteractive(args ...string) error {
	f.runCalls++
	f.runArgs = append([]string{}, args...)
	return nil
}

func (f *fakeSkillAddSandbox) LoadInstalledBundle(skill remotepkg.SkillSummary) (remotepkg.SkillBundle, error) {
	bundle, ok := f.bundles[skill.Name]
	if !ok {
		return remotepkg.SkillBundle{}, os.ErrNotExist
	}
	return bundle, nil
}

func TestSkillAddSkipsReinstallWhenDeclined(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	existingDir := filepath.Join(source, "thirdparty", "demo")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oldContents := "---\nname: thirdparty--demo\ndescription: existing\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n    installed-version: old-hash\n---\n\n# Old\n"
	if err := os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte(oldContents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &fakeSkillAddSandbox{
		before: nil,
		after:  []remotepkg.InstalledSkill{{Name: "demo-skill"}},
		bundles: map[string]remotepkg.SkillBundle{
			"demo-skill": {
				Root: filepath.Join(root, "bundle-demo-skill"),
				Skill: remotepkg.SkillSummary{
					Name:          "demo-skill",
					Source:        "owner/repo",
					SourceType:    "github",
					RepoSkillPath: "skills/demo-skill",
					Hash:          "new-hash",
				},
				Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: demo-skill\ndescription: imported\n---\n\n# New\n"}},
			},
		},
	}

	restore := stubSkillAddDependencies(t, skillAddDeps{
		location: configpkg.Location{Source: source, Target: target},
		sandbox:  sandbox,
		targets: func(installed []remotepkg.InstalledSkill, suggestions []string) (map[string]string, error) {
			return map[string]string{"demo-skill": "new/location"}, nil
		},
		confirm: func(prompt string, defaultValue bool) (bool, error) {
			return false, nil
		},
	})
	defer restore()

	cmd := newSkillAddCommand()
	if err := cmd.RunE(cmd, []string{"owner/repo"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	current, err := os.ReadFile(filepath.Join(existingDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	currentText := string(current)
	for _, want := range []string{
		"name: thirdparty--demo",
		"original-name: demo-skill",
		"installed-version: old-hash",
		"# Old",
	} {
		if !strings.Contains(currentText, want) {
			t.Fatalf("existing skill missing %q\n%s", want, currentText)
		}
	}
	for _, unwanted := range []string{"installed-version: new-hash", "# New"} {
		if strings.Contains(currentText, unwanted) {
			t.Fatalf("existing skill should not contain %q\n%s", unwanted, currentText)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "new", "location")); !os.IsNotExist(err) {
		t.Fatalf("prompt target should not be created, stat err = %v", err)
	}
	backupRoot := backupRootForTests(t)
	assertNoBackupEntries(t, backupRoot)
	if sandbox.runCalls != 1 {
		t.Fatalf("RunInteractive() calls = %d, want 1", sandbox.runCalls)
	}
	if got, want := sandbox.runArgs, []string{"add", "owner/repo"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("RunInteractive() args = %#v, want %#v", got, want)
	}
	if !sandbox.closed {
		t.Fatalf("sandbox.Close() was not called")
	}
	if _, err := os.Stat(filepath.Join(target, "thirdparty--demo")); err != nil {
		t.Fatalf("expected sync target link to exist: %v", err)
	}
}

func TestSkillAddExplicitExistingSkillSkipsExternalInstallWhenDeclined(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	existingDir := filepath.Join(source, "thirdparty", "demo")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oldContents := "---\nname: thirdparty--demo\ndescription: existing\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n    installed-version: old-hash\n---\n\n# Old\n"
	if err := os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte(oldContents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &fakeSkillAddSandbox{}
	confirmCalls := 0
	restore := stubSkillAddDependencies(t, skillAddDeps{
		location: configpkg.Location{Source: source, Target: target},
		sandbox:  sandbox,
		confirm: func(prompt string, defaultValue bool) (bool, error) {
			confirmCalls++
			return false, nil
		},
	})
	defer restore()

	cmd := newSkillAddCommand()
	if err := cmd.Flags().Set("skill", "demo-skill"); err != nil {
		t.Fatalf("Flags().Set() error = %v", err)
	}
	if err := cmd.RunE(cmd, []string{"owner/repo"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if confirmCalls != 1 {
		t.Fatalf("confirm calls = %d, want 1", confirmCalls)
	}
	if sandbox.runCalls != 0 {
		t.Fatalf("RunInteractive() calls = %d, want 0", sandbox.runCalls)
	}
	current, err := os.ReadFile(filepath.Join(existingDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(current), "installed-version: old-hash") {
		t.Fatalf("existing skill should keep old version\n%s", string(current))
	}
	assertNoBackupEntries(t, backupRootForTests(t))
	if _, err := os.Stat(filepath.Join(target, "thirdparty--demo")); err != nil {
		t.Fatalf("expected sync target link to exist: %v", err)
	}
	if !sandbox.closed {
		t.Fatalf("sandbox.Close() was not called")
	}
}

func TestSkillAddExplicitExistingSkillConfirmsReinstallOnlyOnce(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	existingDir := filepath.Join(source, "thirdparty", "demo")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oldContents := "---\nname: thirdparty--demo\ndescription: existing\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n    installed-version: old-hash\n    source: owner/repo\n    repo-skill-path: skills/demo-skill\n---\n\n# Old\n"
	if err := os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte(oldContents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	bundleRoot := filepath.Join(root, "bundle-demo-skill")
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundleFile := filepath.Join(bundleRoot, "SKILL.md")
	if err := os.WriteFile(bundleFile, []byte("# Bundle\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &fakeSkillAddSandbox{
		before: nil,
		after:  []remotepkg.InstalledSkill{{Name: "demo-skill"}},
		bundles: map[string]remotepkg.SkillBundle{
			"demo-skill": {
				Root: bundleRoot,
				Skill: remotepkg.SkillSummary{
					Name:          "demo-skill",
					Source:        "owner/repo",
					SourceType:    "github",
					RepoSkillPath: "skills/demo-skill",
					Hash:          "new-hash",
				},
				Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: demo-skill\ndescription: imported\n---\n\n# New\n"}},
			},
		},
	}
	confirmCalls := 0
	restore := stubSkillAddDependencies(t, skillAddDeps{
		location: configpkg.Location{Source: source, Target: target},
		sandbox:  sandbox,
		targets: func(installed []remotepkg.InstalledSkill, suggestions []string) (map[string]string, error) {
			return map[string]string{"demo-skill": "new/location"}, nil
		},
		confirm: func(prompt string, defaultValue bool) (bool, error) {
			confirmCalls++
			return true, nil
		},
	})
	defer restore()

	cmd := newSkillAddCommand()
	if err := cmd.Flags().Set("skill", "demo-skill"); err != nil {
		t.Fatalf("Flags().Set() error = %v", err)
	}
	if err := cmd.RunE(cmd, []string{"owner/repo"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if confirmCalls != 1 {
		t.Fatalf("confirm calls = %d, want 1", confirmCalls)
	}
	if sandbox.runCalls != 1 {
		t.Fatalf("RunInteractive() calls = %d, want 1", sandbox.runCalls)
	}
	if got, want := sandbox.runArgs, []string{"add", "owner/repo", "--skill", "demo-skill"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("RunInteractive() args = %#v, want %#v", got, want)
	}
}

func TestSkillAddReinstallsExistingManagedSkillInPlace(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	source := filepath.Join(root, "skills-organized")
	target := filepath.Join(root, "skills")
	existingDir := filepath.Join(source, "thirdparty", "demo")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	oldContents := "---\nname: thirdparty--demo\ndescription: existing\nmetadata:\n  skill-organizer:\n    original-name: demo-skill\n    installed-version: old-hash\n    source: owner/repo\n    repo-skill-path: skills/demo-skill\n---\n\n# Old\n"
	if err := os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte(oldContents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	bundleRoot := filepath.Join(root, "bundle-demo-skill")
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	bundleTime := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	bundleFile := filepath.Join(bundleRoot, "SKILL.md")
	if err := os.WriteFile(bundleFile, []byte("# Bundle\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chtimes(bundleRoot, bundleTime, bundleTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if err := os.Chtimes(bundleFile, bundleTime, bundleTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	sandbox := &fakeSkillAddSandbox{
		before: nil,
		after:  []remotepkg.InstalledSkill{{Name: "demo-skill"}},
		bundles: map[string]remotepkg.SkillBundle{
			"demo-skill": {
				Root: bundleRoot,
				Skill: remotepkg.SkillSummary{
					Name:          "demo-skill",
					Source:        "owner/repo",
					SourceType:    "github",
					RepoSkillPath: "skills/demo-skill",
					Hash:          "new-hash",
				},
				Files: []remotepkg.File{{Path: "SKILL.md", Contents: "---\nname: demo-skill\ndescription: imported\n---\n\n# New\n"}},
			},
		},
	}

	restore := stubSkillAddDependencies(t, skillAddDeps{
		location: configpkg.Location{Source: source, Target: target},
		sandbox:  sandbox,
		targets: func(installed []remotepkg.InstalledSkill, suggestions []string) (map[string]string, error) {
			return map[string]string{"demo-skill": "new/location"}, nil
		},
		confirm: func(prompt string, defaultValue bool) (bool, error) {
			return true, nil
		},
	})
	defer restore()

	cmd := newSkillAddCommand()
	if err := cmd.RunE(cmd, []string{"owner/repo"}); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	updatedPath := filepath.Join(existingDir, "SKILL.md")
	updated, err := os.ReadFile(updatedPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(updated)
	for _, want := range []string{
		"name: thirdparty--demo",
		"original-name: demo-skill",
		"installed-version: new-hash",
		"source: owner/repo",
		"repo-skill-path: skills/demo-skill",
		"last-updated-at: \"2026-01-02T03:04:05Z\"",
		"# New",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated skill missing %q\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "new", "location")); !os.IsNotExist(err) {
		t.Fatalf("reinstall should reuse existing path, stat err = %v", err)
	}
	entries := readBackupEntries(t, backupRootForTests(t))
	if len(entries) != 1 {
		t.Fatalf("backup entries = %d, want 1", len(entries))
	}
	backupDir := entries[0]
	backupSkill, err := os.ReadFile(filepath.Join(backupDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() backup error = %v", err)
	}
	if string(backupSkill) != oldContents {
		t.Fatalf("backup skill contents changed unexpectedly:\n%s", string(backupSkill))
	}
	metadata, err := os.ReadFile(filepath.Join(backupDir, ".skill-organizer-backup.yml"))
	if err != nil {
		t.Fatalf("ReadFile() backup metadata error = %v", err)
	}
	for _, want := range []string{
		"original-path: thirdparty/demo",
		"flattened-name: thirdparty--demo",
		"updated-from: old-hash",
		"updated-to: new-hash",
	} {
		if !strings.Contains(string(metadata), want) {
			t.Fatalf("backup metadata missing %q\n%s", want, string(metadata))
		}
	}
	if _, err := os.Stat(filepath.Join(target, "thirdparty--demo")); err != nil {
		t.Fatalf("expected sync target link to exist: %v", err)
	}
	if !sandbox.closed {
		t.Fatalf("sandbox.Close() was not called")
	}
}

type skillAddDeps struct {
	location configpkg.Location
	sandbox  skillAddSandbox
	targets  func(installed []remotepkg.InstalledSkill, suggestions []string) (map[string]string, error)
	confirm  func(prompt string, defaultValue bool) (bool, error)
}

func stubSkillAddDependencies(t *testing.T, deps skillAddDeps) func() {
	t.Helper()

	originalLoad := skillAddLoadResolvedLocation
	originalDetect := detectSkillsCLIFunc
	originalSandbox := newSkillAddSandbox
	originalSuggestions := skillAddSourceFolderSuggestions
	originalTargets := chooseSkillAddTargets
	originalConfirm := confirmSkillAddReinstall
	originalSync := runSkillAddSync

	skillAddLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return filepath.Join(t.TempDir(), ".skill-organizer.yml"), deps.location, nil
	}
	detectSkillsCLIFunc = func() (*remotepkg.SkillsCLIRunner, error) {
		return &remotepkg.SkillsCLIRunner{}, nil
	}
	newSkillAddSandbox = func() (skillAddSandbox, error) {
		return deps.sandbox, nil
	}
	skillAddSourceFolderSuggestions = func(sourceRoot string) ([]string, error) {
		return []string{"", "thirdparty/demo"}, nil
	}
	if deps.targets != nil {
		chooseSkillAddTargets = deps.targets
	}
	if deps.confirm != nil {
		confirmSkillAddReinstall = deps.confirm
	}
	runSkillAddSync = syncpkg.Run

	return func() {
		skillAddLoadResolvedLocation = originalLoad
		detectSkillsCLIFunc = originalDetect
		newSkillAddSandbox = originalSandbox
		skillAddSourceFolderSuggestions = originalSuggestions
		chooseSkillAddTargets = originalTargets
		confirmSkillAddReinstall = originalConfirm
		runSkillAddSync = originalSync
	}
}

func readBackupEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := backuppkg.List(root)
	if err != nil {
		t.Fatalf("backup.List() error = %v", err)
	}
	return entries
}

func assertNoBackupEntries(t *testing.T, root string) {
	t.Helper()
	entries := readBackupEntries(t, root)
	if len(entries) != 0 {
		t.Fatalf("backup entries = %#v, want none", entries)
	}
}

func backupRootForTests(t *testing.T) string {
	t.Helper()
	appDir, err := configpkg.AppDir()
	if err != nil {
		t.Fatalf("config.AppDir() error = %v", err)
	}
	return filepath.Join(appDir, ".old")
}
