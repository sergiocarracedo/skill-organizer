package overlap

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestCollectSkillsExcludesDisabledByDefault(t *testing.T) {
	root := t.TempDir()
	createSkill(t, root, "personal/enabled", "enabled", "Enabled skill", false)
	createSkill(t, root, "personal/disabled", "disabled", "Disabled skill", true)

	items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, false)
	if err != nil {
		t.Fatalf("CollectSkills() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("CollectSkills() len = %d, want 1", len(items))
	}
	if items[0].RelativePath != "personal/enabled" {
		t.Fatalf("CollectSkills()[0].RelativePath = %q, want %q", items[0].RelativePath, "personal/enabled")
	}
}

func TestCollectSkillsIncludesDisabledWhenRequested(t *testing.T) {
	root := t.TempDir()
	createSkill(t, root, "personal/enabled", "enabled", "Enabled skill", false)
	createSkill(t, root, "personal/disabled", "disabled", "Disabled skill", true)

	items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, true)
	if err != nil {
		t.Fatalf("CollectSkills() error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("CollectSkills() len = %d, want 2", len(items))
	}
	if !items[0].Disabled && !items[1].Disabled {
		t.Fatalf("CollectSkills() did not include disabled skill")
	}
}

func TestBuildPromptIncludesStructuredInstructions(t *testing.T) {
	prompt := BuildPrompt([]SkillInfo{
		{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Finds issues in alpha.\nExplains them.",
		},
		{
			Name:          "beta",
			RelativePath:  "personal/beta",
			FlattenedName: "personal--beta",
			Description:   "",
		},
	})

	if !strings.Contains(prompt, "Return only valid JSON") {
		t.Fatalf("BuildPrompt() missing JSON instruction")
	}
	if !strings.Contains(prompt, "\"score\": 0") {
		t.Fatalf("BuildPrompt() missing score field")
	}
	if !strings.Contains(prompt, "description: \"Finds issues in alpha.\\\\nExplains them.\"") {
		t.Fatalf("BuildPrompt() missing escaped multiline description: %q", prompt)
	}
	if !strings.Contains(prompt, "description: \"No description provided.\"") {
		t.Fatalf("BuildPrompt() missing fallback description: %q", prompt)
	}
}

func TestRunParsesStructuredReport(t *testing.T) {
	original := commandRunner
	commandRunner = func(ctx context.Context, binary string, args []string, onStatus func(string)) (string, error) {
		if onStatus != nil {
			onStatus("thinking")
		}
		return `
{
  "summary": "Potential overlap detected.",
  "groups": [
    {
      "skill_names": ["alpha", "beta"],
      "skill_paths": ["thirdparty/alpha", "thirdparty/beta"],
      "score": 84,
      "why_overlap": "They cover the same checks.",
      "overlap_type": "duplicate",
      "recommendation": "Merge them."
    }
  ],
  "recommendations": ["Merge alpha and beta."]
}
`, nil
	}
	t.Cleanup(func() {
		commandRunner = original
	})

	statuses := make([]string, 0, 1)
	result, err := Run(context.Background(), mockInstalledTool("claude", "claude"), "prompt", func(status string) {
		statuses = append(statuses, status)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary != "Potential overlap detected." {
		t.Fatalf("Run().Summary = %q, want %q", result.Summary, "Potential overlap detected.")
	}
	if len(result.Groups) != 1 || result.Groups[0].Score != 84 {
		t.Fatalf("Run().Groups = %#v", result.Groups)
	}
	if len(result.Groups[0].SkillPaths) != 2 || result.Groups[0].SkillPaths[0] != "thirdparty/alpha" {
		t.Fatalf("Run().Groups[0].SkillPaths = %#v", result.Groups[0].SkillPaths)
	}
	if len(statuses) != 1 || statuses[0] != "thinking" {
		t.Fatalf("Run() statuses = %#v", statuses)
	}
	if len(result.Recommendations) != 1 || result.Recommendations[0] != "Merge alpha and beta." {
		t.Fatalf("Run().Recommendations = %#v", result.Recommendations)
	}
}

func TestRunReturnsInterruptedErrorWhenContextCanceled(t *testing.T) {
	original := commandRunner
	commandRunner = func(ctx context.Context, binary string, args []string, onStatus func(string)) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	t.Cleanup(func() {
		commandRunner = original
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, mockInstalledTool("claude", "claude"), "prompt", nil)
	if err == nil {
		t.Fatalf("Run() error = nil, want interruption error")
	}
	if !strings.Contains(err.Error(), "interrupted") && !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("Run() error = %v, want interruption-related error", err)
	}
}

func TestRunCommandReturnsInterruptedErrorWhenContextCanceled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal behavior differs on windows")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := runCommand(ctx, "sh", []string{"-c", "sleep 10"}, nil)
	if err == nil {
		t.Fatalf("runCommand() error = nil, want interruption error")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("runCommand() error = %v, want interrupted error", err)
	}
}

func TestParseReportNormalizesGroupsAndBoundsScores(t *testing.T) {
	report, err := ParseReport(`
{
  "summary": "",
  "groups": [
    {
      "skill_names": ["beta", "", "alpha"],
      "skill_paths": ["thirdparty/beta", "", "thirdparty/alpha"],
      "score": 120,
      "why_overlap": " similar workflows ",
      "overlap_type": " partial ",
      "recommendation": " clarify boundaries "
    },
    {
      "skill_names": ["gamma", "delta"],
      "score": -5,
      "why_overlap": "low overlap",
      "overlap_type": "adjacent",
      "recommendation": "keep separate"
    }
  ],
  "recommendations": ["", "Review alpha and beta."]
}`)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}

	if report.Summary != "Potential overlap detected across multiple skills." {
		t.Fatalf("ParseReport().Summary = %q", report.Summary)
	}
	if len(report.Groups) != 2 {
		t.Fatalf("ParseReport().Groups len = %d, want 2", len(report.Groups))
	}
	if report.Groups[0].Score != 100 {
		t.Fatalf("ParseReport().Groups[0].Score = %d, want 100", report.Groups[0].Score)
	}
	if report.Groups[1].Score != 0 {
		t.Fatalf("ParseReport().Groups[1].Score = %d, want 0", report.Groups[1].Score)
	}
	if len(report.Groups[0].SkillNames) != 2 {
		t.Fatalf("ParseReport().Groups[0].SkillNames = %#v", report.Groups[0].SkillNames)
	}
	if len(report.Groups[0].SkillPaths) != 2 {
		t.Fatalf("ParseReport().Groups[0].SkillPaths = %#v", report.Groups[0].SkillPaths)
	}
	if report.Groups[0].WhyOverlap != "similar workflows" {
		t.Fatalf("ParseReport().Groups[0].WhyOverlap = %q", report.Groups[0].WhyOverlap)
	}
	if len(report.Recommendations) != 1 || report.Recommendations[0] != "Review alpha and beta." {
		t.Fatalf("ParseReport().Recommendations = %#v", report.Recommendations)
	}
}

func createSkill(t *testing.T, root string, relativePath string, name string, description string, disabled bool) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	content := "---\nname: " + name + "\ndescription: " + description + "\nmetadata:\n  skill-organizer:\n    disabled: "
	if disabled {
		content += "true"
	} else {
		content += "false"
	}
	content += "\n---\n\n# Test\n"

	if err := os.WriteFile(filepath.Join(dir, skills.SkillFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func mockInstalledTool(id string, binary string) agenttools.InstalledTool {
	tool, _ := agenttools.FindSupported(id)
	return agenttools.InstalledTool{Tool: tool, Binary: binary}
}

// loadFixtureRoot copies the curated testdata/overlap/<scenario> tree into a
// fresh t.TempDir() and returns its absolute path. The test fixtures live as
// committed SKILL.md files under testdata/overlap/<scenario>/<skill>/SKILL.md;
// the copy is required because skills.ScanSource is a real directory walker
// (not a virtual one) and operates on the live filesystem.
func loadFixtureRoot(t *testing.T, scenario string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("testdata", "overlap", scenario)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read fixture dir %q: %v", src, err)
	}
	for _, entry := range entries {
		copyDir(t, filepath.Join(src, entry.Name()), filepath.Join(root, entry.Name()))
	}
	return root
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dst, err)
	}
	skillFile := filepath.Join(src, skills.SkillFileName)
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", skillFile, err)
	}
	if err := os.WriteFile(filepath.Join(dst, skills.SkillFileName), data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", filepath.Join(dst, skills.SkillFileName), err)
	}
}

func TestCollectSkillsOnConflictingFixture(t *testing.T) {
	root := loadFixtureRoot(t, "conflicting")
	items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, false)
	if err != nil {
		t.Fatalf("CollectSkills() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("CollectSkills() len = %d, want 2: %#v", len(items), items)
	}

	byPath := make(map[string]SkillInfo, len(items))
	for _, item := range items {
		byPath[item.RelativePath] = item
	}

	// loadFixtureRoot copies the inner entries of testdata/overlap/<scenario>/
	// directly into t.TempDir(), so the RelativePath returned by ScanSource is
	// the leaf name (e.g. "alpha"), not the scenario-prefixed path.
	alpha, ok := byPath["alpha"]
	if !ok {
		t.Fatalf("CollectSkills() missing alpha: %#v", items)
	}
	if alpha.Name != "release-announcer" {
		t.Fatalf("alpha Name = %q, want %q", alpha.Name, "release-announcer")
	}
	if !strings.Contains(alpha.Description, "release announcement") {
		t.Fatalf("alpha Description = %q, want containing 'release announcement'", alpha.Description)
	}
	if alpha.Disabled {
		t.Fatalf("alpha Disabled = true, want false")
	}

	beta, ok := byPath["beta"]
	if !ok {
		t.Fatalf("CollectSkills() missing beta: %#v", items)
	}
	if beta.Name != "release-notes-writer" {
		t.Fatalf("beta Name = %q, want %q", beta.Name, "release-notes-writer")
	}
	if !strings.Contains(beta.Description, "release announcement") {
		t.Fatalf("beta Description = %q, want containing 'release announcement'", beta.Description)
	}
	if beta.Disabled {
		t.Fatalf("beta Disabled = true, want false")
	}
}

func TestCollectSkillsOnCleanFixture(t *testing.T) {
	root := loadFixtureRoot(t, "clean")
	items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, false)
	if err != nil {
		t.Fatalf("CollectSkills() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("CollectSkills() len = %d, want 2: %#v", len(items), items)
	}

	byPath := make(map[string]SkillInfo, len(items))
	for _, item := range items {
		byPath[item.RelativePath] = item
	}

	alpha, ok := byPath["alpha"]
	if !ok {
		t.Fatalf("CollectSkills() missing alpha: %#v", items)
	}
	if alpha.Name != "image-resizer" {
		t.Fatalf("alpha Name = %q, want %q", alpha.Name, "image-resizer")
	}
	if !strings.Contains(alpha.Description, "resize") {
		t.Fatalf("alpha Description = %q, want containing 'resize'", alpha.Description)
	}

	beta, ok := byPath["beta"]
	if !ok {
		t.Fatalf("CollectSkills() missing beta: %#v", items)
	}
	if beta.Name != "sql-query-builder" {
		t.Fatalf("beta Name = %q, want %q", beta.Name, "sql-query-builder")
	}
	if !strings.Contains(beta.Description, "SQL") {
		t.Fatalf("beta Description = %q, want containing 'SQL'", beta.Description)
	}

	// Lock in that the two descriptions are disjoint — neither contains the
	// other skill's distinguishing keyword. This is what makes the 'clean'
	// scenario suitable for asserting an empty agent report.
	if strings.Contains(alpha.Description, "SQL") || strings.Contains(alpha.Description, "sql") {
		t.Fatalf("alpha Description should not mention SQL: %q", alpha.Description)
	}
	if strings.Contains(beta.Description, "resize") || strings.Contains(beta.Description, "image") {
		t.Fatalf("beta Description should not mention image/resize: %q", beta.Description)
	}
}

func TestCollectSkillsOnPartialFixture(t *testing.T) {
	root := loadFixtureRoot(t, "partial")
	items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, false)
	if err != nil {
		t.Fatalf("CollectSkills() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("CollectSkills() len = %d, want 3: %#v", len(items), items)
	}

	byPath := make(map[string]SkillInfo, len(items))
	for _, item := range items {
		byPath[item.RelativePath] = item
	}

	expected := []struct {
		path string
		name string
	}{
		{"alpha", "changelog-formatter"},
		{"beta", "changelog-deduplicator"},
		{"gamma", "release-summary-writer"},
	}
	for _, want := range expected {
		got, ok := byPath[want.path]
		if !ok {
			t.Fatalf("CollectSkills() missing %q: %#v", want.path, items)
		}
		if got.Name != want.name {
			t.Fatalf("%s Name = %q, want %q", want.path, got.Name, want.name)
		}
	}
}

func TestRunParsesReportWithMixedSeverities(t *testing.T) {
	original := commandRunner
	commandRunner = func(_ context.Context, _ string, _ []string, _ func(string)) (string, error) {
		// Intentionally unsorted: adjacent (30) before partial (80). Normalize
		// must sort by Score descending so the cmd-package filter (which sees
		// the normalized report) can drop the adjacent group when
		// --min-overlap-type=partial.
		return `{
  "summary": "Mixed overlap detected.",
  "groups": [
    {
      "skill_names": ["a", "b"],
      "skill_paths": ["thirdparty/a", "thirdparty/b"],
      "score": 30,
      "why_overlap": "low overlap",
      "overlap_type": "adjacent",
      "recommendation": "keep separate"
    },
    {
      "skill_names": ["c", "d"],
      "skill_paths": ["thirdparty/c", "thirdparty/d"],
      "score": 80,
      "why_overlap": "shared trigger",
      "overlap_type": "partial",
      "recommendation": "merge"
    }
  ],
  "recommendations": ["Review c and d."]
}`, nil
	}
	t.Cleanup(func() { commandRunner = original })

	result, err := Run(context.Background(), mockInstalledTool("codex", "codex"), "prompt", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("Run().Groups len = %d, want 2: %#v", len(result.Groups), result.Groups)
	}
	if result.Groups[0].OverlapType != "partial" || result.Groups[0].Score != 80 {
		t.Fatalf("Run().Groups[0] = (type=%q, score=%d), want (partial, 80)", result.Groups[0].OverlapType, result.Groups[0].Score)
	}
	if result.Groups[1].OverlapType != "adjacent" || result.Groups[1].Score != 30 {
		t.Fatalf("Run().Groups[1] = (type=%q, score=%d), want (adjacent, 30)", result.Groups[1].OverlapType, result.Groups[1].Score)
	}
}
