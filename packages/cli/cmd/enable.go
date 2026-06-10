package cmd

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

var (
	enableConfirm              = confirm
	enableRewriteManagedFields = skills.RewriteManagedFields
	enableUpdateManagedFields  = skills.UpdateManagedMetadata
	enableSetDisabled          = skills.SetDisabled
)

func newEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "enable <source-path>",
		Aliases: []string{"on"},
		Short:   "Enable a source skill by source path",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			skill, err := skills.ResolveSourceSkill(location.Source, args[0])
			if err != nil {
				return err
			}

			metadata, err := loadSkillMetadataForGate(skill)
			if err == nil && metadata.RiskScore >= highRiskThreshold && strings.TrimSpace(metadata.RiskEvaluator) != "" {
				pterm.Warning.Printfln("This skill has a high risk score of %d/100 (evaluated by %s)", metadata.RiskScore, metadata.RiskEvaluator)
				if strings.TrimSpace(metadata.RiskReason) != "" {
					pterm.Warning.Printfln("Reason: %s", metadata.RiskReason)
				}

				accepted, confirmErr := enableConfirm("Are you sure you want to enable this high-risk skill?", false)
				if confirmErr != nil {
					return confirmErr
				}
				if !accepted {
					if err := enableSetDisabled(skill, true); err != nil {
						return err
					}
					return fmt.Errorf("aborted: skill remains disabled")
				}
			}

			if err := enableWithMetadataPreserved(skill); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}

			pterm.Success.Printfln("Enabled skill: %s", skill.RelativePath)
			printSyncResult(configFile, result)
			return nil
		},
	}
}

func loadSkillMetadataForGate(skill skills.Skill) (skills.ManagedMetadata, error) {
	doc, err := skills.LoadDocument(skill.SkillFile)
	if err != nil {
		return skills.ManagedMetadata{}, err
	}
	return doc.ManagedMetadata(), nil
}

func enableWithMetadataPreserved(skill skills.Skill) error {
	return skills.SetDisabled(skill, false)
}
