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

func TestParseSecurityReportNewSchema(t *testing.T) {
	input := `{"results":[{"name":"test--skill","analysis":"Downloads and executes remote code with environment variable exfiltration.","scores":{"obfuscation_evasion":20,"system_impact":85,"network_exfiltration":90,"prompt_hijacking":10,"deception_index":40},"overall_risk_level":"CRITICAL"}]}`

	report, err := ParseReport(input)
	if err != nil {
		t.Fatalf("ParseReport() error = %v", err)
	}

	if len(report.Results) != 1 {
		t.Fatalf("ParseReport() Results len = %d, want 1", len(report.Results))
	}

	r := report.Results[0]
	if r.Name != "test--skill" {
		t.Fatalf("Name = %q, want %q", r.Name, "test--skill")
	}
	if r.Analysis == "" {
		t.Fatal("Analysis should not be empty")
	}
	if r.OverallRiskLevel != "CRITICAL" {
		t.Fatalf("OverallRiskLevel = %q, want %q", r.OverallRiskLevel, "CRITICAL")
	}
	if r.RiskScore != 90 {
		t.Fatalf("RiskScore (computed) = %d, want 90 (max of scores)", r.RiskScore)
	}
	if !strings.Contains(r.RiskReason, "Downloads and executes") {
		t.Fatalf("RiskReason = %q, should contain analysis text", r.RiskReason)
	}
	if len(r.Scores) != 5 {
		t.Fatalf("Scores map length = %d, want 5", len(r.Scores))
	}
	if r.Scores["system_impact"] != 85 {
		t.Fatalf("Scores[system_impact] = %d, want 85", r.Scores["system_impact"])
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
			Content:       "# Demo\n\nThis is the body.",
		},
	}

	prompt := BuildPrompt(items)
	if !strings.Contains(prompt, "thirdparty--demo") {
		t.Fatalf("BuildPrompt() output missing flattened name\n%s", prompt)
	}
	if !strings.Contains(prompt, "obfuscation_evasion") {
		t.Fatalf("BuildPrompt() output missing obfuscation_evasion keyword\n%s", prompt)
	}
	if !strings.Contains(prompt, "security") {
		t.Fatalf("BuildPrompt() output missing security keyword\n%s", prompt)
	}
	if !strings.Contains(prompt, "This is the body.") {
		t.Fatalf("BuildPrompt() output missing skill content\n%s", prompt)
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
