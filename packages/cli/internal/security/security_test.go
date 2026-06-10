package security

import (
	"context"
	"strings"
	"testing"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
)

func TestParseSecurityReport(t *testing.T) {
	input := `{"results":[{"name":"test--skill","risk-score":85,"risk-reason":"Uses eval()"}]}`

	report, err := ParseReport(input)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}

	if len(report.Results) != 1 {
		t.Fatalf("ParseReport() Results len = %d, want 1", len(report.Results))
	}
	if report.Results[0].Name != "test--skill" {
		t.Fatalf("ParseReport() Results[0].Name = %q, want %q", report.Results[0].Name, "test--skill")
	}
	if report.Results[0].RiskScore != 85 {
		t.Fatalf("ParseReport() Results[0].RiskScore = %d, want 85", report.Results[0].RiskScore)
	}
	if report.Results[0].RiskReason != "Uses eval()" {
		t.Fatalf("ParseReport() Results[0].RiskReason = %q, want %q", report.Results[0].RiskReason, "Uses eval()")
	}
}

func TestParseSecurityReportWithCodeFences(t *testing.T) {
	input := "```json\n" + `{"results":[{"name":"test--skill","risk-score":50,"risk-reason":"Reads env vars"}]}` + "\n```"

	report, err := ParseReport(input)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}

	if len(report.Results) != 1 {
		t.Fatalf("ParseReport() Results len = %d, want 1", len(report.Results))
	}
	if report.Results[0].RiskScore != 50 {
		t.Fatalf("ParseReport() Results[0].RiskScore = %d, want 50", report.Results[0].RiskScore)
	}
}

func TestBuildPromptIncludesSkills(t *testing.T) {
	items := []SkillInfo{
		{
			Name:          "demo",
			FlattenedName: "thirdparty--demo",
			RelativePath:  "thirdparty/demo",
			Description:   "A demo skill",
		},
	}

	prompt := BuildPrompt(items)
	if !strings.Contains(prompt, "thirdparty--demo") {
		t.Fatalf("BuildPrompt() output missing flattened name\n%s", prompt)
	}
	if !strings.Contains(prompt, "risk-score") {
		t.Fatalf("BuildPrompt() output missing risk-score keyword\n%s", prompt)
	}
	if !strings.Contains(prompt, "security") {
		t.Fatalf("BuildPrompt() output missing security keyword\n%s", prompt)
	}
}

func TestRunWithFakeCommand(t *testing.T) {
	original := commandRunner
	commandRunner = func(_ context.Context, _ string, _ []string, _ func(string)) (string, error) {
		return `{"results":[{"name":"alpha","risk-score":15,"risk-reason":"Safe"}]}`, nil
	}
	t.Cleanup(func() { commandRunner = original })

	tool, ok := agenttools.FindSupported("codex")
	if !ok {
		t.Fatal("FindSupported(codex) returned false")
	}
	installed := agenttools.InstalledTool{Tool: tool, Binary: "codex"}

	report, err := Run(context.Background(), installed, "prompt", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("Run() Results len = %d, want 1", len(report.Results))
	}
	if report.Results[0].RiskScore != 15 {
		t.Fatalf("Run() Results[0].RiskScore = %d, want 15", report.Results[0].RiskScore)
	}
}

func TestParseReportClampsRiskScore(t *testing.T) {
	input := `{"results":[{"name":"a","risk-score":150,"risk-reason":"x"}]}`

	report, err := ParseReport(input)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}
	if report.Results[0].RiskScore != 100 {
		t.Fatalf("RiskScore = %d, want 100 (clamped)", report.Results[0].RiskScore)
	}
}

func TestParseReportClampsNegativeRiskScore(t *testing.T) {
	input := `{"results":[{"name":"a","risk-score":-50,"risk-reason":"x"}]}`

	report, err := ParseReport(input)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}
	if report.Results[0].RiskScore != 0 {
		t.Fatalf("RiskScore = %d, want 0 (clamped)", report.Results[0].RiskScore)
	}
}
