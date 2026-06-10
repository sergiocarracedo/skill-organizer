package agenttools

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/pterm/pterm"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
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

// ToolSelector is a function that prompts the user to select one option from a list.
type ToolSelector func(prompt string, labels []string, defaultOption string) (string, error)

// ChooseAgentToolFunc is a swappable function variable for ChooseAgentTool.
var ChooseAgentToolFunc = chooseAgentToolImpl

// SelectInstalledToolFunc is a swappable function variable for SelectInstalledTool.
var SelectInstalledToolFunc = selectInstalledToolImpl

// ChooseAgentTool selects an agent tool from the installed list, using explicit ID,
// saved default, or interactive prompt. Returns the selected tool and updated config.
func ChooseAgentTool(installed []InstalledTool, cfg configpkg.AgentSelectionConfig, explicitID string, choose bool, selector ToolSelector) (InstalledTool, configpkg.AgentSelectionConfig, error) {
	return ChooseAgentToolFunc(installed, cfg, explicitID, choose, selector)
}

// SelectInstalledTool prompts the user to select one of the installed tools.
func SelectInstalledTool(installed []InstalledTool, selector ToolSelector) (InstalledTool, error) {
	return SelectInstalledToolFunc(installed, selector)
}

func chooseAgentToolImpl(installed []InstalledTool, cfg configpkg.AgentSelectionConfig, explicitID string, choose bool, selector ToolSelector) (InstalledTool, configpkg.AgentSelectionConfig, error) {
	if explicitID != "" {
		tool, ok := FindInstalled(explicitID, installed)
		if !ok {
			return InstalledTool{}, cfg, fmt.Errorf("requested tool %q is not installed. Installed tools: %s", explicitID, FormatInstalledNames(installed))
		}
		cfg.DefaultAgentTool = tool.Tool.ID
		return tool, cfg, nil
	}

	if !choose && cfg.DefaultAgentTool != "" {
		if tool, ok := FindInstalled(cfg.DefaultAgentTool, installed); ok {
			return tool, cfg, nil
		}
	}

	selection, err := selectInstalledToolImpl(installed, selector)
	if err != nil {
		return InstalledTool{}, cfg, err
	}

	cfg.DefaultAgentTool = selection.Tool.ID
	return selection, cfg, nil
}

func selectInstalledToolImpl(installed []InstalledTool, selector ToolSelector) (InstalledTool, error) {
	ordered := make([]InstalledTool, len(installed))
	copy(ordered, installed)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Tool.Name < ordered[j].Tool.Name
	})

	labels := make([]string, 0, len(ordered))
	byLabel := make(map[string]InstalledTool, len(ordered))
	for _, tool := range ordered {
		label := Label(tool)
		labels = append(labels, label)
		byLabel[label] = tool
	}

	selection, err := selector("Select the agent tool", labels, labels[0])
	if err != nil {
		return InstalledTool{}, err
	}

	tool, ok := byLabel[selection]
	if !ok {
		return InstalledTool{}, fmt.Errorf("unknown selected tool %q", selection)
	}

	return tool, nil
}

// SpinnerHandle abstracts the pterm spinner for testing.
type SpinnerHandle interface {
	UpdateText(text string)
	Success(text ...any)
	Fail(text ...any)
}

// StartSpinnerFunc is a swappable function variable for StartSpinner.
var StartSpinnerFunc func(text string) (SpinnerHandle, error)

// HideCursorFunc is a swappable function variable for HideCursor.
var HideCursorFunc func()

// ShowCursorFunc is a swappable function variable for ShowCursor.
var ShowCursorFunc func()

// LaunchSessionFunc is a swappable function variable for LaunchSession.
var LaunchSessionFunc func(tool InstalledTool, prompt string) error

func init() {
	StartSpinnerFunc = defaultStartSpinner
	HideCursorFunc = func() {
		_, _ = fmt.Fprint(os.Stdout, "\033[?25l")
	}
	ShowCursorFunc = func() {
		_, _ = fmt.Fprint(os.Stdout, "\033[?25h")
	}
	LaunchSessionFunc = defaultLaunchSession
}

// StartSpinner starts a pterm spinner with the given text.
func StartSpinner(text string) (SpinnerHandle, error) {
	return StartSpinnerFunc(text)
}

// HideCursor hides the terminal cursor.
func HideCursor() {
	HideCursorFunc()
}

// ShowCursor shows the terminal cursor.
func ShowCursor() {
	ShowCursorFunc()
}

// LaunchSession launches the agent tool in plan/interactive mode with the given prompt.
func LaunchSession(tool InstalledTool, prompt string) error {
	return LaunchSessionFunc(tool, prompt)
}

func defaultStartSpinner(text string) (SpinnerHandle, error) {
	HideCursor()
	spinner, err := pterm.DefaultSpinner.Start(text)
	if err != nil {
		ShowCursor()
		return nil, err
	}
	return spinner, nil
}

func defaultLaunchSession(tool InstalledTool, prompt string) error {
	if tool.Tool.PlanArgs == nil {
		return fmt.Errorf("%s cannot be opened in plan mode from this CLI yet", tool.Tool.Name)
	}
	command := exec.Command(tool.Binary, tool.Tool.PlanArgs(prompt)...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
