package cmd

import (
	"fmt"
	"strings"
	"testing"

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

	cmd := newOverlapCommand()
	overlapPrintPrompt = true
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if printed == "" {
		t.Fatalf("RunE() output is empty")
	}
	if !strings.Contains(printed, "## Potential Overlap Groups") {
		t.Fatalf("RunE() output did not contain generated prompt: %q", printed)
	}
}

func mockInstalledTool(id string, binary string) agenttools.InstalledTool {
	tool, _ := agenttools.FindSupported(id)
	return agenttools.InstalledTool{Tool: tool, Binary: binary}
}
