package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pterm/pterm"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/overlap"
)

func TestChooseOverlapToolUsesSavedInstalledDefault(t *testing.T) {
	installed := []agenttools.InstalledTool{
		mockInstalledTool("claude", "claude"),
		mockInstalledTool("codex", "codex"),
	}

	tool, cfg, err := chooseOverlapTool(installed, configpkg.OverlapConfig{DefaultAgentTool: "codex"}, "", false)
	if err != nil {
		t.Fatalf("chooseOverlapTool() error = %v", err)
	}

	if tool.Tool.ID != "codex" {
		t.Fatalf("chooseOverlapTool().Tool.ID = %q, want %q", tool.Tool.ID, "codex")
	}
	if cfg.DefaultAgentTool != "codex" {
		t.Fatalf("chooseOverlapTool().DefaultAgentTool = %q, want %q", cfg.DefaultAgentTool, "codex")
	}
}

func TestChooseOverlapToolUsesExplicitInstalledTool(t *testing.T) {
	installed := []agenttools.InstalledTool{
		mockInstalledTool("claude", "claude"),
		mockInstalledTool("codex", "codex"),
	}

	tool, cfg, err := chooseOverlapTool(installed, configpkg.OverlapConfig{DefaultAgentTool: "claude"}, "codex", false)
	if err != nil {
		t.Fatalf("chooseOverlapTool() error = %v", err)
	}

	if tool.Tool.ID != "codex" {
		t.Fatalf("chooseOverlapTool().Tool.ID = %q, want %q", tool.Tool.ID, "codex")
	}
	if cfg.DefaultAgentTool != "codex" {
		t.Fatalf("chooseOverlapTool().DefaultAgentTool = %q, want %q", cfg.DefaultAgentTool, "codex")
	}
}

func TestChooseOverlapToolErrorsWhenExplicitToolIsMissing(t *testing.T) {
	installed := []agenttools.InstalledTool{mockInstalledTool("claude", "claude")}

	_, _, err := chooseOverlapTool(installed, configpkg.OverlapConfig{}, "codex", false)
	if err == nil {
		t.Fatalf("chooseOverlapTool() error = nil, want error")
	}
}

func TestChooseOverlapToolPromptsWhenRequested(t *testing.T) {
	original := selectToolOption
	selectToolOption = func(prompt string, options []string, defaultOption string) (string, error) {
		if len(options) != 2 {
			return "", fmt.Errorf("got %d options", len(options))
		}
		return options[1], nil
	}
	t.Cleanup(func() {
		selectToolOption = original
	})

	installed := []agenttools.InstalledTool{
		mockInstalledTool("claude", "claude"),
		mockInstalledTool("codex", "codex"),
	}

	tool, cfg, err := chooseOverlapTool(installed, configpkg.OverlapConfig{DefaultAgentTool: "claude"}, "", true)
	if err != nil {
		t.Fatalf("chooseOverlapTool() error = %v", err)
	}

	if tool.Tool.ID != "codex" {
		t.Fatalf("chooseOverlapTool().Tool.ID = %q, want %q", tool.Tool.ID, "codex")
	}
	if cfg.DefaultAgentTool != "codex" {
		t.Fatalf("chooseOverlapTool().DefaultAgentTool = %q, want %q", cfg.DefaultAgentTool, "codex")
	}
}

func TestOverlapPrintPromptBypassesToolDetection(t *testing.T) {
	originalChooseTool := overlapChooseTool
	originalToolID := overlapToolID
	originalAllSkills := overlapAllSkills
	originalPrintPrompt := overlapPrintPrompt
	originalDetectInstalled := detectInstalledTools
	originalLoadResolvedLocation := loadResolvedLocationFunc
	originalCollectSkills := collectOverlapSkills
	originalPrintPromptFunc := printOverlapPromptFunc

	overlapChooseTool = false
	overlapToolID = ""
	overlapAllSkills = false
	overlapPrintPrompt = false
	detectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return nil, fmt.Errorf("should not be called")
	}
	loadResolvedLocationFunc = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	collectOverlapSkills = func(location configpkg.Location, includeDisabled bool) ([]overlap.SkillInfo, error) {
		return []overlap.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	printed := ""
	printOverlapPromptFunc = func(prompt string) {
		printed = prompt
	}
	t.Cleanup(func() {
		overlapChooseTool = originalChooseTool
		overlapToolID = originalToolID
		overlapAllSkills = originalAllSkills
		overlapPrintPrompt = originalPrintPrompt
		detectInstalledTools = originalDetectInstalled
		loadResolvedLocationFunc = originalLoadResolvedLocation
		collectOverlapSkills = originalCollectSkills
		printOverlapPromptFunc = originalPrintPromptFunc
	})

	cmd := newCheckOverlapCommand()
	overlapPrintPrompt = true
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if printed == "" {
		t.Fatalf("RunE() output is empty")
	}
	if !strings.Contains(printed, "Return only valid JSON") {
		t.Fatalf("RunE() output did not contain generated prompt: %q", printed)
	}
}

func TestPrintOverlapReportWrapsLongLines(t *testing.T) {
	long := "This is a deliberately long explanation that should be wrapped across multiple lines so the rendered overlap report stays readable within the configured width."
	wrapped := wrapText(long, 80)
	if len(wrapped) < 2 {
		t.Fatalf("wrapText() produced %d lines, want at least 2", len(wrapped))
	}
	for _, line := range wrapped {
		if visibleRuneWidth(line) > 80 {
			t.Fatalf("wrapText() line width = %d, want <= 80 for %q", visibleRuneWidth(line), line)
		}
	}
}

func TestLimitSpinnerTextTruncatesLongValues(t *testing.T) {
	text := limitSpinnerText("abcdefghijklmnopqrstuvwxyz", 10)
	if text != "abcdefg..." {
		t.Fatalf("limitSpinnerText() = %q, want %q", text, "abcdefg...")
	}
}

func TestParseMinOverlapTypeAcceptsTextAndNumbers(t *testing.T) {
	tests := []struct {
		input     string
		wantRank  int
		wantLabel string
	}{
		{input: "adjacent", wantRank: 1, wantLabel: "adjacent"},
		{input: "1", wantRank: 1, wantLabel: "adjacent"},
		{input: "partial", wantRank: 2, wantLabel: "partial"},
		{input: "2", wantRank: 2, wantLabel: "partial"},
		{input: "duplicate", wantRank: 3, wantLabel: "duplicate"},
		{input: "3", wantRank: 3, wantLabel: "duplicate"},
	}

	for _, test := range tests {
		rank, label, err := parseMinOverlapType(test.input)
		if err != nil {
			t.Fatalf("parseMinOverlapType(%q) error = %v", test.input, err)
		}
		if rank != test.wantRank || label != test.wantLabel {
			t.Fatalf("parseMinOverlapType(%q) = (%d, %q), want (%d, %q)", test.input, rank, label, test.wantRank, test.wantLabel)
		}
	}
}

func TestParseMinOverlapTypeRejectsInvalidValue(t *testing.T) {
	if _, _, err := parseMinOverlapType("4"); err == nil {
		t.Fatalf("parseMinOverlapType() error = nil, want error")
	}
}

func TestFilterOverlapGroupsHidesAdjacentByDefault(t *testing.T) {
	groups := []overlap.Group{
		{OverlapType: "adjacent", Score: 20},
		{OverlapType: "partial", Score: 60},
		{OverlapType: "duplicate", Score: 90},
	}

	filtered := filterOverlapGroups(groups, 2)
	if len(filtered) != 2 {
		t.Fatalf("filterOverlapGroups() len = %d, want 2", len(filtered))
	}
	if filtered[0].OverlapType != "partial" || filtered[1].OverlapType != "duplicate" {
		t.Fatalf("filterOverlapGroups() = %#v", filtered)
	}
}

func TestFormatBoxContentIncludesExpectedSections(t *testing.T) {
	content := formatBoxContent(overlap.Group{
		SkillNames:     []string{"alpha", "beta"},
		SkillPaths:     []string{"thirdparty/alpha", "thirdparty/beta"},
		Score:          71,
		WhyOverlap:     "They share the same purpose.",
		OverlapType:    "partial",
		Recommendation: "Clarify boundaries.",
	})

	if !strings.Contains(content, "Skills") {
		t.Fatalf("formatBoxContent() missing Skills label: %q", content)
	}
	if !strings.Contains(content, "thirdparty/alpha") || !strings.Contains(content, "thirdparty/beta") {
		t.Fatalf("formatBoxContent() missing relative skill paths: %q", content)
	}
	if !strings.Contains(content, "- ") {
		t.Fatalf("formatBoxContent() missing skill bullets: %q", content)
	}
	if !strings.Contains(content, "Overlap") || !strings.Contains(content, "71/100") || !strings.Contains(content, "Partial") {
		t.Fatalf("formatBoxContent() missing unified overlap summary: %q", content)
	}
	if !strings.Contains(content, "Why the overlap") {
		t.Fatalf("formatBoxContent() missing why section: %q", content)
	}
}

func TestChooseOverlapToolReportHelpersCompile(t *testing.T) {
	_ = styleLabel("Why the overlap")
	_ = overlapScoreStyle(95)
	_ = overlapTypeStyle("duplicate")
	_, _ = startDefaultSpinner("Testing spinner")
	_ = pterm.DefaultSpinner
}

func TestCompletionCommandIncludesSupportedShells(t *testing.T) {
	cmd := newCompletionCommand()
	subs := cmd.Commands()
	if len(subs) != 4 {
		t.Fatalf("newCompletionCommand() subcommands = %d, want 4", len(subs))
	}
	got := make(map[string]bool, len(subs))
	for _, sub := range subs {
		got[sub.Use] = true
	}
	for _, want := range []string{"bash", "zsh", "fish", "powershell"} {
		if !got[want] {
			t.Fatalf("newCompletionCommand() missing %q in %#v", want, got)
		}
	}
}

func TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode(t *testing.T) {
	originalChooseTool := overlapChooseTool
	originalToolID := overlapToolID
	originalAllSkills := overlapAllSkills
	originalPrintPrompt := overlapPrintPrompt
	originalNoAsk := overlapNoAskToApply
	originalDetectInstalled := detectInstalledTools
	originalLoadResolvedLocation := loadResolvedLocationFunc
	originalCollectSkills := collectOverlapSkills
	originalLoadConfig := loadOverlapConfigFunc
	originalSaveConfig := saveOverlapConfigFunc
	originalRunOverlap := runOverlapAnalysis
	originalConfirm := confirmApplyPlan
	originalConfirmCosts := confirmExternalCosts
	originalSavePrompt := saveApplyPlanPrompt
	originalLaunch := launchPlanSession
	originalInfo := printInfoMessage
	originalDebug := printDebugMessage
	originalWarning := printWarningMessage
	originalSpinner := startOverlapSpinner

	overlapChooseTool = false
	overlapToolID = ""
	overlapAllSkills = false
	overlapPrintPrompt = false
	overlapNoAskToApply = false
	detectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return []agenttools.InstalledTool{mockInstalledTool("opencode", "opencode")}, nil
	}
	loadResolvedLocationFunc = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	collectOverlapSkills = func(location configpkg.Location, includeDisabled bool) ([]overlap.SkillInfo, error) {
		return []overlap.SkillInfo{{Name: "alpha", RelativePath: "personal/alpha", FlattenedName: "personal--alpha", Description: "Alpha description"}}, nil
	}
	loadOverlapConfigFunc = func(path string) (configpkg.OverlapConfig, error) {
		return configpkg.OverlapConfig{DefaultAgentTool: "opencode", AcknowledgedExternalToolCosts: true}, nil
	}
	saveOverlapConfigFunc = func(path string, cfg configpkg.OverlapConfig) error {
		return nil
	}
	runOverlapAnalysis = func(_ context.Context, _ agenttools.InstalledTool, _ string, _ func(string)) (overlap.Report, error) {
		return overlap.Report{Groups: []overlap.Group{{SkillNames: []string{"alpha", "beta"}, SkillPaths: []string{"personal/alpha", "personal/beta"}, Score: 72, OverlapType: "partial", WhyOverlap: "They overlap.", Recommendation: "Separate them."}}}, nil
	}
	var questions []string
	confirmApplyPlan = func(prompt string, defaultValue bool) (bool, error) {
		questions = append(questions, prompt)
		return true, nil
	}
	confirmExternalCosts = func(prompt string, defaultValue bool) (bool, error) {
		return true, nil
	}
	savedPath := ""
	savedPrompt := ""
	saveApplyPlanPrompt = func(prompt string) (string, error) {
		savedPrompt = prompt
		savedPath = "/abs/plans/skill-overlap-fix-20262804-120304.md"
		return savedPath, nil
	}
	launchPlanSession = func(tool agenttools.InstalledTool, prompt string) error {
		return fmt.Errorf("launchPlanSession should not be called")
	}
	var infos []string
	var debugs []string
	printInfoMessage = func(format string, args ...any) {
		infos = append(infos, fmt.Sprintf(format, args...))
	}
	printDebugMessage = func(format string, args ...any) {
		debugs = append(debugs, fmt.Sprintf(format, args...))
	}
	printWarningMessage = func(format string, args ...any) {
	}
	startOverlapSpinner = func(text string) (spinnerHandle, error) {
		return stubSpinner{}, nil
	}
	t.Cleanup(func() {
		overlapChooseTool = originalChooseTool
		overlapToolID = originalToolID
		overlapAllSkills = originalAllSkills
		overlapPrintPrompt = originalPrintPrompt
		overlapNoAskToApply = originalNoAsk
		detectInstalledTools = originalDetectInstalled
		loadResolvedLocationFunc = originalLoadResolvedLocation
		collectOverlapSkills = originalCollectSkills
		loadOverlapConfigFunc = originalLoadConfig
		saveOverlapConfigFunc = originalSaveConfig
		runOverlapAnalysis = originalRunOverlap
		confirmApplyPlan = originalConfirm
		confirmExternalCosts = originalConfirmCosts
		saveApplyPlanPrompt = originalSavePrompt
		launchPlanSession = originalLaunch
		printInfoMessage = originalInfo
		printDebugMessage = originalDebug
		printWarningMessage = originalWarning
		startOverlapSpinner = originalSpinner
	})

	cmd := newCheckOverlapCommand()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if len(questions) != 1 || questions[0] != "Generate a prompt to apply the recommendations?" {
		t.Fatalf("confirmApplyPlan prompts = %#v", questions)
	}
	if savedPrompt == "" {
		t.Fatalf("saveApplyPlanPrompt() prompt is empty")
	}
	if !strings.Contains(savedPrompt, "Do not modify files") {
		t.Fatalf("saved prompt missing apply-plan instructions: %q", savedPrompt)
	}
	if savedPath == "" {
		t.Fatalf("saveApplyPlanPrompt() path is empty")
	}
	if !containsLine(debugs, "OpenCode has no verified interactive plan mode.") {
		t.Fatalf("debug messages = %#v", debugs)
	}
	if !containsLine(infos, "Saved apply-plan prompt: "+savedPath) {
		t.Fatalf("info messages = %#v", infos)
	}
}

func TestWriteApplyPlanPromptCreatesTimestampedFile(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	path, err := writeApplyPlanPrompt("test prompt")
	if err != nil {
		t.Fatalf("writeApplyPlanPrompt() error = %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("writeApplyPlanPrompt() path = %q, want absolute path", path)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "skill-overlap-fix-") || !strings.HasSuffix(base, ".md") {
		t.Fatalf("writeApplyPlanPrompt() file = %q", base)
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(base, "skill-overlap-fix-"), ".md"), "-")
	if len(parts) != 2 {
		t.Fatalf("writeApplyPlanPrompt() file = %q", base)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 6 {
		t.Fatalf("writeApplyPlanPrompt() timestamp parts = %#v", parts)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "test prompt" {
		t.Fatalf("writeApplyPlanPrompt() content = %q, want %q", string(content), "test prompt")
	}
}

func containsLine(lines []string, target string) bool {
	for _, line := range lines {
		if line == target {
			return true
		}
	}
	return false
}

type stubSpinner struct{}

func (stubSpinner) UpdateText(text string) {}
func (stubSpinner) Success(text ...any)    {}
func (stubSpinner) Fail(text ...any)       {}

func mockInstalledTool(id string, binary string) agenttools.InstalledTool {
	tool, _ := agenttools.FindSupported(id)
	return agenttools.InstalledTool{Tool: tool, Binary: binary}
}
