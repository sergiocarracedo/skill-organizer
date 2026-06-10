package agenttools

import (
	"fmt"
	"testing"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestDetectInstalled(t *testing.T) {
	original := lookPath
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "opencode", "agcl":
			return "/usr/bin/" + file, nil
		default:
			return "", fmt.Errorf("not found")
		}
	}
	t.Cleanup(func() {
		lookPath = original
	})

	installed, err := DetectInstalled()
	if err != nil {
		t.Fatalf("DetectInstalled() error = %v", err)
	}

	if len(installed) != 3 {
		t.Fatalf("DetectInstalled() len = %d, want 3", len(installed))
	}

	if installed[0].Tool.ID != "antigravity" {
		t.Fatalf("DetectInstalled()[0].Tool.ID = %q, want %q", installed[0].Tool.ID, "antigravity")
	}
	if installed[1].Tool.ID != "claude" {
		t.Fatalf("DetectInstalled()[1].Tool.ID = %q, want %q", installed[1].Tool.ID, "claude")
	}
	if installed[2].Tool.ID != "opencode" {
		t.Fatalf("DetectInstalled()[2].Tool.ID = %q, want %q", installed[2].Tool.ID, "opencode")
	}
	if installed[0].Binary != "agcl" {
		t.Fatalf("DetectInstalled()[0].Binary = %q, want %q", installed[0].Binary, "agcl")
	}
}

func TestChooseAgentToolUsesSavedDefault(t *testing.T) {
	original := ChooseAgentToolFunc
	defer func() { ChooseAgentToolFunc = original }()

	installed := []InstalledTool{
		{Tool: supportedTools[0], Binary: "claude"},
		{Tool: supportedTools[1], Binary: "codex"},
	}

	errSelector := func(_ string, labels []string, _ string) (string, error) {
		return "", fmt.Errorf("should not be called")
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "codex"}, "", false, errSelector)
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "codex" {
		t.Fatalf("ChooseAgentTool().Tool.ID = %q, want %q", tool.Tool.ID, "codex")
	}
	if cfg.DefaultAgentTool != "codex" {
		t.Fatalf("ChooseAgentTool().DefaultAgentTool = %q, want %q", cfg.DefaultAgentTool, "codex")
	}
}

func TestChooseAgentToolUsesExplicitID(t *testing.T) {
	original := ChooseAgentToolFunc
	defer func() { ChooseAgentToolFunc = original }()

	installed := []InstalledTool{
		{Tool: supportedTools[0], Binary: "claude"},
		{Tool: supportedTools[1], Binary: "codex"},
	}

	errSelector := func(_ string, labels []string, _ string) (string, error) {
		return "", fmt.Errorf("should not be called")
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "codex"}, "claude", false, errSelector)
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "claude" {
		t.Fatalf("ChooseAgentTool().Tool.ID = %q, want %q", tool.Tool.ID, "claude")
	}
	if cfg.DefaultAgentTool != "claude" {
		t.Fatalf("ChooseAgentTool().DefaultAgentTool = %q, want %q", cfg.DefaultAgentTool, "claude")
	}
}

func TestChooseAgentToolErrorsOnMissingExplicitID(t *testing.T) {
	original := ChooseAgentToolFunc
	defer func() { ChooseAgentToolFunc = original }()

	installed := []InstalledTool{
		{Tool: supportedTools[0], Binary: "claude"},
	}

	_, _, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{}, "codex", false, nil)
	if err == nil {
		t.Fatalf("ChooseAgentTool() error = nil, want error")
	}
}

func TestSelectInstalledToolPrompts(t *testing.T) {
	original := SelectInstalledToolFunc
	defer func() { SelectInstalledToolFunc = original }()

	installed := []InstalledTool{
		{Tool: supportedTools[0], Binary: "claude"},
		{Tool: supportedTools[1], Binary: "codex"},
	}

	selector := func(_ string, labels []string, _ string) (string, error) {
		if len(labels) != 2 {
			return "", fmt.Errorf("got %d labels, want 2", len(labels))
		}
		return labels[1], nil
	}

	tool, err := SelectInstalledTool(installed, selector)
	if err != nil {
		t.Fatalf("SelectInstalledTool() error = %v", err)
	}
	if tool.Tool.ID != "codex" {
		t.Fatalf("SelectInstalledTool().Tool.ID = %q, want %q", tool.Tool.ID, "codex")
	}
}

func TestStartSpinnerAndLaunchSessionCompile(t *testing.T) {
	// Verify the function variables are set (by init) and callable.
	_, err := StartSpinner("test")
	if err != nil {
		// Expected: may fail when running in test without terminal.
		t.Logf("StartSpinner() error (expected in test env): %v", err)
	}
	if HideCursorFunc == nil {
		t.Fatal("HideCursorFunc is nil")
	}
	if ShowCursorFunc == nil {
		t.Fatal("ShowCursorFunc is nil")
	}
	if LaunchSessionFunc == nil {
		t.Fatal("LaunchSessionFunc is nil")
	}
}
