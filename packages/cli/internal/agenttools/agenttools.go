package agenttools

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type Tool struct {
	ID          string
	Name        string
	Binaries    []string
	Description string
	Args        func(prompt string) []string
	PlanArgs    func(prompt string) []string
}

type InstalledTool struct {
	Tool   Tool
	Binary string
}

var supportedTools = []Tool{
	{
		ID:          "claude",
		Name:        "Claude Code",
		Binaries:    []string{"claude"},
		Description: "Anthropic Claude Code CLI",
		Args: func(prompt string) []string {
			return []string{"-p", prompt}
		},
		PlanArgs: func(prompt string) []string {
			return []string{"--permission-mode", "plan", prompt}
		},
	},
	{
		ID:          "codex",
		Name:        "Codex",
		Binaries:    []string{"codex"},
		Description: "OpenAI Codex CLI",
		Args: func(prompt string) []string {
			return []string{"exec", prompt}
		},
		PlanArgs: nil,
	},
	{
		ID:          "opencode",
		Name:        "OpenCode",
		Binaries:    []string{"opencode"},
		Description: "OpenCode CLI",
		Args: func(prompt string) []string {
			return []string{"run", prompt}
		},
		PlanArgs: nil,
	},
	{
		ID:          "cursor",
		Name:        "Cursor",
		Binaries:    []string{"agent"},
		Description: "Cursor agent CLI",
		Args: func(prompt string) []string {
			return []string{"-p", prompt}
		},
		PlanArgs: nil,
	},
	{
		ID:          "antigravity",
		Name:        "Antigravity",
		Binaries:    []string{"antigravity-cli", "agcl"},
		Description: "Antigravity CLI",
		Args: func(prompt string) []string {
			return []string{prompt}
		},
		PlanArgs: nil,
	},
}

var lookPath = exec.LookPath

func Supported() []Tool {
	tools := make([]Tool, len(supportedTools))
	copy(tools, supportedTools)
	return tools
}

func DetectInstalled() ([]InstalledTool, error) {
	installed := make([]InstalledTool, 0, len(supportedTools))
	for _, tool := range supportedTools {
		binary, ok := detectToolBinary(tool)
		if !ok {
			continue
		}
		installed = append(installed, InstalledTool{Tool: tool, Binary: binary})
	}

	sort.Slice(installed, func(i, j int) bool {
		return installed[i].Tool.Name < installed[j].Tool.Name
	})

	return installed, nil
}

func FindInstalled(toolID string, installed []InstalledTool) (InstalledTool, bool) {
	for _, tool := range installed {
		if tool.Tool.ID == toolID {
			return tool, true
		}
	}
	return InstalledTool{}, false
}

func FindSupported(toolID string) (Tool, bool) {
	for _, tool := range supportedTools {
		if tool.ID == toolID {
			return tool, true
		}
	}
	return Tool{}, false
}

func Labels(installed []InstalledTool) []string {
	labels := make([]string, 0, len(installed))
	for _, tool := range installed {
		labels = append(labels, Label(tool))
	}
	return labels
}

func Label(tool InstalledTool) string {
	return fmt.Sprintf("%s (%s)", tool.Tool.Name, tool.Binary)
}

func InstalledIDs(installed []InstalledTool) []string {
	ids := make([]string, 0, len(installed))
	for _, tool := range installed {
		ids = append(ids, tool.Tool.ID)
	}
	return ids
}

func FormatInstalledNames(installed []InstalledTool) string {
	parts := make([]string, 0, len(installed))
	for _, tool := range installed {
		parts = append(parts, fmt.Sprintf("%s (%s)", tool.Tool.Name, tool.Binary))
	}
	return strings.Join(parts, ", ")
}

func detectToolBinary(tool Tool) (string, bool) {
	for _, binary := range tool.Binaries {
		if _, err := lookPath(binary); err == nil {
			return binary, true
		}
	}
	return "", false
}

func SupportsInteractivePlan(tool InstalledTool) bool {
	return tool.Tool.PlanArgs != nil
}
