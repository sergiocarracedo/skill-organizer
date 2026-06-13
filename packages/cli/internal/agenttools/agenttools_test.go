package agenttools

import (
	"fmt"
	"os/exec"
	"strings"
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

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "codex"}, "", false, errSelector, "")
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

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "codex"}, "claude", false, errSelector, "")
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

	_, _, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{}, "codex", false, nil, "")
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

func TestQueryToolModels_ReturnsNilWhenListModelsNil(t *testing.T) {
	tool := InstalledTool{
		Tool: Tool{
			ID:         "test-tool",
			ListModels: nil,
		},
		Binary: "/usr/bin/test-tool",
	}

	models, err := QueryToolModels(tool)
	if err != nil {
		t.Fatalf("QueryToolModels() error = %v, want nil", err)
	}
	if models != nil {
		t.Fatalf("QueryToolModels() models = %v, want nil", models)
	}
}

func TestQueryToolModels_ReturnsModelsForOpenCode(t *testing.T) {
	// Swap execCommand with a fake that returns known model names, one per line.
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("printf", "anthropic/claude-sonnet-4\nopenai/gpt-4o\ngoogle/gemini-2.0-flash\n")
	}
	t.Cleanup(func() {
		execCommand = origExec
	})

	tool := InstalledTool{
		Tool: Tool{
			ID: "opencode",
			ListModels: func(binary string) ([]string, error) {
				out, err := execCommand(binary, "models").Output()
				if err != nil {
					return nil, fmt.Errorf("query opencode models: %w", err)
				}
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				models := make([]string, 0, len(lines))
				for _, line := range lines {
					if trimmed := strings.TrimSpace(line); trimmed != "" {
						models = append(models, trimmed)
					}
				}
				return models, nil
			},
		},
		Binary: "/usr/bin/opencode",
	}

	models, err := QueryToolModels(tool)
	if err != nil {
		t.Fatalf("QueryToolModels() error = %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("QueryToolModels() len = %d, want 3: %v", len(models), models)
	}
	if models[0] != "anthropic/claude-sonnet-4" {
		t.Fatalf("QueryToolModels()[0] = %q, want %q", models[0], "anthropic/claude-sonnet-4")
	}
	if models[1] != "openai/gpt-4o" {
		t.Fatalf("QueryToolModels()[1] = %q, want %q", models[1], "openai/gpt-4o")
	}
	if models[2] != "google/gemini-2.0-flash" {
		t.Fatalf("QueryToolModels()[2] = %q, want %q", models[2], "google/gemini-2.0-flash")
	}
}

func TestQueryToolModels_ReturnsErrorOnFailedCommand(t *testing.T) {
	origExec := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	t.Cleanup(func() {
		execCommand = origExec
	})

	tool := InstalledTool{
		Tool: Tool{
			ID: "opencode",
			ListModels: func(binary string) ([]string, error) {
				out, err := execCommand(binary, "models").Output()
				if err != nil {
					return nil, fmt.Errorf("query opencode models: %w", err)
				}
				_ = out
				return nil, nil
			},
		},
		Binary: "/usr/bin/opencode",
	}

	_, err := QueryToolModels(tool)
	if err == nil {
		t.Fatalf("QueryToolModels() error = nil, want error for failed command")
	}
}

func TestChooseAgentTool_ExplicitModelFlag(t *testing.T) {
	origChoose := ChooseAgentToolFunc
	origQuery := QueryToolModelsFunc
	defer func() {
		ChooseAgentToolFunc = origChoose
		QueryToolModelsFunc = origQuery
	}()

	// Should not call QueryToolModels when explicitModel is set.
	QueryToolModelsFunc = func(_ InstalledTool) ([]string, error) {
		return nil, fmt.Errorf("should not be called")
	}

	installed := []InstalledTool{
		{Tool: Tool{ID: "opencode", Name: "OpenCode", ListModels: func(_ string) ([]string, error) { return nil, nil }}, Binary: "opencode"},
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{}, "opencode", false, nil, "anthropic/claude-sonnet-4")
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "opencode" {
		t.Fatalf("tool ID = %q, want %q", tool.Tool.ID, "opencode")
	}
	if cfg.DefaultModel != "anthropic/claude-sonnet-4" {
		t.Fatalf("DefaultModel = %q, want %q", cfg.DefaultModel, "anthropic/claude-sonnet-4")
	}
}

func TestChooseAgentTool_NoModelPromptWhenToolHasNoModels(t *testing.T) {
	origChoose := ChooseAgentToolFunc
	origQuery := QueryToolModelsFunc
	defer func() {
		ChooseAgentToolFunc = origChoose
		QueryToolModelsFunc = origQuery
	}()

	// QueryToolModels should not be called since claude has ListModels: nil.
	QueryToolModelsFunc = func(_ InstalledTool) ([]string, error) {
		return nil, fmt.Errorf("should not be called")
	}

	installed := []InstalledTool{
		{Tool: supportedTools[0], Binary: "claude"}, // claude has ListModels: nil
	}

	// Use saved default to skip interactive tool selection.
	errSelector := func(_ string, _ []string, _ string) (string, error) {
		return "", fmt.Errorf("should not be called")
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "claude"}, "", false, errSelector, "")
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "claude" {
		t.Fatalf("tool ID = %q, want %q", tool.Tool.ID, "claude")
	}
	if cfg.DefaultModel != "" {
		t.Fatalf("DefaultModel = %q, want empty string", cfg.DefaultModel)
	}
}

func TestChooseAgentTool_SelectsModelWhenToolExposesModels(t *testing.T) {
	origChoose := ChooseAgentToolFunc
	origQuery := QueryToolModelsFunc
	defer func() {
		ChooseAgentToolFunc = origChoose
		QueryToolModelsFunc = origQuery
	}()

	QueryToolModelsFunc = func(_ InstalledTool) ([]string, error) {
		return []string{"anthropic/claude-sonnet-4", "openai/gpt-4o"}, nil
	}

	installed := []InstalledTool{
		{Tool: Tool{ID: "opencode", Name: "OpenCode", ListModels: func(_ string) ([]string, error) { return nil, nil }}, Binary: "opencode"},
	}

	// The selector is called for model selection, not tool selection
	// (tool is resolved via saved default).
	selector := func(_ string, labels []string, defaultOption string) (string, error) {
		if len(labels) != 2 {
			return "", fmt.Errorf("got %d labels, want 2", len(labels))
		}
		return labels[1], nil
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "opencode"}, "", false, selector, "")
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "opencode" {
		t.Fatalf("tool ID = %q, want %q", tool.Tool.ID, "opencode")
	}
	if cfg.DefaultModel != "openai/gpt-4o" {
		t.Fatalf("DefaultModel = %q, want %q", cfg.DefaultModel, "openai/gpt-4o")
	}
}

func TestChooseAgentTool_UsesDefaultModel(t *testing.T) {
	origChoose := ChooseAgentToolFunc
	origQuery := QueryToolModelsFunc
	defer func() {
		ChooseAgentToolFunc = origChoose
		QueryToolModelsFunc = origQuery
	}()

	QueryToolModelsFunc = func(_ InstalledTool) ([]string, error) {
		return []string{"anthropic/claude-sonnet-4", "openai/gpt-4o"}, nil
	}

	installed := []InstalledTool{
		{Tool: Tool{ID: "opencode", Name: "OpenCode", ListModels: func(_ string) ([]string, error) { return nil, nil }}, Binary: "opencode"},
	}

	// DefaultModel is in the list, so no prompt should be shown.
	errSelector := func(_ string, _ []string, _ string) (string, error) {
		return "", fmt.Errorf("should not be called")
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "opencode", DefaultModel: "anthropic/claude-sonnet-4"}, "", false, errSelector, "")
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "opencode" {
		t.Fatalf("tool ID = %q, want %q", tool.Tool.ID, "opencode")
	}
	if cfg.DefaultModel != "anthropic/claude-sonnet-4" {
		t.Fatalf("DefaultModel = %q, want %q", cfg.DefaultModel, "anthropic/claude-sonnet-4")
	}
}

func TestChooseAgentTool_ModelQueryErrorShowsToolWithoutModel(t *testing.T) {
	origChoose := ChooseAgentToolFunc
	origQuery := QueryToolModelsFunc
	defer func() {
		ChooseAgentToolFunc = origChoose
		QueryToolModelsFunc = origQuery
	}()

	QueryToolModelsFunc = func(_ InstalledTool) ([]string, error) {
		return nil, fmt.Errorf("query failed")
	}

	installed := []InstalledTool{
		{Tool: Tool{ID: "opencode", Name: "OpenCode", ListModels: func(_ string) ([]string, error) { return nil, nil }}, Binary: "opencode"},
	}

	// Should not be called since model query fails, skipping model prompt.
	errSelector := func(_ string, _ []string, _ string) (string, error) {
		return "", fmt.Errorf("should not be called")
	}

	tool, cfg, err := ChooseAgentTool(installed, configpkg.AgentSelectionConfig{DefaultAgentTool: "opencode"}, "", false, errSelector, "")
	if err != nil {
		t.Fatalf("ChooseAgentTool() error = %v", err)
	}
	if tool.Tool.ID != "opencode" {
		t.Fatalf("tool ID = %q, want %q", tool.Tool.ID, "opencode")
	}
	if cfg.DefaultModel != "" {
		t.Fatalf("DefaultModel = %q, want empty string", cfg.DefaultModel)
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
