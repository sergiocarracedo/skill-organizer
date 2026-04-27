package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/mover"
	serviceinternal "github.com/sergiocarracedo/skill-organizer/cli/internal/service"
	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type onboardTool struct {
	Name         string
	Target       string
	SourcePrompt string
	TargetPrompt string
}

func newOnboardCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "onboard",
		Short: "Guide first-time setup for a global skills project",
		RunE: func(_ *cobra.Command, _ []string) error {
			pterm.Println(cliLogo())
			pterm.Println(cliHeader())
			pterm.DefaultSection.Println("Welcome")
			pterm.Println("skill-organizer keeps your tool's flat skills folder in sync with a source folder that you actually edit.")
			pterm.Println("Manage skills in the source folder, usually a sibling like skills-organized, and let sync/watch update the target folder for the selected tool.")
			pterm.Println()

			tool, err := chooseOnboardTool()
			if err != nil {
				return err
			}

			target, err := configpkg.ResolvePath(tool.Target)
			if err != nil {
				return fmt.Errorf("resolve target path: %w", err)
			}

			source, err := promptPath(tool.SourcePrompt, configpkg.DefaultSourceForTarget(target))
			if err != nil {
				return err
			}

			if err := os.MkdirAll(source, 0o755); err != nil {
				return fmt.Errorf("create source directory: %w", err)
			}

			project := configpkg.Location{Source: source, Target: target}
			if err := project.Validate(); err != nil {
				return err
			}

			configFile, err := saveOnboardProject(target, project)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Configured %s project at: %s", tool.Name, configFile)
			pterm.Info.Printfln("Source: %s", source)
			pterm.Info.Printfln("Target: %s", target)

			if err := moveOnboardUnmanagedSkills(project); err != nil {
				return err
			}

			result, err := syncpkg.Run(project)
			if err != nil {
				return err
			}
			printSyncResult(configFile, result)

			installService, err := confirm("Install and start the watch service?", true)
			if err != nil {
				return err
			}

			if installService {
				if err := addWatchedConfig(configFile); err != nil {
					return err
				}
				pterm.Success.Printfln("Registered watched config: %s", configFile)

				if err := runServiceAction("install"); err != nil {
					pterm.Warning.Printfln("Service install failed: %v", err)
				} else {
					if err := runServiceAction("start"); err != nil {
						pterm.Warning.Printfln("Service start failed: %v", err)
					}
				}
			}

			report, err := statuspkg.Build(project)
			if err != nil {
				return err
			}

			return printStatusReport(configFile, project, report)
		},
	}
}

func onboardTools() []onboardTool {
	return []onboardTool{
		{
			Name:         "Generic (.agents)",
			Target:       "~/.agents/skills",
			SourcePrompt: "Select the source skills-organized folder for your .agents setup",
			TargetPrompt: "Generic (.agents: OpenCode, OpenAI Codex CLI, aider, goose) -> ~/.agents/skills",
		},
		{
			Name:         "Claude Code",
			Target:       "~/.claude/skills",
			SourcePrompt: "Select the source skills-organized folder for Claude Code",
			TargetPrompt: "Claude Code -> ~/.claude/skills",
		},
		{
			Name:         "Codex",
			Target:       "~/.codex/skills",
			SourcePrompt: "Select the source skills-organized folder for Codex",
			TargetPrompt: "Codex -> ~/.codex/skills",
		},
		{
			Name:         "Antigravity",
			Target:       "~/.agent/skills",
			SourcePrompt: "Select the source skills-organized folder for Antigravity",
			TargetPrompt: "Antigravity -> ~/.agent/skills",
		},
	}
}

func chooseOnboardTool() (onboardTool, error) {
	tools := onboardTools()
	options := make([]string, 0, len(tools))
	index := make(map[string]onboardTool, len(tools))
	for _, tool := range tools {
		options = append(options, tool.TargetPrompt)
		index[tool.TargetPrompt] = tool
	}

	selection, err := selectOption("Select the tool to onboard", options, options[0])
	if err != nil {
		return onboardTool{}, err
	}

	return index[selection], nil
}

func saveOnboardProject(target string, project configpkg.Location) (string, error) {
	configFile := configpkg.ConfigPathForTarget(target)
	if _, err := os.Stat(configFile); err == nil {
		message := fmt.Sprintf("A project config already exists at %s. Overwrite it?", configFile)
		overwrite, err := confirm(message, false)
		if err != nil {
			return "", err
		}
		if !overwrite {
			return "", fmt.Errorf("aborted")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check existing config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create target parent directory: %w", err)
	}
	if err := configpkg.SaveLocation(configFile, project); err != nil {
		return "", err
	}

	return configFile, nil
}

func moveOnboardUnmanagedSkills(project configpkg.Location) error {
	moves, err := mover.Plan(project)
	if err != nil {
		return err
	}
	if len(moves) == 0 {
		return nil
	}

	moveExisting, err := confirm("Move existing target skills into the source skills-organized folder now?", true)
	if err != nil {
		return err
	}
	if !moveExisting {
		return nil
	}

	defaultSelected := make([]string, 0, len(moves))
	for _, move := range moves {
		defaultSelected = append(defaultSelected, move.Name)
	}

	selectedMoves, err := chooseUnmanagedMovesWithDefaults(moves, defaultSelected)
	if err != nil {
		return err
	}
	if len(selectedMoves) == 0 {
		pterm.Info.Println("No unmanaged target entries selected")
		return nil
	}

	if err := mover.Apply(selectedMoves); err != nil {
		return err
	}

	pterm.Success.Printfln("Moved %d unmanaged target entries", len(selectedMoves))
	return nil
}

func addWatchedConfig(configFile string) error {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return err
	}

	registry, err := configpkg.LoadRegistryOrEmpty(registryPath)
	if err != nil {
		return err
	}

	registry.Add(configFile)
	return configpkg.SaveRegistry(registryPath, registry)
}

func runServiceAction(action string) error {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return err
	}

	status, err := serviceinternal.Control(registryPath, action)
	if err != nil {
		return err
	}

	pterm.Info.Printfln("Service %s: %s", action, status)
	if action == "stop" || action == "restart" || action == "uninstall" {
		serviceinternal.WaitForStopDelay()
	}
	return nil
}
