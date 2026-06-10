package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func writeTestSkillWithRisk(t *testing.T, root, relativePath, riskScore, riskEvaluator, riskReason, disabled string) (skills.Skill, string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, skills.SkillFileName)
	content := fmt.Sprintf("---\nname: example\ndescription: test\nmetadata:\n  skill-organizer:\n    risk-score: %s\n    risk-evaluator: %q\n    risk-reason: %q\n    disabled: %s\n---\n\n# Example\n", riskScore, riskEvaluator, riskReason, disabled)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	flat := strings.ReplaceAll(relativePath, "/", "--")
	return skills.Skill{
		Dir:           dir,
		SkillFile:     path,
		RelativePath:  relativePath,
		FlattenedName: flat,
	}, path
}

func TestEnableHighRiskSkillPromptsConfirmation(t *testing.T) {
	root := t.TempDir()
	_, path := writeTestSkillWithRisk(t, root, "personal/example", "85", "claude", "Contains shell execution", "true")

	originalConfirm := enableConfirm
	originalRewrite := enableRewriteManagedFields
	enableConfirm = func(prompt string, defaultValue bool) (bool, error) {
		return false, nil
	}
	enableRewriteManagedFields = func(_ skills.Skill, _ bool, _ bool) error { return nil }
	t.Cleanup(func() {
		enableConfirm = originalConfirm
		enableRewriteManagedFields = originalRewrite
	})

	skill, err := skills.ResolveSourceSkill(root, "personal/example")
	if err != nil {
		t.Fatalf("ResolveSourceSkill() error = %v", err)
	}

	metadata, err := loadSkillMetadataForGate(skill)
	if err != nil {
		t.Fatalf("loadSkillMetadataForGate() error = %v", err)
	}
	if metadata.RiskScore < highRiskThreshold {
		t.Fatalf("RiskScore = %d, want >= %d", metadata.RiskScore, highRiskThreshold)
	}
	if strings.TrimSpace(metadata.RiskEvaluator) == "" {
		t.Fatalf("RiskEvaluator is empty, want non-empty")
	}

	accepted, err := enableConfirm("Are you sure?", false)
	if err != nil {
		t.Fatalf("enableConfirm() error = %v", err)
	}
	if accepted {
		t.Fatalf("user declined but accepted=true")
	}

	if err := enableRewriteManagedFields(skill, false, true); err != nil {
		t.Fatalf("enableRewriteManagedFields() error = %v", err)
	}

	doc, err := skills.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	reloaded := doc.ManagedMetadata()
	if !reloaded.Disabled {
		t.Fatalf("Reloaded Disabled = false, want true (skill must stay disabled on decline)")
	}
}

func TestEnableHighRiskSkillProceedsOnConfirmation(t *testing.T) {
	root := t.TempDir()
	_, path := writeTestSkillWithRisk(t, root, "personal/example", "85", "claude", "Contains shell execution", "true")

	originalConfirm := enableConfirm
	enableConfirm = func(_ string, _ bool) (bool, error) { return true, nil }
	t.Cleanup(func() { enableConfirm = originalConfirm })

	skill, err := skills.ResolveSourceSkill(root, "personal/example")
	if err != nil {
		t.Fatalf("ResolveSourceSkill() error = %v", err)
	}

	metadata, err := loadSkillMetadataForGate(skill)
	if err != nil {
		t.Fatalf("loadSkillMetadataForGate() error = %v", err)
	}
	if metadata.RiskScore < highRiskThreshold {
		t.Fatalf("RiskScore = %d, want >= %d", metadata.RiskScore, highRiskThreshold)
	}

	accepted, _ := enableConfirm("Are you sure?", false)
	if !accepted {
		t.Fatalf("user accepted but accepted=false")
	}

	if err := enableWithMetadataPreserved(skill); err != nil {
		t.Fatalf("enableWithMetadataPreserved() error = %v", err)
	}

	doc, err := skills.LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}
	reloaded := doc.ManagedMetadata()
	if reloaded.Disabled {
		t.Fatalf("Reloaded Disabled = true, want false (skill should be enabled)")
	}
	if reloaded.RiskScore != 85 {
		t.Fatalf("Reloaded RiskScore = %d, want 85 (risk fields should survive enable)", reloaded.RiskScore)
	}
	if reloaded.RiskEvaluator != "claude" {
		t.Fatalf("Reloaded RiskEvaluator = %q, want %q", reloaded.RiskEvaluator, "claude")
	}
}

func TestEnableLowRiskSkillBypassesGate(t *testing.T) {
	root := t.TempDir()
	_, _ = writeTestSkillWithRisk(t, root, "personal/example", "25", "claude", "Plain markdown", "true")

	originalConfirm := enableConfirm
	enableConfirm = func(_ string, _ bool) (bool, error) {
		t.Fatalf("enableConfirm was called for low-risk skill; gate should bypass")
		return false, nil
	}
	t.Cleanup(func() { enableConfirm = originalConfirm })

	skill, err := skills.ResolveSourceSkill(root, "personal/example")
	if err != nil {
		t.Fatalf("ResolveSourceSkill() error = %v", err)
	}
	metadata, err := loadSkillMetadataForGate(skill)
	if err != nil {
		t.Fatalf("loadSkillMetadataForGate() error = %v", err)
	}
	if metadata.RiskScore >= highRiskThreshold {
		t.Fatalf("RiskScore = %d, want < %d (low-risk fixture)", metadata.RiskScore, highRiskThreshold)
	}
}

func TestEnableUnevaluatedSkillBypassesGate(t *testing.T) {
	root := t.TempDir()
	_, _ = writeTestSkillWithRisk(t, root, "personal/example", "0", "", "", "true")

	originalConfirm := enableConfirm
	enableConfirm = func(_ string, _ bool) (bool, error) {
		t.Fatalf("enableConfirm was called for unevaluated skill; gate should bypass")
		return false, nil
	}
	t.Cleanup(func() { enableConfirm = originalConfirm })

	skill, err := skills.ResolveSourceSkill(root, "personal/example")
	if err != nil {
		t.Fatalf("ResolveSourceSkill() error = %v", err)
	}
	metadata, err := loadSkillMetadataForGate(skill)
	if err != nil {
		t.Fatalf("loadSkillMetadataForGate() error = %v", err)
	}
	if strings.TrimSpace(metadata.RiskEvaluator) != "" {
		t.Fatalf("RiskEvaluator = %q, want empty (unevaluated)", metadata.RiskEvaluator)
	}
}
