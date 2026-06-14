package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	securitypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/security"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

const highRiskThreshold = 70

var (
	securityPrintPrompt bool
	securityToolID      string
	securitySelectTooling  bool
	securityAllSkills   bool
	securityModelID     string
	securityForceRecheck bool
	securitySourceDir   string
)

var (
	securityLoadResolvedLocation = loadResolvedLocation
	securityDetectInstalledTools = agenttools.DetectInstalled
	securityLoadConfigFunc       = configpkg.LoadAgentSelectionConfigOrDefault
	securitySaveConfigFunc       = configpkg.SaveAgentSelectionConfig
	securityCollectSkills        = securitypkg.CollectSkills
	securityBuildPrompt          = securitypkg.BuildPrompt
	securityRunAnalysis          = defaultSecurityRunAnalysis
	securityConfirm              = confirm
	securityUpdateMetadata       = skills.UpdateManagedMetadata
	securityWriteDisabled        = skills.RewriteManagedFields
	securityPrintPromptFunc      = func(prompt string) { pterm.Println(prompt) }
	securityPrintInfo            = func(format string, args ...any) { pterm.Info.Printfln(format, args...) }
	securityPrintSuccess         = func(format string, args ...any) { pterm.Success.Printfln(format, args...) }
	securityPrintWarning         = func(format string, args ...any) { pterm.Warning.Printfln(format, args...) }
)

// defaultSecurityRunAnalysis wraps securitypkg.Run with model selection.
// If a model is specified and the tool supports it (ModelArgs != nil),
// ModelArgs is used instead of Args to build the CLI arguments.
func defaultSecurityRunAnalysis(ctx context.Context, tool agenttools.InstalledTool, prompt string, model string, onStatus func(string)) (securitypkg.SecurityReport, error) {
	if model != "" && tool.Tool.ModelArgs != nil {
		m := model
		t := tool
		t.Tool.Args = func(p string) []string {
			return tool.Tool.ModelArgs(m, p)
		}
		return securitypkg.Run(ctx, t, prompt, onStatus)
	}
	return securitypkg.Run(ctx, tool, prompt, onStatus)
}

func newCheckSecurityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-security",
		Short: "Evaluate skills for security risks using an installed agent tool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, location, err := securityLoadResolvedLocation()
			if err != nil {
				return err
			}

			if securitySourceDir != "" {
				abs, err := filepath.Abs(securitySourceDir)
				if err != nil {
					return fmt.Errorf("resolve --source path: %w", err)
				}
				location.Source = abs
			}

			items, err := securityCollectSkills(location, securityAllSkills)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				if securityAllSkills {
					return fmt.Errorf("no skills found in %s", location.Source)
				}
				return fmt.Errorf("no enabled skills found in %s", location.Source)
			}

			prompt := securityBuildPrompt(items)
			if securityPrintPrompt {
				securityPrintPromptFunc(prompt)
				return nil
			}

			installed, err := securityDetectInstalledTools()
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				securityPrintPromptFunc(prompt)
				return nil
			}

			registryPath, err := configpkg.RegistryPath()
			if err != nil {
				return err
			}

			agentCfg, err := securityLoadConfigFunc(registryPath)
			if err != nil {
				return err
			}

			tool, agentCfg, err := agenttools.ChooseAgentTool(installed, agentCfg, securityToolID, securitySelectTooling, selectOption, securityModelID)
			if err != nil {
				return err
			}

			if !agentCfg.AcknowledgedExternalToolCosts {
				accepted, err := securityConfirm("This command runs an installed external agent CLI to analyze your skills. Depending on the selected tool and account, usage may incur charges or metered costs. Continue?", false)
				if err != nil {
					return err
				}
				if !accepted {
					return fmt.Errorf("aborted")
				}
				agentCfg.AcknowledgedExternalToolCosts = true
			}

			if err := securitySaveConfigFunc(registryPath, agentCfg); err != nil {
				return err
			}

			securityPrintInfo("Using tool: %s (%s) model: %s", tool.Tool.Name, tool.Binary, agentCfg.DefaultModel)
			securityPrintInfo("Reconfigure later with: skill-organizer skill check-security --select-ai-tooling")

			spinner, err := agenttools.StartSpinner("")
			if err != nil {
				return err
			}
			defer agenttools.ShowCursor()

			type securityResult struct {
				name     string
				result   securitypkg.SkillResult
				skill    skills.Skill
				cached   bool
			}

			hashCache, err := securitypkg.LoadHashCache(location.Source)
			if err != nil {
				securityPrintWarning("Failed to load hash cache: %v. Starting fresh.", err)
				hashCache = make(securitypkg.HashCache)
			}
			if securityForceRecheck {
				securityPrintInfo("Force flag set: re-analyzing all skills regardless of cache")
			}

			total := len(items)
			var results []securityResult
			safeCount, warningCount, dangerousCount, skippedCount := 0, 0, 0, 0

			fmt.Println()
			progressTmpl := pterm.Green("Safe: %d")+pterm.Yellow("  |  ")+pterm.Magenta("Warning: %d")+pterm.Yellow("  |  ")+pterm.Red("Dangerous: %d")+pterm.Yellow("  |  Skipped: %d")+"\n"
			fmt.Printf(progressTmpl, 0, 0, 0, 0)
			fmt.Println()

			for i, item := range items {
				concrete := toSkill(item, location)

				currentHash, hashErr := skills.ComputeSkillHash(concrete.Dir)
				if hashErr != nil {
					securityPrintWarning("Failed to compute hash for %q: %v", item.Name, hashErr)
				}

				var r securitypkg.SkillResult
				cached := false

				if !securityForceRecheck && hashErr == nil {
					if cachedResult, found := securitypkg.GetCachedResult(hashCache, item.FlattenedName); found {
						if cachedResult.Hash == currentHash && cachedResult.Model == agentCfg.DefaultModel {
							r = securitypkg.SkillResult{
								Name:       item.FlattenedName,
								RiskScore:  cachedResult.RiskScore,
								RiskReason: cachedResult.RiskReason,
							}
							cached = true
						}
					}
				}

				if !cached {
					spinner.UpdateText(limitSpinnerText(
						fmt.Sprintf("[%d/%d] Analyzing %q...", i+1, total, item.Name), 80))

					skillPrompt := securityBuildPrompt([]securitypkg.SkillInfo{item})

					report, analysisErr := securityRunAnalysis(
						cmd.Context(), tool, skillPrompt, agentCfg.DefaultModel,
						func(status string) {
							spinner.UpdateText(limitSpinnerText(
								fmt.Sprintf("[%d/%d] Analyzing %q: %s", i+1, total, item.Name, status), 80))
						})
					if analysisErr != nil {
						securityPrintWarning("Analysis failed for %q: %v", item.Name, analysisErr)
						continue
					}

					for _, res := range report.Results {
						if res.Name == item.FlattenedName {
							r = res
							break
						}
					}
				}

				if r.Name == "" {
					r.Name = item.FlattenedName
					r.RiskScore = 0
					r.RiskReason = "No result returned"
				}

				results = append(results, securityResult{
					name:   item.Name,
					result: r,
					skill:  concrete,
					cached: cached,
				})

				if cached {
					skippedCount++
				}

				switch {
				case r.RiskScore >= highRiskThreshold:
					dangerousCount++
				case r.RiskScore >= 30:
					warningCount++
				default:
					safeCount++
				}

				var scoreStyled string
				switch {
				case r.RiskScore >= highRiskThreshold:
					scoreStyled = pterm.Red(fmt.Sprintf("%d", r.RiskScore))
				case r.RiskScore >= 30:
					scoreStyled = pterm.Magenta(fmt.Sprintf("%d", r.RiskScore))
				default:
					scoreStyled = pterm.Green(fmt.Sprintf("%d", r.RiskScore))
				}

				reason := r.RiskReason
				if len(reason) > 20 {
					reason = reason[:17] + "..."
				}
				batch := ""
				if cached {
					batch = " [cached]"
				}

				fmt.Printf("• %s - Score: %s │ %s%s\n", item.FlattenedName, scoreStyled, reason, batch)

				if !cached && hashErr == nil {
					securitypkg.SetCachedResult(hashCache, item.FlattenedName, securitypkg.CachedSkillResult{
						Hash:       currentHash,
						RiskScore:  r.RiskScore,
						RiskReason: r.RiskReason,
						Model:      agentCfg.DefaultModel,
						CheckedAt:  time.Now().UTC().Format(time.RFC3339),
					})
					if err := securitypkg.SaveHashCache(location.Source, hashCache); err != nil {
						securityPrintWarning("Failed to save hash cache: %v", err)
					}
				}
			}

			fmt.Println()
			spinner.Success(fmt.Sprintf(pterm.Green("Safe: %d")+pterm.Yellow("  |  ")+pterm.Magenta("WARNING: %d")+pterm.Yellow("  |  ")+pterm.Red("DANGER: %d")+pterm.Yellow("  |  Skipped: %d"),
				safeCount, warningCount, dangerousCount, skippedCount))

			now := time.Now().UTC().Format(time.RFC3339)
			highRiskCount := 0
			disabledCount := 0

			for _, sr := range results {
				updates := skills.ManagedMetadata{
					RiskScore:       sr.result.RiskScore,
					RiskEvaluatedAt: now,
					RiskEvaluator:   tool.Tool.ID,
					RiskReason:      sr.result.RiskReason,
				}

				hash, hashErr := skills.ComputeSkillHash(sr.skill.Dir)
				if hashErr == nil {
					updates.RiskSourceHash = hash
				} else {
					securityPrintWarning("Failed to compute content hash for %q: %v", sr.name, hashErr)
				}

				if err := securityUpdateMetadata(sr.skill, updates); err != nil {
					return fmt.Errorf("write risk score for %s: %w", sr.name, err)
				}

				if sr.result.RiskScore >= highRiskThreshold {
					highRiskCount++
					fmt.Printf(" - %s%s%s %s %s %s\n",
						pterm.White("["), pterm.Red("Danger"), pterm.White("]"),
						sr.name,
						pterm.White("Scored"),
						pterm.Red(fmt.Sprintf("%d/100", sr.result.RiskScore)))
					fmt.Printf("   %s\n\n", sr.result.RiskReason)
				} else if sr.result.RiskScore >= 30 {
					fmt.Printf(" - %s%s%s %s %s %s\n",
						pterm.White("["), pterm.Magenta("Warning"), pterm.White("]"),
						sr.name,
						pterm.White("Scored"),
						pterm.Magenta(fmt.Sprintf("%d/100", sr.result.RiskScore)))
					fmt.Printf("   %s\n\n", sr.result.RiskReason)
				}
			}

			for _, sr := range results {
				if sr.result.RiskScore < highRiskThreshold {
					continue
				}
				accept, err := securityConfirm(fmt.Sprintf("Disable skill %q due to high risk?", sr.name), true)
				if err != nil {
					return err
				}
				if accept {
					if err := securityWriteDisabled(sr.skill, false, true); err != nil {
						return fmt.Errorf("disable %s: %w", sr.name, err)
					}
					disabledCount++
				}
			}

			securityPrintSuccess("Checked %d skills, %d high-risk, %d disabled", total, highRiskCount, disabledCount)

			return nil
		},
	}

	cmd.Flags().BoolVar(&securityPrintPrompt, "print-prompt", false, "Print the generated security prompt without invoking an external tool")
	cmd.Flags().StringVar(&securityToolID, "tool", "", "Use a specific installed tool id (claude, codex, opencode, cursor, antigravity)")
	cmd.Flags().StringVar(&securityModelID, "model", "", "AI model to use (format: provider/model)")
	cmd.Flags().BoolVar(&securitySelectTooling, "select-ai-tooling", false, "Prompt to select AI tool and model again")
	cmd.Flags().BoolVar(&securityAllSkills, "include-disabled", false, "Include disabled skills in the analysis")
	cmd.Flags().BoolVarP(&securityForceRecheck, "force", "f", false, "Force re-analysis of all skills, ignoring cached results")
	cmd.Flags().StringVar(&securitySourceDir, "source", "", "Override the source directory for skill scanning (useful with test fixtures)")

	return cmd
}

func skillByFlattenedName(items []securitypkg.SkillInfo, name string) (securitypkg.SkillInfo, bool) {
	for _, item := range items {
		if item.FlattenedName == name {
			return item, true
		}
	}
	return securitypkg.SkillInfo{}, false
}

func toSkill(info securitypkg.SkillInfo, location configpkg.Location) skills.Skill {
	return skills.Skill{
		Dir:           filepath.Join(location.Source, info.RelativePath),
		SkillFile:     filepath.Join(location.Source, info.RelativePath, skills.SkillFileName),
		RelativePath:  info.RelativePath,
		FlattenedName: info.FlattenedName,
	}
}

// RunCheckSecurityForSkill performs a security analysis on a single skill
// using the default agent selection flow. Used by the skill-add hook.
// Hook mode: no cost-acknowledgment prompt; auto-pick first installed tool.
func RunCheckSecurityForSkill(skill skills.Skill, location configpkg.Location) error {
	info := securitypkg.SkillInfo{
		FlattenedName: skill.FlattenedName,
		RelativePath:  skill.RelativePath,
		Name:          skill.FlattenedName,
	}
	if doc, err := skills.LoadDocument(skill.SkillFile); err == nil {
		if name := doc.Name(); name != "" {
			info.Name = name
		}
		info.Description = doc.Description()
	}

	prompt := securitypkg.BuildPrompt([]securitypkg.SkillInfo{info})

	installed, err := securityDetectInstalledTools()
	if err != nil {
		return fmt.Errorf("detect tools: %w", err)
	}
	if len(installed) == 0 {
		pterm.Warning.Println("No agent tools detected. Run 'skill-organizer skill check-security' manually after installing a tool.")
		return nil
	}

	tool := installed[0]

	report, err := securityRunAnalysis(context.Background(), tool, prompt, "", func(_ string) {})
	if err != nil {
		return fmt.Errorf("security analysis failed: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, result := range report.Results {
		if result.Name != skill.FlattenedName {
			continue
		}
		updates := skills.ManagedMetadata{
			RiskScore:       result.RiskScore,
			RiskEvaluatedAt: now,
			RiskEvaluator:   tool.Tool.ID,
			RiskReason:      result.RiskReason,
		}

		hash, hashErr := skills.ComputeSkillHash(skill.Dir)
		if hashErr == nil {
			updates.RiskSourceHash = hash
		} else {
			pterm.Warning.Printfln("Failed to compute content hash for %q: %v", skill.FlattenedName, hashErr)
		}

		if err := skills.UpdateManagedMetadata(skill, updates); err != nil {
			return fmt.Errorf("persist risk score: %w", err)
		}
		break
	}

	return nil
}
