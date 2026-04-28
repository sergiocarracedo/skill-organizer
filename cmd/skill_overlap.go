package cmd

import (
	"fmt"
	"sort"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/overlap"
)

var (
	overlapChooseTool  bool
	overlapToolID      string
	overlapAllSkills   bool
	overlapPrintPrompt bool
)

var (
	loadOverlapConfigFunc    = configpkg.LoadOverlapConfigOrDefault
	saveOverlapConfigFunc    = configpkg.SaveOverlapConfig
	loadResolvedLocationFunc = loadResolvedLocation
	detectInstalledTools     = agenttools.DetectInstalled
	confirmExternalCosts     = confirm
	selectToolOption         = selectOption
	collectOverlapSkills     = overlap.CollectSkills
	printOverlapPromptFunc   = func(prompt string) {
		pterm.Println(prompt)
	}
	runOverlapAnalysis = overlap.Run
)

func newOverlapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overlap",
		Short: "Evaluate skills for overlap using an installed agent tool",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, location, err := loadResolvedLocationFunc()
			if err != nil {
				return err
			}

			items, err := collectOverlapSkills(location, overlapAllSkills)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				if overlapAllSkills {
					return fmt.Errorf("no skills found in %s", location.Source)
				}
				return fmt.Errorf("no enabled skills found in %s", location.Source)
			}

			prompt := overlap.BuildPrompt(items)
			if overlapPrintPrompt {
				printOverlapPromptFunc(prompt)
				return nil
			}

			installed, err := detectInstalledTools()
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				return fmt.Errorf("no supported agent tools were detected. Install one of: Claude Code, Codex, OpenCode, Cursor, or Antigravity")
			}

			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			overlapCfg, err := loadOverlapConfigFunc(registryPath)
			if err != nil {
				return err
			}

			tool, overlapCfg, err := chooseOverlapTool(installed, overlapCfg, overlapToolID, overlapChooseTool)
			if err != nil {
				return err
			}

			if !overlapCfg.AcknowledgedExternalToolCosts {
				accepted, err := confirmExternalCosts("This command runs an installed external agent CLI to analyze your skills. Depending on the selected tool and account, usage may incur charges or metered costs. Continue?", false)
				if err != nil {
					return err
				}
				if !accepted {
					return fmt.Errorf("aborted")
				}
				overlapCfg.AcknowledgedExternalToolCosts = true
			}

			if err := saveOverlapConfigFunc(registryPath, overlapCfg); err != nil {
				return err
			}

			report, err := runOverlapAnalysis(tool, prompt)
			if err != nil {
				return err
			}

			printOverlapReport(tool, len(items), overlapAllSkills, report)
			return nil
		},
	}

	cmd.Flags().BoolVar(&overlapChooseTool, "choose-tool", false, "Prompt to choose the agent tool again")
	cmd.Flags().StringVar(&overlapToolID, "tool", "", "Use a specific installed tool id (claude, codex, opencode, cursor, antigravity)")
	cmd.Flags().BoolVar(&overlapAllSkills, "include-disabled", false, "Include disabled skills in the overlap analysis")
	cmd.Flags().BoolVar(&overlapPrintPrompt, "print-prompt", false, "Print the generated overlap prompt without invoking an external tool")

	return cmd
}

func chooseOverlapTool(installed []agenttools.InstalledTool, cfg configpkg.OverlapConfig, explicitID string, choose bool) (agenttools.InstalledTool, configpkg.OverlapConfig, error) {
	if explicitID != "" {
		tool, ok := agenttools.FindInstalled(explicitID, installed)
		if !ok {
			return agenttools.InstalledTool{}, cfg, fmt.Errorf("requested tool %q is not installed. Installed tools: %s", explicitID, agenttools.FormatInstalledNames(installed))
		}
		cfg.DefaultAgentTool = tool.Tool.ID
		return tool, cfg, nil
	}

	if !choose && cfg.DefaultAgentTool != "" {
		if tool, ok := agenttools.FindInstalled(cfg.DefaultAgentTool, installed); ok {
			return tool, cfg, nil
		}
	}

	selection, err := selectInstalledTool(installed)
	if err != nil {
		return agenttools.InstalledTool{}, cfg, err
	}

	cfg.DefaultAgentTool = selection.Tool.ID
	return selection, cfg, nil
}

func selectInstalledTool(installed []agenttools.InstalledTool) (agenttools.InstalledTool, error) {
	ordered := make([]agenttools.InstalledTool, len(installed))
	copy(ordered, installed)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Tool.Name < ordered[j].Tool.Name
	})

	labels := make([]string, 0, len(ordered))
	byLabel := make(map[string]agenttools.InstalledTool, len(ordered))
	for _, tool := range ordered {
		label := agenttools.Label(tool)
		labels = append(labels, label)
		byLabel[label] = tool
	}

	selection, err := selectToolOption("Select the agent tool to evaluate overlap", labels, labels[0])
	if err != nil {
		return agenttools.InstalledTool{}, err
	}

	tool, ok := byLabel[selection]
	if !ok {
		return agenttools.InstalledTool{}, fmt.Errorf("unknown selected tool %q", selection)
	}

	return tool, nil
}

func printOverlapReport(tool agenttools.InstalledTool, skillCount int, includeDisabled bool, report string) {
	pterm.Println(cliLogo())
	pterm.Println(cliHeader())
	pterm.Println(fmt.Sprintf("commit %s, built %s", commit, date))
	pterm.Println()

	pterm.DefaultSection.Println("Overlap Analysis")
	pterm.Println("Tool: " + tool.Tool.Name + " (" + tool.Binary + ")")
	pterm.Println(fmt.Sprintf("Analyzed skills: %d", skillCount))
	if includeDisabled {
		pterm.Println("Included disabled skills: yes")
	} else {
		pterm.Println("Included disabled skills: no")
	}

	pterm.DefaultSection.Println("Report")
	pterm.Println(report)
}
