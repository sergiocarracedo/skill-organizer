package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/overlap"
)

var (
	overlapChooseTool   bool
	overlapToolID       string
	overlapAllSkills    bool
	overlapPrintPrompt  bool
	overlapMinType      string
	overlapNoAskToApply bool
)

type spinnerHandle interface {
	UpdateText(text string)
	Success(text ...any)
	Fail(text ...any)
}

var (
	loadOverlapConfigFunc    = configpkg.LoadOverlapConfigOrDefault
	saveOverlapConfigFunc    = configpkg.SaveOverlapConfig
	loadResolvedLocationFunc = loadResolvedLocation
	detectInstalledTools     = agenttools.DetectInstalled
	confirmExternalCosts     = confirm
	confirmApplyPlan         = confirm
	selectToolOption         = selectOption
	collectOverlapSkills     = overlap.CollectSkills
	printOverlapPromptFunc   = func(prompt string) {
		pterm.Println(prompt)
	}
	saveApplyPlanPrompt = writeApplyPlanPrompt
	startOverlapSpinner = startDefaultSpinner
	runOverlapAnalysis  = overlap.Run
	launchPlanSession   = func(tool agenttools.InstalledTool, prompt string) error {
		if tool.Tool.PlanArgs == nil {
			return fmt.Errorf("%s cannot be opened in plan mode from this CLI yet", tool.Tool.Name)
		}
		command := exec.Command(tool.Binary, tool.Tool.PlanArgs(prompt)...)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}
	printInfoMessage = func(format string, args ...any) {
		pterm.Info.Printfln(format, args...)
	}
	printDebugMessage = func(format string, args ...any) {
		pterm.Debug.Printfln(format, args...)
	}
	printWarningMessage = func(format string, args ...any) {
		pterm.Warning.Printfln(format, args...)
	}
	hideCursor = func() {
		_, _ = fmt.Fprint(os.Stdout, "\033[?25l")
	}
	showCursor = func() {
		_, _ = fmt.Fprint(os.Stdout, "\033[?25h")
	}
)

func newCheckOverlapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-overlap",
		Short: "Evaluate skills for overlap using an installed agent tool",
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			minOverlapRank, minOverlapLabel, err := parseMinOverlapType(overlapMinType)
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

			printInfoMessage("Using tool: %s (%s)", tool.Tool.Name, tool.Binary)
			printInfoMessage("Reconfigure later with: skill-organizer skill check-overlap --choose-tool")
			printInfoMessage("Showing overlap types: %s and stronger", minOverlapLabel)

			spinner, err := startOverlapSpinner("Analyzing skills")
			if err != nil {
				return err
			}
			defer showCursor()

			report, err := runOverlapAnalysis(cmd.Context(), tool, prompt, func(status string) {
				spinner.UpdateText(limitSpinnerText("Analyzing skills: "+status, 80))
			})
			if err != nil {
				spinner.Fail("Overlap analysis failed")
				return err
			}
			spinner.Success("Overlap analysis completed")
			report.Groups = filterOverlapGroups(report.Groups, minOverlapRank)

			printOverlapReport(tool, len(items), overlapAllSkills, report)

			if overlapNoAskToApply {
				return nil
			}

			planPrompt := overlap.BuildApplyPlanPrompt(report)

			pterm.Println()

			if !agenttools.SupportsInteractivePlan(tool) {

				printDebugMessage("%s has no verified interactive plan mode.", tool.Tool.Name)
				askToSave, err := confirmApplyPlan("Generate a prompt to apply the recommendations?", false)
				if err != nil {
					return err
				}
				if !askToSave {
					return nil
				}

				path, err := saveApplyPlanPrompt(planPrompt)
				if err != nil {
					return err
				}
				printInfoMessage("Saved apply-plan prompt: %s", path)
				return nil
			}

			askToApply, err := confirmApplyPlan("Ask the agent to prepare an apply plan?", false)
			if err != nil {
				return err
			}
			if !askToApply {
				return nil
			}

			warnApply, err := confirmApplyPlan("Agent opens in plan mode. Back up or commit first. Continue?", false)
			if err != nil {
				return err
			}
			if !warnApply {
				return nil
			}

			printInfoMessage("Opening %s in plan mode", tool.Tool.Name)
			return launchPlanSession(tool, planPrompt)
		},
	}

	cmd.Flags().BoolVar(&overlapChooseTool, "choose-tool", false, "Prompt to choose the agent tool again")
	cmd.Flags().StringVar(&overlapToolID, "tool", "", "Use a specific installed tool id (claude, codex, opencode, cursor, antigravity)")
	cmd.Flags().BoolVar(&overlapAllSkills, "include-disabled", false, "Include disabled skills in the overlap analysis")
	cmd.Flags().BoolVar(&overlapPrintPrompt, "print-prompt", false, "Print the generated overlap prompt without invoking an external tool")
	cmd.Flags().StringVar(&overlapMinType, "min-overlap-type", "partial", "Minimum overlap type to show: adjacent|partial|duplicate or 1|2|3")
	cmd.Flags().BoolVar(&overlapNoAskToApply, "no-ask-to-apply", false, "Do not ask the selected agent to prepare an apply plan after the report")

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

func printOverlapReport(tool agenttools.InstalledTool, skillCount int, includeDisabled bool, report overlap.Report) {
	pterm.Println(cliLogo())
	pterm.Println(cliHeader())
	pterm.Println(fmt.Sprintf("commit %s, built %s", commit, date))
	pterm.Println()

	pterm.DefaultSection.Println("Overlap Analysis")
	pterm.Println(styleLabel("Tool") + ": " + pterm.NewStyle(pterm.FgCyan, pterm.Bold).Sprint(tool.Tool.Name) + " (" + pterm.NewStyle(pterm.FgLightMagenta).Sprint(tool.Binary) + ")")
	pterm.Println(styleLabel("Analyzed skills") + ": " + pterm.NewStyle(pterm.FgLightWhite, pterm.Bold).Sprint(skillCount))
	if includeDisabled {
		pterm.Println(styleLabel("Included disabled skills") + ": yes")
	} else {
		pterm.Println(styleLabel("Included disabled skills") + ": no")
	}

	pterm.DefaultSection.Println("Summary")
	printWrappedStyled(report.Summary, 80, pterm.NewStyle(pterm.FgLightWhite))

	pterm.DefaultSection.Println("Potential Overlap Groups")
	if len(report.Groups) == 0 {
		pterm.Println(pterm.NewStyle(pterm.FgGreen).Sprint("No notable overlap groups reported."))
	} else {
		for index, group := range report.Groups {
			if index > 0 {
				pterm.Println()
			}
			printOverlapGroup(index+1, group)
		}
	}

	pterm.DefaultSection.Println("Recommendations")
	if len(report.Recommendations) == 0 {
		pterm.Println("None")
		return
	}
	for _, recommendation := range report.Recommendations {
		printWrappedBullet(recommendation, 80)
	}
}

func startDefaultSpinner(text string) (spinnerHandle, error) {
	hideCursor()
	spinner, err := pterm.DefaultSpinner.Start(text)
	if err != nil {
		showCursor()
		return nil, err
	}
	return spinner, nil
}

func limitSpinnerText(value string, width int) string {
	value = strings.TrimSpace(value)
	if visibleRuneWidth(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func printOverlapGroup(index int, group overlap.Group) {
	header := pterm.NewStyle(pterm.FgLightCyan, pterm.Bold).Sprint(fmt.Sprintf("Group %d", index))
	content := formatBoxContent(group)
	printer := pterm.DefaultBox.
		WithTitle(header).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgDarkGray)).
		WithLeftPadding(1).
		WithRightPadding(1)
	printer.Println(content)
}

func styleLabel(label string) string {
	return pterm.NewStyle(pterm.FgMagenta, pterm.Bold).Sprint(label)
}

func styleSkillNames(names []string) []string {
	styled := make([]string, 0, len(names))
	for _, name := range names {
		styled = append(styled, pterm.NewStyle(pterm.FgLightCyan, pterm.Bold).Sprint(name))
	}
	return styled
}

func overlapScoreStyle(score int) *pterm.Style {
	switch {
	case score >= 80:
		return pterm.NewStyle(pterm.FgRed, pterm.Bold)
	case score >= 50:
		return pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	default:
		return pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	}
}

func overlapTypeStyle(value string) *pterm.Style {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "duplicate":
		return pterm.NewStyle(pterm.FgRed, pterm.Bold)
	case "partial":
		return pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	default:
		return pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	}
}

func parseMinOverlapType(value string) (int, string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "", "2", "partial":
		return 2, "partial", nil
	case "1", "adjacent":
		return 1, "adjacent", nil
	case "3", "duplicate":
		return 3, "duplicate", nil
	default:
		if _, err := strconv.Atoi(trimmed); err == nil {
			return 0, "", fmt.Errorf("invalid min overlap type %q: use 1, 2, 3 or adjacent, partial, duplicate", value)
		}
		return 0, "", fmt.Errorf("invalid min overlap type %q: use 1, 2, 3 or adjacent, partial, duplicate", value)
	}
}

func overlapTypeRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "duplicate":
		return 3
	case "partial":
		return 2
	default:
		return 1
	}
}

func filterOverlapGroups(groups []overlap.Group, minRank int) []overlap.Group {
	filtered := make([]overlap.Group, 0, len(groups))
	for _, group := range groups {
		if overlapTypeRank(group.OverlapType) < minRank {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

func printWrappedStyled(text string, width int, style *pterm.Style) {
	for _, line := range wrapText(text, width) {
		pterm.Println(style.Sprint(line))
	}
}

func printWrappedBullet(text string, width int) {
	lines := wrapText(text, width-2)
	for index, line := range lines {
		prefix := "  "
		if index == 0 {
			prefix = "- "
		}
		pterm.Println(prefix + pterm.NewStyle(pterm.FgLightWhite).Sprint(line))
	}
}

func formatBoxContent(group overlap.Group) string {
	var builder strings.Builder
	builder.WriteString(styleLabel("Skills") + ":\n")
	builder.WriteString(formatSkillList(groupDisplayPaths(group)))
	builder.WriteString("\n")
	builder.WriteString(styleLabel("Overlap") + ": " + formatOverlapSummary(group) + "\n")
	builder.WriteString(styleLabel("Why the overlap") + ":\n")
	builder.WriteString(joinWrappedLines(group.WhyOverlap, 76, pterm.NewStyle(pterm.FgLightWhite)))
	builder.WriteString("\n")
	builder.WriteString(styleLabel("Recommendation") + ":\n")
	builder.WriteString(joinWrappedLines(group.Recommendation, 76, pterm.NewStyle(pterm.FgLightWhite)))
	return builder.String()
}

func formatSkillList(names []string) string {
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, "- "+pterm.NewStyle(pterm.FgLightCyan, pterm.Bold).Sprint(name))
	}
	return strings.Join(lines, "\n")
}

func groupDisplayPaths(group overlap.Group) []string {
	if len(group.SkillPaths) > 0 {
		return group.SkillPaths
	}
	return group.SkillNames
}

func formatOverlapSummary(group overlap.Group) string {
	marker := overlapScoreStyle(group.Score).Sprint("■")
	typeLabel := overlapTypeStyle(group.OverlapType).Sprint(strings.Title(group.OverlapType))
	score := overlapScoreStyle(group.Score).Sprint(fmt.Sprintf("(%d/100)", group.Score))
	return marker + " " + typeLabel + " " + score
}

func writeApplyPlanPrompt(prompt string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	plansDir := filepath.Join(wd, "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return "", fmt.Errorf("create plans directory: %w", err)
	}
	now := time.Now()
	fileName := fmt.Sprintf("skill-overlap-fix-%s-%s.md", now.Format("20060201"), now.Format("150405"))
	absolutePath := filepath.Join(plansDir, fileName)
	if err := os.WriteFile(absolutePath, []byte(prompt), 0o644); err != nil {
		return "", fmt.Errorf("write apply-plan prompt: %w", err)
	}
	return absolutePath, nil
}

func joinWrappedLines(text string, width int, style *pterm.Style) string {
	lines := wrapText(text, width)
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, style.Sprint(line))
	}
	return strings.Join(styled, "\n")
}

func wrapText(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{""}
	}
	paragraphs := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
				continue
			}
			candidate := current + " " + word
			if visibleRuneWidth(candidate) > width {
				lines = append(lines, current)
				current = word
				continue
			}
			current = candidate
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}
