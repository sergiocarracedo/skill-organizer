package overlap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestBuildPromptIncludesDescriptionsAndFallback(t *testing.T) {
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

	if !strings.Contains(prompt, "## Potential Overlap Groups") {
		t.Fatalf("BuildPrompt() missing overlap section")
	}
	if !strings.Contains(prompt, "description: \"Finds issues in alpha.\\\\nExplains them.\"") {
		t.Fatalf("BuildPrompt() missing escaped multiline description: %q", prompt)
	}
	if !strings.Contains(prompt, "description: \"No description provided.\"") {
		t.Fatalf("BuildPrompt() missing fallback description: %q", prompt)
	}
}

func TestRunReturnsTrimmedOutput(t *testing.T) {
	original := commandRunner
	commandRunner = func(binary string, args []string) (string, error) {
		return "\n report body \n", nil
	}
	t.Cleanup(func() {
		commandRunner = original
	})

	result, err := Run(mockInstalledTool("claude", "claude"), "prompt")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result != "report body" {
		t.Fatalf("Run() = %q, want %q", result, "report body")
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
