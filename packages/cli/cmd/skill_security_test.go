package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	securitypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/security"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestCheckSecurityPrintPromptBypassesToolDetection(t *testing.T) {
	originalPrintFunc := securityPrintPromptFunc
	originalLoad := securityLoadResolvedLocation
	originalCollect := securityCollectSkills
	originalDetect := securityDetectInstalledTools

	securityLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	securityCollectSkills = func(_ configpkg.Location, _ bool) ([]securitypkg.SkillInfo, error) {
		return []securitypkg.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	securityDetectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return nil, fmt.Errorf("should not be called")
	}
	printed := ""
	securityPrintPromptFunc = func(prompt string) {
		printed = prompt
	}
	t.Cleanup(func() {
		securityPrintPromptFunc = originalPrintFunc
		securityLoadResolvedLocation = originalLoad
		securityCollectSkills = originalCollect
		securityDetectInstalledTools = originalDetect
	})

	cmd := newCheckSecurityCommand()
	securityPrintPrompt = true
	t.Cleanup(func() { securityPrintPrompt = false })

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}
	if printed == "" {
		t.Fatalf("RunE() output is empty")
	}
	if !strings.Contains(printed, "risk-score") {
		t.Fatalf("RunE() output missing risk-score: %q", printed)
	}
}

func TestCheckSecurityNoToolsDetectedPrintsPromptAndExits0(t *testing.T) {
	originalPrintFunc := securityPrintPromptFunc
	originalLoad := securityLoadResolvedLocation
	originalCollect := securityCollectSkills
	originalDetect := securityDetectInstalledTools

	securityLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	securityCollectSkills = func(_ configpkg.Location, _ bool) ([]securitypkg.SkillInfo, error) {
		return []securitypkg.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	securityDetectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return []agenttools.InstalledTool{}, nil
	}
	printed := ""
	securityPrintPromptFunc = func(prompt string) {
		printed = prompt
	}
	t.Cleanup(func() {
		securityPrintPromptFunc = originalPrintFunc
		securityLoadResolvedLocation = originalLoad
		securityCollectSkills = originalCollect
		securityDetectInstalledTools = originalDetect
	})

	cmd := newCheckSecurityCommand()
	securityPrintPrompt = false
	t.Cleanup(func() { securityPrintPrompt = false })

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}
	if printed == "" {
		t.Fatalf("RunE() did not print prompt when no tools detected")
	}
	if !strings.Contains(printed, "security") {
		t.Fatalf("printed prompt missing security keyword: %q", printed)
	}
}

func TestCheckSecurityStoresRiskScoreOnLowRisk(t *testing.T) {
	originalLoad := securityLoadResolvedLocation
	originalCollect := securityCollectSkills
	originalDetect := securityDetectInstalledTools
	originalLoadConfig := securityLoadConfigFunc
	originalSaveConfig := securitySaveConfigFunc
	originalRunAnalysis := securityRunAnalysis
	originalUpdate := securityUpdateMetadata
	originalWriteDisabled := securityWriteDisabled
	originalConfirm := securityConfirm
	originalSpinner := agenttools.StartSpinnerFunc

	securityLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	securityCollectSkills = func(_ configpkg.Location, _ bool) ([]securitypkg.SkillInfo, error) {
		return []securitypkg.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	securityDetectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return []agenttools.InstalledTool{mockInstalledTool("codex", "codex")}, nil
	}
	securityLoadConfigFunc = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultAgentTool: "codex", AcknowledgedExternalToolCosts: true}, nil
	}
	securitySaveConfigFunc = func(_ string, _ configpkg.AgentSelectionConfig) error { return nil }
	securityRunAnalysis = func(_ context.Context, _ agenttools.InstalledTool, _ string, _ func(string)) (securitypkg.SecurityReport, error) {
		return securitypkg.SecurityReport{Results: []securitypkg.SkillResult{{
			Name:       "personal--alpha",
			RiskScore:  25,
			RiskReason: "Plain markdown",
		}}}, nil
	}
	var captured skills.ManagedMetadata
	securityUpdateMetadata = func(_ skills.Skill, updates skills.ManagedMetadata) error {
		captured = updates
		return nil
	}
	disabledCalled := false
	securityWriteDisabled = func(_ skills.Skill, _ bool, _ bool) error {
		disabledCalled = true
		return nil
	}
	securityConfirm = func(_ string, _ bool) (bool, error) {
		return false, nil
	}
	agenttools.StartSpinnerFunc = func(_ string) (agenttools.SpinnerHandle, error) {
		return stubSpinner{}, nil
	}
	t.Cleanup(func() {
		securityLoadResolvedLocation = originalLoad
		securityCollectSkills = originalCollect
		securityDetectInstalledTools = originalDetect
		securityLoadConfigFunc = originalLoadConfig
		securitySaveConfigFunc = originalSaveConfig
		securityRunAnalysis = originalRunAnalysis
		securityUpdateMetadata = originalUpdate
		securityWriteDisabled = originalWriteDisabled
		securityConfirm = originalConfirm
		agenttools.StartSpinnerFunc = originalSpinner
	})

	cmd := newCheckSecurityCommand()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if captured.RiskScore != 25 {
		t.Fatalf("UpdateManagedMetadata RiskScore = %d, want 25", captured.RiskScore)
	}
	if captured.RiskEvaluator != "codex" {
		t.Fatalf("UpdateManagedMetadata RiskEvaluator = %q, want %q", captured.RiskEvaluator, "codex")
	}
	if captured.RiskReason != "Plain markdown" {
		t.Fatalf("UpdateManagedMetadata RiskReason = %q, want %q", captured.RiskReason, "Plain markdown")
	}
	if captured.RiskEvaluatedAt == "" {
		t.Fatalf("UpdateManagedMetadata RiskEvaluatedAt is empty")
	}
	if disabledCalled {
		t.Fatalf("WriteDisabled was called for low-risk skill (risk-score < 70)")
	}
}

func TestCheckSecurityPromptsToDisableHighRisk(t *testing.T) {
	originalLoad := securityLoadResolvedLocation
	originalCollect := securityCollectSkills
	originalDetect := securityDetectInstalledTools
	originalLoadConfig := securityLoadConfigFunc
	originalSaveConfig := securitySaveConfigFunc
	originalRunAnalysis := securityRunAnalysis
	originalUpdate := securityUpdateMetadata
	originalWriteDisabled := securityWriteDisabled
	originalConfirm := securityConfirm
	originalSpinner := agenttools.StartSpinnerFunc

	securityLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	securityCollectSkills = func(_ configpkg.Location, _ bool) ([]securitypkg.SkillInfo, error) {
		return []securitypkg.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	securityDetectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return []agenttools.InstalledTool{mockInstalledTool("codex", "codex")}, nil
	}
	securityLoadConfigFunc = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultAgentTool: "codex", AcknowledgedExternalToolCosts: true}, nil
	}
	securitySaveConfigFunc = func(_ string, _ configpkg.AgentSelectionConfig) error { return nil }
	securityRunAnalysis = func(_ context.Context, _ agenttools.InstalledTool, _ string, _ func(string)) (securitypkg.SecurityReport, error) {
		return securitypkg.SecurityReport{Results: []securitypkg.SkillResult{{
			Name:       "personal--alpha",
			RiskScore:  85,
			RiskReason: "Uses eval()",
		}}}, nil
	}
	securityUpdateMetadata = func(_ skills.Skill, _ skills.ManagedMetadata) error { return nil }
	disabledCalled := false
	securityWriteDisabled = func(_ skills.Skill, _ bool, disabled bool) error {
		disabledCalled = true
		if !disabled {
			t.Fatalf("WriteDisabled called with disabled=false for high-risk skill")
		}
		return nil
	}
	securityConfirm = func(prompt string, defaultValue bool) (bool, error) {
		if !strings.Contains(prompt, "alpha") {
			t.Fatalf("confirm prompt = %q, want to mention alpha", prompt)
		}
		return true, nil
	}
	agenttools.StartSpinnerFunc = func(_ string) (agenttools.SpinnerHandle, error) {
		return stubSpinner{}, nil
	}
	t.Cleanup(func() {
		securityLoadResolvedLocation = originalLoad
		securityCollectSkills = originalCollect
		securityDetectInstalledTools = originalDetect
		securityLoadConfigFunc = originalLoadConfig
		securitySaveConfigFunc = originalSaveConfig
		securityRunAnalysis = originalRunAnalysis
		securityUpdateMetadata = originalUpdate
		securityWriteDisabled = originalWriteDisabled
		securityConfirm = originalConfirm
		agenttools.StartSpinnerFunc = originalSpinner
	})

	cmd := newCheckSecurityCommand()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if !disabledCalled {
		t.Fatalf("WriteDisabled was not called for high-risk skill")
	}
}

func TestCheckSecurityWritesScoreEvenWhenDecliningDisable(t *testing.T) {
	originalLoad := securityLoadResolvedLocation
	originalCollect := securityCollectSkills
	originalDetect := securityDetectInstalledTools
	originalLoadConfig := securityLoadConfigFunc
	originalSaveConfig := securitySaveConfigFunc
	originalRunAnalysis := securityRunAnalysis
	originalUpdate := securityUpdateMetadata
	originalWriteDisabled := securityWriteDisabled
	originalConfirm := securityConfirm
	originalSpinner := agenttools.StartSpinnerFunc

	securityLoadResolvedLocation = func() (string, configpkg.Location, error) {
		return "/tmp/.skill-organizer.yml", configpkg.Location{Source: "/tmp/source", Target: "/tmp/target"}, nil
	}
	securityCollectSkills = func(_ configpkg.Location, _ bool) ([]securitypkg.SkillInfo, error) {
		return []securitypkg.SkillInfo{{
			Name:          "alpha",
			RelativePath:  "personal/alpha",
			FlattenedName: "personal--alpha",
			Description:   "Alpha description",
		}}, nil
	}
	securityDetectInstalledTools = func() ([]agenttools.InstalledTool, error) {
		return []agenttools.InstalledTool{mockInstalledTool("codex", "codex")}, nil
	}
	securityLoadConfigFunc = func(_ string) (configpkg.AgentSelectionConfig, error) {
		return configpkg.AgentSelectionConfig{DefaultAgentTool: "codex", AcknowledgedExternalToolCosts: true}, nil
	}
	securitySaveConfigFunc = func(_ string, _ configpkg.AgentSelectionConfig) error { return nil }
	securityRunAnalysis = func(_ context.Context, _ agenttools.InstalledTool, _ string, _ func(string)) (securitypkg.SecurityReport, error) {
		return securitypkg.SecurityReport{Results: []securitypkg.SkillResult{{
			Name:       "personal--alpha",
			RiskScore:  85,
			RiskReason: "Uses eval()",
		}}}, nil
	}
	updateCalled := false
	var captured skills.ManagedMetadata
	securityUpdateMetadata = func(_ skills.Skill, updates skills.ManagedMetadata) error {
		updateCalled = true
		captured = updates
		return nil
	}
	securityWriteDisabled = func(_ skills.Skill, _ bool, _ bool) error {
		t.Fatalf("WriteDisabled should not be called when user declines")
		return nil
	}
	securityConfirm = func(_ string, _ bool) (bool, error) {
		return false, nil
	}
	agenttools.StartSpinnerFunc = func(_ string) (agenttools.SpinnerHandle, error) {
		return stubSpinner{}, nil
	}
	t.Cleanup(func() {
		securityLoadResolvedLocation = originalLoad
		securityCollectSkills = originalCollect
		securityDetectInstalledTools = originalDetect
		securityLoadConfigFunc = originalLoadConfig
		securitySaveConfigFunc = originalSaveConfig
		securityRunAnalysis = originalRunAnalysis
		securityUpdateMetadata = originalUpdate
		securityWriteDisabled = originalWriteDisabled
		securityConfirm = originalConfirm
		agenttools.StartSpinnerFunc = originalSpinner
	})

	cmd := newCheckSecurityCommand()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE() error = %v", err)
	}

	if !updateCalled {
		t.Fatalf("UpdateManagedMetadata was not called when user declined disable")
	}
	if captured.RiskScore != 85 {
		t.Fatalf("UpdateManagedMetadata RiskScore = %d, want 85", captured.RiskScore)
	}
}
