package main_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"
	"github.com/creack/pty"
	"gopkg.in/yaml.v3"
)

const testTimeout = 90 * time.Second

type cliEnv struct {
	t              *testing.T
	root           string
	home           string
	configHome     string
	cacheHome      string
	binDir         string
	workspace      string
	source         string
	target         string
	configPath     string
	updatesPath    string
	fixtureRoot    string
	publicRepoRoot string
	binaryPath     string
	env            []string
	statePath      string
	mu             sync.Mutex
	state          fakeSkillsState
}

type fakeSkillsState struct {
	Installed map[string]fakeInstalledSkill `json:"installed"`
}

type fakeInstalledSkill struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Path          string `json:"path"`
	RepoSkillPath string `json:"repoSkillPath"`
	Hash          string `json:"hash"`
	Version       string `json:"version,omitempty"`
	Date          string `json:"date,omitempty"`
}

type updatesState struct {
	UpdateCount int                  `yaml:"update-count,omitempty"`
	Pending     []updatesStateRecord `yaml:"pending,omitempty"`
}

type updatesStateRecord struct {
	Source string `yaml:"source,omitempty"`
}

type interactiveStep struct {
	waitFor  string
	waitForAny []string
	send     string
}

func TestMoveUnmanagedBinary(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.writeTargetSkill("orphan-skill", "orphan-skill", map[string]any{"metadata": map[string]any{"version": "0.1.0"}}, "# Orphan\n")

	output := env.run(t, env.workspace, nil, "skill", "move-unmanaged", "--yes", "--to", "3rdparty/demo/orphan-skill")
	assertContains(t, output, "Moved 1 unmanaged target entries")
	assertExists(t, filepath.Join(env.source, "3rdparty", "demo", "orphan-skill", "SKILL.md"))
	assertNotExists(t, filepath.Join(env.target, "orphan-skill"))
	assertSymlinkTargetContains(t, filepath.Join(env.target, "3rdparty--demo--orphan-skill"), filepath.Join("..", "skills-organized", "3rdparty", "demo", "orphan-skill"))

	manifest := env.readFile(filepath.Join(env.target, ".skill-organizer.manifest.yml"))
	assertContains(t, manifest, "3rdparty--demo--orphan-skill: 3rdparty/demo/orphan-skill")
}

func TestSkillDeleteWildcardBinary(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.writeSourceSkill("google/gws-admin-reports", "google--gws-admin-reports", map[string]any{"metadata": map[string]any{"version": "1.0.0"}}, "# Reports\n")
	env.writeSourceSkill("google/gws-docs", "google--gws-docs", map[string]any{"metadata": map[string]any{"version": "1.0.0"}}, "# Docs\n")
	env.run(t, env.workspace, nil, "sync")

	output := env.run(t, env.workspace, nil, "delete", "google/*", "--yes", "--no-backup")
	assertContains(t, output, "Deleted skills: 2")
	assertNotExists(t, filepath.Join(env.source, "google", "gws-admin-reports"))
	assertNotExists(t, filepath.Join(env.source, "google", "gws-docs"))
	assertNotExists(t, filepath.Join(env.target, "google--gws-admin-reports"))
	assertNotExists(t, filepath.Join(env.target, "google--gws-docs"))
}

func TestSkillAddAndCheckUpdatesBinary(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.writeFakeSkillsFixtures("owner/repo", map[string]fakeFixtureSkill{
		"demo-skill": {
			Version:       "1.0.0",
			Hash:          "hash-100",
			RepoSkillPath: "skills/demo-skill",
			Body:          "# Demo v1\n",
		},
	})

	addOutput := env.runInteractive(t, env.workspace, nil, []interactiveStep{
		{waitForAny: []string{"Set the target folders for the imported skills.", "demo-skill -> skills-organized/"}, send: "\r"},
		{waitForAny: []string{"check-security", "No agent tools detected"}, send: "\r"},
	}, "skill", "add", "owner/repo", "--skill", "demo-skill")
	assertContains(t, addOutput, "Imported skill: demo-skill -> demo-skill")
	installed := env.readFile(filepath.Join(env.source, "demo-skill", "SKILL.md"))
	assertContains(t, installed, "installed-version: 1.0.0")
	assertSymlinkTargetContains(t, filepath.Join(env.target, "demo-skill"), filepath.Join("..", "skills-organized", "demo-skill"))

	env.writeFakeSkillsFixtures("owner/repo", map[string]fakeFixtureSkill{
		"demo-skill": {
			Version:       "1.1.0",
			Hash:          "hash-110",
			RepoSkillPath: "skills/demo-skill",
			Body:          "# Demo v2\n",
		},
	})

	updateOutput := env.run(t, env.workspace, nil, "check-updates", "--yes")
	assertContains(t, updateOutput, "Updated skill: demo-skill (1.0.0 -> 1.1.0)")
	updated := env.readFile(filepath.Join(env.source, "demo-skill", "SKILL.md"))
	assertContains(t, updated, "installed-version: 1.1.0")
	assertContains(t, updated, "# Demo v2")

	state := env.loadUpdatesState()
	if state.UpdateCount != 1 {
		t.Fatalf("update-count = %d, want 1", state.UpdateCount)
	}
	backupRoot := filepath.Join(env.configHome, "skill-organizer", ".old")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", backupRoot, err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected backup entry in %s", backupRoot)
	}
	metadata := env.findBackupMetadata(backupRoot)
	assertContains(t, metadata, "updated-from: 1.0.0")
	assertContains(t, metadata, "updated-to: 1.1.0")
	assertContains(t, metadata, "original-path: demo-skill")
}

func TestSkillAddWithRealNpxSkillsSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real npx smoke in short mode")
	}
	if os.Getenv("SKILL_ORGANIZER_E2E_REAL_NPX") == "" {
		t.Skip("set SKILL_ORGANIZER_E2E_REAL_NPX=1 to run real npx skills smoke")
	}
	t.Parallel()

	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.removeFakeSkillsShim()

	npxOutput := env.runRaw(t, env.workspace, env.realSkillsEnv(), "npx", "skills", "add", "https://github.com/vercel-labs/agent-browser", "--skill", "agent-browser", "-y", "--copy")
	assertContains(t, strings.ToLower(npxOutput), "agent-browser")
	listOutput := env.runRaw(t, env.workspace, env.realSkillsEnv(), "npx", "skills", "list", "--json")
	assertContains(t, listOutput, "agent-browser")

	cliOutput := env.runInteractive(t, env.workspace, env.realSkillsEnv(), []interactiveStep{{waitForAny: []string{"Set the target folders for the imported skills.", "agent-browser -> skills-organized/"}, send: "\r"}}, "skill", "add", "vercel-labs/agent-browser", "--skill", "agent-browser")
	assertContains(t, cliOutput, "Imported skill: agent-browser")
	installed := env.readFile(filepath.Join(env.source, "agent-browser", "SKILL.md"))
	assertContains(t, installed, "original-name: agent-browser")
	assertContains(t, installed, "source: vercel-labs/agent-browser")
	assertContains(t, installed, "repo-skill-path: skills/agent-browser")
	assertSymlinkTargetContains(t, filepath.Join(env.target, "agent-browser"), filepath.Join("..", "skills-organized", "agent-browser"))
}

func TestSkillTryFindMetadataBinary(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.writeFakeSkillsFixtures("owner/repo", map[string]fakeFixtureSkill{
		"demo-skill": {
			Version:       "1.1.0",
			Hash:          "hash-110",
			RepoSkillPath: "skills/demo-skill",
			Body:          "# Demo\n",
		},
	})
	env.writeSourceSkill("demo-skill", "demo-skill", map[string]any{
		"metadata": map[string]any{
			"skill-organizer": map[string]any{
				"original-name": "demo-skill",
			},
		},
	}, "# Demo\n")

	output := env.run(t, env.workspace, nil, "skill", "try-find-metadata")
	assertContains(t, output, "Updated metadata for demo-skill using owner/repo@demo-skill")
	installed := env.readFile(filepath.Join(env.source, "demo-skill", "SKILL.md"))
	assertContains(t, installed, "source: owner/repo")
	assertContains(t, installed, "repo-skill-path: skills/demo-skill")
	assertContains(t, installed, "installed-version: 1.1.0")

	env.writeFakeSkillsFixtures("owner/repo", map[string]fakeFixtureSkill{
		"demo-skill": {
			Version:       "1.2.0",
			Hash:          "hash-120",
			RepoSkillPath: "skills/demo-skill",
			Body:          "# Demo\n",
		},
	})
	updateOutput := env.run(t, env.workspace, nil, "check-updates", "--yes")
	assertContains(t, updateOutput, "Found: 1")
	state := env.loadUpdatesState()
	if state.UpdateCount != 1 {
		t.Fatalf("update-count = %d, want 1", state.UpdateCount)
	}
	if len(state.Pending) != 1 || state.Pending[0].Source != "owner/repo" {
		t.Fatalf("pending = %#v", state.Pending)
	}
}

func TestSkillTryFindMetadataSkipsUnresolvedBinary(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	env.writeProjectConfig()
	env.writeSourceSkill("demo-skill", "demo-skill", map[string]any{
		"metadata": map[string]any{
			"skill-organizer": map[string]any{
				"original-name": "demo-skill",
			},
		},
	}, "# Demo\n")
	output := env.run(t, env.workspace, nil, "skill", "try-find-metadata")
	assertContains(t, output, "Skipped demo-skill: could not identify upstream source")
	installed := env.readFile(filepath.Join(env.source, "demo-skill", "SKILL.md"))
	if strings.Contains(installed, "source:") {
		t.Fatalf("expected SKILL.md not to gain source metadata\n%s", installed)
	}
	state := env.loadUpdatesState()
	if state.UpdateCount != 0 {
		t.Fatalf("update-count = %d, want 0", state.UpdateCount)
	}
}

type fakeFixtureSkill struct {
	Version       string
	Hash          string
	RepoSkillPath string
	Body          string
}

func newCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	configHome := filepath.Join(root, "xdg-config")
	cacheHome := filepath.Join(root, "xdg-cache")
	binDir := filepath.Join(root, "bin")
	workspace := filepath.Join(root, "workspace")
	source := filepath.Join(home, ".agents", "skills-organized")
	target := filepath.Join(home, ".agents", "skills")
	configPath := filepath.Join(home, ".agents", ".skill-organizer.yml")
	updatesPath := filepath.Join(configHome, "skill-organizer", ".updates")
	fixtureRoot := filepath.Join(root, "fixtures")
	publicRepoRoot := filepath.Join(root, "public-repos")
	statePath := filepath.Join(root, "fake-skills-state.yml")
	for _, dir := range []string{home, configHome, cacheHome, binDir, workspace, source, target, fixtureRoot, publicRepoRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	binaryPath := buildBinary(t, root)
	env := &cliEnv{
		t:              t,
		root:           root,
		home:           home,
		configHome:     configHome,
		cacheHome:      cacheHome,
		binDir:         binDir,
		workspace:      workspace,
		source:         source,
		target:         target,
		configPath:     configPath,
		updatesPath:    updatesPath,
		fixtureRoot:    fixtureRoot,
		publicRepoRoot: publicRepoRoot,
		binaryPath:     binaryPath,
		statePath:      statePath,
		state:          fakeSkillsState{Installed: map[string]fakeInstalledSkill{}},
	}
	env.env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+configHome,
		"XDG_CACHE_HOME="+cacheHome,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	env.writeFile(filepath.Join(configHome, "skill-organizer", ".cache.yml"), "last-checked-at: \"2100-01-01T00:00:00Z\"\nlatest-version: dev\nlatest-page-url: https://example.invalid/releases/latest\n")
	// Pre-create the telemetry-prompted sentinel so the
	// PersistentPreRun does not fire the first-run prompt during
	// e2e tests (the test PTY does not drive that prompt and the
	// binary would block on it). Default answer is "no" so
	// telemetry is opt-out for the e2e environment.
	if err := os.WriteFile(filepath.Join(configHome, "skill-organizer", "telemetry-prompted"), []byte("no"), 0o644); err != nil {
		t.Fatalf("WriteFile(telemetry-prompted) error = %v", err)
	}
	env.saveFakeSkillsState()
	env.writeFakeSkillsShim()
	return env
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	binaryName := "skill-organizer"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(root, binaryName)
	repoCLIPath := repoCLIPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "./main.go")
	cmd.Dir = repoCLIPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build error = %v\n%s", err, string(output))
	}
	return binaryPath
}

func (e *cliEnv) writeProjectConfig() {
	e.writeFile(e.configPath, fmt.Sprintf("source: %s\ntarget: %s\n", e.source, e.target))
	if err := os.MkdirAll(filepath.Dir(e.configPath), 0o755); err != nil {
		e.t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(e.configPath), err)
	}
	registryPath := filepath.Join(e.configHome, "skill-organizer", "skill-organizer.yml")
	e.writeFile(registryPath, fmt.Sprintf("watched:\n  - %s\n", e.configPath))
	e.writeFile(e.updatesPath, "last-checked-at: \"2100-01-01T00:00:00Z\"\nupdate-count: 0\n")
}

func (e *cliEnv) writeSourceSkill(relativePath string, name string, frontmatter map[string]any, body string) {
	dir := filepath.Join(e.source, filepath.FromSlash(relativePath))
	e.writeSkill(dir, name, frontmatter, body)
}

func (e *cliEnv) writeTargetSkill(relativePath string, name string, frontmatter map[string]any, body string) {
	dir := filepath.Join(e.target, filepath.FromSlash(relativePath))
	e.writeSkill(dir, name, frontmatter, body)
}

func (e *cliEnv) writeSkill(dir string, name string, frontmatter map[string]any, body string) {
	e.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	meta := map[string]any{"name": name, "description": "test skill"}
	for key, value := range frontmatter {
		meta[key] = value
	}
	encoded, err := yaml.Marshal(meta)
	if err != nil {
		e.t.Fatalf("yaml.Marshal() error = %v", err)
	}
	content := "---\n" + string(encoded) + "---\n\n" + body
	e.writeFile(filepath.Join(dir, "SKILL.md"), content)
}

func (e *cliEnv) writeFakeSkillsFixtures(source string, skills map[string]fakeFixtureSkill) {
	e.t.Helper()
	root := filepath.Join(e.fixtureRoot, sanitizeSource(source))
	if err := os.RemoveAll(root); err != nil {
		e.t.Fatalf("RemoveAll(%q) error = %v", root, err)
	}
	for name, skill := range skills {
		dir := filepath.Join(root, "skills", name)
		frontmatter := map[string]any{"metadata": map[string]any{"version": skill.Version}}
		e.writeSkill(dir, name, frontmatter, skill.Body)
		metaJSON := fmt.Sprintf("{\n  \"version\": %q,\n  \"date\": \"2026-01-02T03:04:05Z\"\n}\n", skill.Version)
		e.writeFile(filepath.Join(dir, "metadata.json"), metaJSON)
	}
	lockLines := []string{"{", "  \"skills\": {"}
	index := 0
	for name, skill := range skills {
		comma := ","
		if index == len(skills)-1 {
			comma = ""
		}
		lockLines = append(lockLines,
			fmt.Sprintf("    %q: {", name),
			fmt.Sprintf("      \"source\": %q,", source),
			"      \"sourceType\": \"github\",",
			fmt.Sprintf("      \"skillPath\": %q,", filepath.ToSlash(filepath.Join(skill.RepoSkillPath, "SKILL.md"))),
			fmt.Sprintf("      \"computedHash\": %q", skill.Hash),
			"    }"+comma,
		)
		index++
	}
	lockLines = append(lockLines, "  }", "}")
	e.writeFile(filepath.Join(root, "skills-lock.json"), strings.Join(lockLines, "\n")+"\n")
	publicRoot := filepath.Join(e.publicRepoRoot, sanitizeSource(source))
	if err := os.RemoveAll(publicRoot); err != nil {
		e.t.Fatalf("RemoveAll(%q) error = %v", publicRoot, err)
	}
	if err := os.MkdirAll(filepath.Dir(publicRoot), 0o755); err != nil {
		e.t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(publicRoot), err)
	}
	if err := os.Symlink(root, publicRoot); err != nil {
		e.t.Fatalf("Symlink(%q -> %q) error = %v", publicRoot, root, err)
	}
}

func (e *cliEnv) writeFakeSkillsShim() {
	e.t.Helper()
	repoCLIPath := repoCLIPath(e.t)
	shim := fmt.Sprintf("#!/usr/bin/env sh\nset -eu\norig_dir=\"$PWD\"\ncd \"%s\"\nSKILL_ORGANIZER_FAKE_SKILLS_WORKDIR=\"$orig_dir\" exec \"%s\" run ./testdata/fake-skills-cli.go \"$@\"\n", repoCLIPath, commandForShell("go"))
	e.writeExecutable(filepath.Join(e.binDir, "skills"), shim)
	if runtime.GOOS == "windows" {
		e.writeExecutable(filepath.Join(e.binDir, "skills.cmd"), fmt.Sprintf("@echo off\r\nset SKILL_ORGANIZER_FAKE_SKILLS_WORKDIR=%%CD%%\r\ncd /d \"%s\"\r\ngo run ./testdata/fake-skills-cli.go %%*\r\n", repoCLIPath))
	}
}

func repoCLIPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	return wd
}

func (e *cliEnv) removeFakeSkillsShim() {
	for _, name := range []string{"skills", "skills.cmd"} {
		_ = os.Remove(filepath.Join(e.binDir, name))
	}
}

func (e *cliEnv) realSkillsEnv() []string {
	env := append([]string{}, e.env...)
	for i, item := range env {
		if strings.HasPrefix(item, "PATH=") {
			env[i] = "PATH=" + os.Getenv("PATH")
		}
	}
	return env
}

func (e *cliEnv) run(t *testing.T, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, e.env...), extraEnv...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s error = %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func (e *cliEnv) runRaw(t *testing.T, dir string, extraEnv []string, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, e.env...), extraEnv...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s error = %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func (e *cliEnv) runInteractive(t *testing.T, dir string, extraEnv []string, steps []interactiveStep, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, e.env...), extraEnv...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start() error = %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	var output bytes.Buffer
	var outputMu sync.Mutex
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := bufio.NewReader(ptmx).WriteTo(writerFunc(func(p []byte) (int, error) {
			outputMu.Lock()
			defer outputMu.Unlock()
			return output.Write(p)
		}))
		readDone <- copyErr
	}()
	for _, step := range steps {
		needles := append([]string{}, step.waitForAny...)
		if strings.TrimSpace(step.waitFor) != "" {
			needles = append([]string{step.waitFor}, needles...)
		}
		if err := waitForOutput(ctx, &output, &outputMu, needles...); err != nil {
			t.Fatalf("%s interactive wait error = %v\n%s", strings.Join(args, " "), err, snapshotOutput(&output, &outputMu))
		}
		if _, err := ptmx.Write([]byte(step.send)); err != nil {
			t.Fatalf("pty write error = %v", err)
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("%s interactive error = %v\n%s", strings.Join(args, " "), err, snapshotOutput(&output, &outputMu))
	}
	_ = ptmx.Close()
	readErr := <-readDone
	if readErr != nil && !errors.Is(readErr, os.ErrClosed) && !isBenignPTYReadError(readErr) {
		t.Fatalf("pty read error = %v\n%s", readErr, snapshotOutput(&output, &outputMu))
	}
	return snapshotOutput(&output, &outputMu)
}

type writerFunc func(p []byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) {
	return w(p)
}

func waitForOutput(ctx context.Context, output *bytes.Buffer, outputMu *sync.Mutex, needles ...string) error {
	filtered := make([]string, 0, len(needles))
	for _, needle := range needles {
		if strings.TrimSpace(needle) != "" {
			filtered = append(filtered, needle)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := snapshotOutput(output, outputMu)
		for _, needle := range filtered {
			if strings.Contains(snapshot, needle) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for one of %q", strings.Join(filtered, ", "))
		case <-ticker.C:
		}
	}
}

func snapshotOutput(output *bytes.Buffer, outputMu *sync.Mutex) string {
	outputMu.Lock()
	defer outputMu.Unlock()
	return output.String()
}

func isBenignPTYReadError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "input/output error")
}

func (e *cliEnv) loadUpdatesState() updatesState {
	e.t.Helper()
	var state updatesState
	content := e.readFile(e.updatesPath)
	if err := yaml.Unmarshal([]byte(content), &state); err != nil {
		e.t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	return state
}

func (e *cliEnv) findBackupMetadata(root string) string {
	e.t.Helper()
	var result string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != ".skill-organizer-backup.yml" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		result = string(content)
		return filepath.SkipAll
	})
	if err != nil {
		e.t.Fatalf("WalkDir(%q) error = %v", root, err)
	}
	if result == "" {
		e.t.Fatalf("backup metadata not found in %s", root)
	}
	return result
}

func (e *cliEnv) readFile(path string) string {
	e.t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}

func (e *cliEnv) writeFile(path string, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func (e *cliEnv) writeExecutable(path string, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		e.t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func (e *cliEnv) saveFakeSkillsState() {
	e.t.Helper()
	content, err := yaml.Marshal(e.state)
	if err != nil {
		e.t.Fatalf("yaml.Marshal() error = %v", err)
	}
	e.writeFile(e.statePath, string(content))
	for _, item := range e.env {
		if strings.HasPrefix(item, "SKILL_ORGANIZER_FAKE_SKILLS_STATE=") {
			return
		}
	}
	e.env = append(e.env,
		"SKILL_ORGANIZER_FAKE_SKILLS_STATE="+e.statePath,
		"SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES="+e.fixtureRoot,
		"SKILL_ORGANIZER_FAKE_PUBLIC_REPOS="+e.publicRepoRoot,
	)
}

func sanitizeSource(source string) string {
	replacer := strings.NewReplacer("/", "__", ":", "__", ".", "_", "-", "-")
	return replacer.Replace(strings.TrimSpace(source))
}

func commandForShell(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	return path
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("output missing %q\n%s", needle, haystack)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, stat err = %v", path, err)
	}
}

func assertSymlinkTargetContains(t *testing.T, linkPath string, want string) {
	t.Helper()
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink(%q) error = %v", linkPath, err)
	}
	if !strings.Contains(filepath.ToSlash(target), filepath.ToSlash(want)) {
		t.Fatalf("Readlink(%q) = %q, want to contain %q", linkPath, target, want)
	}
}

// telemetryConfigPath is the on-disk path the binary reads/writes for
// the telemetry subcommand. It's a helper because e2e tests need to
// pre-write the YAML for the disabled / status subcommand cases.
func telemetryConfigPath(env *cliEnv) string {
	return filepath.Join(env.configHome, "skill-organizer", "skill-organizer.yml")
}

// TestTelemetryFirstRunPromptFiresOnce exercises the e2e flow:
//  1. fresh XDG_CONFIG_HOME with the sentinel removed
//  2. first run fires the pterm confirm prompt
//  3. pressing Enter (default = no) writes the sticky YAML
//  4. second run does NOT fire the prompt (sentinel short-circuits)
func TestTelemetryFirstRunPromptFiresOnce(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)
	// Remove the sentinel newCLIEnv pre-created; we want the
	// first-run prompt to actually fire on the first run.
	sentinel := filepath.Join(env.configHome, "skill-organizer", "telemetry-prompted")
	if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Remove(%q) error = %v", sentinel, err)
	}

	// First run: prompt fires; press Enter to accept the default
	// ("no" — opt-out).
	output := env.runInteractive(t, env.workspace, nil, []interactiveStep{
		{waitForAny: []string{"Enable anonymous", "Enable"}, send: "\r"},
	}, "sync")
	if !strings.Contains(output, "Enable anonymous") {
		t.Fatalf("first run output missing the first-run prompt text 'Enable anonymous'\noutput:\n%s", output)
	}

	// The sticky answer is written. The TelemetryConfig has
	// `omitempty` on the `telemetry` parent key, so the YAML may or
	// may not contain the `telemetry:` line depending on whether
	// any inner field is non-zero. The strong evidence that the
	// answer was persisted is: (1) the registry YAML file was
	// created/touched, and (2) the sentinel file exists. We assert
	// both.
	registryYAML := telemetryConfigPath(env)
	if _, err := os.Stat(registryYAML); err != nil {
		t.Fatalf("registry YAML not created at %q after first run: %v", registryYAML, err)
	}
	content, err := os.ReadFile(registryYAML)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v", registryYAML, err)
	}
	// The YAML must parse as a valid AppConfig — that's the
	// structural assertion. (The exact key set varies with omitempty.)
	var parsed configpkg.AppConfig
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("registry YAML is not a valid AppConfig: %v\nyaml:\n%s", err, string(content))
	}

	// The sentinel file is created.
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not created after first run: %v", err)
	}

	// Second run: prompt does NOT fire.
	output2 := env.run(t, env.workspace, nil, "sync")
	if strings.Contains(output2, "Enable anonymous") {
		t.Fatalf("second run still shows the first-run prompt (sentinel should short-circuit)\noutput:\n%s", output2)
	}
}

// TestTelemetryDisabledNoBuffer asserts the "disabled" state never
// writes the on-disk buffer (per REQ-8 acceptance: zero network
// egress and zero on-disk writes when disabled).
//
// Phase 5 REQ-10: the Identity module is removed, so install_id
// and host_id files must NOT be created in any state.
func TestTelemetryDisabledNoBuffer(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)

	// Pre-write the registry YAML with telemetry disabled.
	registryYAML := telemetryConfigPath(env)
	if err := os.MkdirAll(filepath.Dir(registryYAML), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(registryYAML), err)
	}
	env.writeFile(registryYAML, "telemetry:\n  enabled: false\n")

	// Run a command; the buffer file must not be created.
	_ = env.run(t, env.workspace, nil, "sync")

	bufferPath := filepath.Join(env.configHome, "skill-organizer", telemetrypkg.BufferFileName)
	if _, err := os.Stat(bufferPath); !os.IsNotExist(err) {
		t.Fatalf("telemetry buffer file was created at %q even though telemetry is disabled (stat err = %v)", bufferPath, err)
	}

	// Phase 5 REQ-10: install_id and host_id files are NOT
	// created. The Identity module is removed.
	for _, name := range []string{"install_id", "host_id"} {
		path := filepath.Join(env.configHome, "skill-organizer", name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s file must NOT be created (Phase 5 REQ-10 drops the identity module); stat err = %v", name, err)
		}
	}
}

// TestTelemetryStatusSubcommandE2E asserts the `telemetry status`
// subcommand prints the Phase 5 REQ-10 two-line output (Enabled,
// Recorder) when telemetry is enabled. The recorder type falls
// back to NoopRecorder in the e2e binary (no build-time creds
// are baked in for the test build).
func TestTelemetryStatusSubcommandE2E(t *testing.T) {
	t.Parallel()
	env := newCLIEnv(t)

	// Pre-write the registry YAML with telemetry enabled.
	registryYAML := telemetryConfigPath(env)
	if err := os.MkdirAll(filepath.Dir(registryYAML), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(registryYAML), err)
	}
	env.writeFile(registryYAML, "telemetry:\n  enabled: true\n")

	output := env.run(t, env.workspace, nil, "telemetry", "status")

	for _, needle := range []string{"Enabled:", "true", "Recorder:", "NoopRecorder"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("telemetry status output missing %q\noutput:\n%s", needle, output)
		}
	}
	// The OLD (Phase 3/4) 8-line fields are NOT present.
	banned := []string{"Endpoint:", "Account ID:", "Insert key:", "Install ID:", "Host ID:", "Buffer file:"}
	for _, b := range banned {
		if strings.Contains(output, b) {
			t.Fatalf("telemetry status output must not contain %q (Phase 5 REQ-10 collapses to 2 lines)\noutput:\n%s", b, output)
		}
	}
}
