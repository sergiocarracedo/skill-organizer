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
	securityChooseTool  bool
	securityAllSkills   bool
	securityModelID     string
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

			tool, agentCfg, err := agenttools.ChooseAgentTool(installed, agentCfg, securityToolID, securityChooseTool, selectOption, securityModelID)
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

			securityPrintInfo("Using tool: %s (%s)", tool.Tool.Name, tool.Binary)
			securityPrintInfo("Reconfigure later with: skill-organizer skill check-security --choose-tool")

			spinner, err := agenttools.StartSpinner("Analyzing skills for security risks")
			if err != nil {
				return err
			}
			defer agenttools.ShowCursor()

			report, err := securityRunAnalysis(cmd.Context(), tool, prompt, agentCfg.DefaultModel, func(status string) {
				spinner.UpdateText(limitSpinnerText("Analyzing skills: "+status, 80))
			})
			if err != nil {
				spinner.Fail("Security analysis failed")
				return err
			}
			spinner.Success("Security analysis completed")

			now := time.Now().UTC().Format(time.RFC3339)
			highRiskCount := 0
			disabledCount := 0
			missingCount := 0

			for _, result := range report.Results {
				skill, ok := skillByFlattenedName(items, result.Name)
				if !ok {
					missingCount++
					securityPrintWarning("Skill %q from agent report not found in scanned list", result.Name)
					continue
				}

			updates := skills.ManagedMetadata{
				RiskScore:       result.RiskScore,
				RiskEvaluatedAt: now,
				RiskEvaluator:   tool.Tool.ID,
				RiskReason:      result.RiskReason,
			}

			concrete := toSkill(skill, location)

			hash, hashErr := skills.ComputeSkillHash(concrete.Dir)
			if hashErr == nil {
				updates.RiskSourceHash = hash
			} else {
				securityPrintWarning("Failed to compute content hash for %q: %v", skill.Name, hashErr)
			}

			if err := securityUpdateMetadata(concrete, updates); err != nil {
				return fmt.Errorf("write risk score for %s: %w", skill.FlattenedName, err)
			}

				if result.RiskScore < highRiskThreshold {
					continue
				}
				highRiskCount++

				securityPrintWarning("Skill %q scored %d/100: %s", skill.Name, result.RiskScore, result.RiskReason)

				accept, err := securityConfirm(fmt.Sprintf("Disable skill %q due to high risk?", skill.Name), true)
				if err != nil {
					return err
				}
				if accept {
					if err := securityWriteDisabled(concrete, false, true); err != nil {
						return fmt.Errorf("disable %s: %w", skill.FlattenedName, err)
					}
					disabledCount++
				}
			}

			securityPrintSuccess("Checked %d skills, %d high-risk, %d disabled", len(items), highRiskCount, disabledCount)
			if missingCount > 0 {
				securityPrintWarning("%d skills reported by agent were not found in the scanned source", missingCount)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&securityPrintPrompt, "print-prompt", false, "Print the generated security prompt without invoking an external tool")
	cmd.Flags().StringVar(&securityToolID, "tool", "", "Use a specific installed tool id (claude, codex, opencode, cursor, antigravity)")
	cmd.Flags().StringVar(&securityModelID, "model", "", "AI model to use (format: provider/model)")
	cmd.Flags().BoolVar(&securityChooseTool, "choose-tool", false, "Prompt to choose the agent tool again")
	cmd.Flags().BoolVar(&securityAllSkills, "include-disabled", false, "Include disabled skills in the analysis")

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
