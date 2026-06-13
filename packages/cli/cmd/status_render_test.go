package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

func TestFormatSkillLabel_ShowsRiskForEvaluatedSkill(t *testing.T) {
	// Low score: green
	label := formatSkillLabel(statuspkg.SkillStatus{
		State:          statuspkg.StateSynced,
		RiskScore:      15,
		RiskEvaluator:  "claude-code",
		RiskSourceHash: "abc123",
	}, "test-skill")

	stripped := stripANSI(label)
	if !strings.Contains(stripped, "[risk: 15]") {
		t.Fatalf("formatSkillLabel() missing risk tag in %q", stripped)
	}

	// High score: red
	label = formatSkillLabel(statuspkg.SkillStatus{
		State:          statuspkg.StateSynced,
		RiskScore:      85,
		RiskEvaluator:  "claude-code",
		RiskSourceHash: "abc123",
	}, "test-skill")

	stripped = stripANSI(label)
	if !strings.Contains(stripped, "[risk: 85]") {
		t.Fatalf("formatSkillLabel() missing high risk tag in %q", stripped)
	}
}

func TestFormatSkillLabel_ShowsUncheckForUnevaluated(t *testing.T) {
	label := formatSkillLabel(statuspkg.SkillStatus{
		State: statuspkg.StateSynced,
		// No RiskEvaluator set = unevaluated
	}, "test-skill")

	stripped := stripANSI(label)
	if !strings.Contains(stripped, "[risk: uncheck]") {
		t.Fatalf("formatSkillLabel() missing uncheck tag for unevaluated skill in %q", stripped)
	}
}

func TestFormatSkillLabel_ShowsStaleWhenHashMismatch(t *testing.T) {
	// Create a real skill on disk so ComputeSkillHash produces a known hash.
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillContent := "---\nname: test-skill\ndescription: test\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Compute the real hash so we have a correct baseline.
	realHash, err := skills.ComputeSkillHash(skillDir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() error = %v", err)
	}

	// Create status entry with a DIFFERENT hash → stale.
	entry := statuspkg.SkillStatus{
		Skill: skills.Skill{
			Dir:           skillDir,
			SkillFile:     filepath.Join(skillDir, "SKILL.md"),
			FlattenedName: "test--test-skill",
		},
		State:          statuspkg.StateSynced,
		RiskScore:      42,
		RiskEvaluator:  "claude-code",
		RiskSourceHash: "different_hash_that_does_not_match",
	}

	label := formatSkillLabel(entry, "test-skill")
	stripped := stripANSI(label)
	if !strings.Contains(stripped, "[risk: 42 (stale)]") {
		t.Fatalf("formatSkillLabel() missing stale tag in %q", stripped)
	}

	// Now use the correct hash — should NOT show stale.
	entry.RiskSourceHash = realHash
	label = formatSkillLabel(entry, "test-skill")
	stripped = stripANSI(label)
	if strings.Contains(stripped, "(stale)") {
		t.Fatalf("formatSkillLabel() shows stale when hash matches in %q", stripped)
	}
	if !strings.Contains(stripped, "[risk: 42]") {
		t.Fatalf("formatSkillLabel() missing risk tag for fresh entry in %q", stripped)
	}
}

func TestFormatSkillLabelShowsInstalledAndAvailableVersions(t *testing.T) {
	label := formatSkillLabel(statuspkg.SkillStatus{
		State: statuspkg.StateSynced,
		InstalledVersion: "006f8413941b59eff54a7ce64851b8a2fb79e7a3a5f1a895e97a48f01482553d",
		InstalledDate: "2026-05-01T12:00:00Z",
		AvailableVersion: "0.22.5",
		AvailableCheckedDate: "2026-05-10T13:49:48Z",
	}, "asciinema-recorder")

	for _, want := range []string{"[synced]", "installed 006f841 2026-05-01", "update 0.22.5 2026-05-10"} {
		if !strings.Contains(stripANSI(label), want) {
			t.Fatalf("formatSkillLabel() missing %q in %q", want, stripANSI(label))
		}
	}
}
