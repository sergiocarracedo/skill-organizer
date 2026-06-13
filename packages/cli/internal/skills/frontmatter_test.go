package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentManagedFieldsPreserveExtraFrontmatter(t *testing.T) {
	content := []byte("---\nname: allium\ndescription: test\nversion: 1\nauto_trigger:\n  - keywords: [\"allium\"]\n---\n\n# Body\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}

	doc.SetManagedFields("thirdparty--allium", ManagedMetadata{
		OriginalName:       "allium",
		SourceRelativePath: "thirdparty/allium",
		Disabled:           true,
		Source:             "terrylica/cc-skills",
		SourceType:         "github",
		InstalledVersion:   "abc123",
		InstalledAt:        "2026-05-09T12:00:00Z",
		RepoSkillPath:      "skills/asciinema-recorder",
		LastUpdatedAt:      "2026-05-01T12:00:00Z",
	}, true)

	marshaled, err := doc.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	output := string(marshaled)
	for _, want := range []string{
		"name: thirdparty--allium",
		"version: 1",
		"auto_trigger:",
		"original-name: allium",
		"source-relative-path: thirdparty/allium",
		"disabled: true",
		"source: terrylica/cc-skills",
		"source-type: github",
		"installed-version: abc123",
		"installed-at: \"2026-05-09T12:00:00Z\"",
		"repo-skill-path: skills/asciinema-recorder",
		"last-updated-at: \"2026-05-01T12:00:00Z\"",
		"# Body",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Marshal() output missing %q\n%s", want, output)
		}
	}
}

func TestRewriteManagedFieldsWithMetadataMergesImportFields(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "thirdparty", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path := filepath.Join(dir, SkillFileName)
	content := "---\nname: example\ndescription: test\n---\n\n# Example\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	skill := Skill{
		Dir:           dir,
		SkillFile:     path,
		RelativePath:  "thirdparty/example",
		FlattenedName: "thirdparty--example",
	}

	if err := RewriteManagedFieldsWithMetadata(skill, true, false, ManagedMetadata{
		Source:           "terrylica/cc-skills",
		InstalledVersion: "deadbeef",
	}); err != nil {
		t.Fatalf("RewriteManagedFieldsWithMetadata() error = %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	output := string(updated)
	if !strings.Contains(output, "source: terrylica/cc-skills") {
		t.Fatalf("missing source metadata\n%s", output)
	}
	if !strings.Contains(output, "installed-version: deadbeef") {
		t.Fatalf("missing installed version\n%s", output)
	}
}

func TestRewriteManagedFieldsCreatesMetadataWithoutRenaming(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "personal", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path := filepath.Join(dir, SkillFileName)
	content := "---\nname: example\ndescription: test\n---\n\n# Example\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	skill := Skill{
		Dir:           dir,
		SkillFile:     path,
		RelativePath:  "personal/example",
		FlattenedName: "personal--example",
	}

	if err := RewriteManagedFields(skill, false, true); err != nil {
		t.Fatalf("RewriteManagedFields() error = %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	output := string(updated)
	if !strings.Contains(output, "name: example") {
		t.Fatalf("RewriteManagedFields() renamed skill unexpectedly\n%s", output)
	}
	if !strings.Contains(output, "original-name: example") {
		t.Fatalf("RewriteManagedFields() missing original name\n%s", output)
	}
	if !strings.Contains(output, "disabled: true") {
		t.Fatalf("RewriteManagedFields() missing disabled flag\n%s", output)
	}
}

func TestParseDocumentAcceptsUnquotedDescriptionWithColon(t *testing.T) {
	content := []byte("---\nname: frontend-project-bootstrap\ndescription: Bootstrap modern TypeScript Frontend projects and helps with tooling: formaters, etc.\n---\n\n# Body\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}

	if doc.Name() != "frontend-project-bootstrap" {
		t.Fatalf("Name() = %q, want %q", doc.Name(), "frontend-project-bootstrap")
	}
}

func TestWithoutManagedMetadataRemovesOnlyOrganizerSection(t *testing.T) {
	content := []byte("---\nname: demo-skill\ndescription: test\nmetadata:\n  version: 1.2.3\n  skill-organizer:\n    source: owner/repo\n    installed-version: 1.2.3\n---\n\n# Demo\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}
	marshaled, err := doc.WithoutManagedMetadata().Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	output := string(marshaled)
	if strings.Contains(output, "skill-organizer:") {
		t.Fatalf("managed metadata still present\n%s", output)
	}
	if !strings.Contains(output, "version: 1.2.3") {
		t.Fatalf("non-organizer metadata removed unexpectedly\n%s", output)
	}
	if !strings.Contains(output, "# Demo") {
		t.Fatalf("body removed unexpectedly\n%s", output)
	}
}

func TestDocumentManagedMetadataIncludesRiskScore(t *testing.T) {
	content := []byte("---\nname: allium\ndescription: test\nmetadata:\n  skill-organizer:\n    risk-score: 85\n    risk-evaluated-at: \"2026-06-10T12:00:00Z\"\n    risk-evaluator: \"claude-code\"\n    risk-reason: \"Contains shell execution patterns\"\n---\n\n# Body\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}

	parsed := doc.ManagedMetadata()
	if parsed.RiskScore != 85 {
		t.Fatalf("ManagedMetadata().RiskScore = %d, want 85", parsed.RiskScore)
	}
	if parsed.RiskEvaluatedAt != "2026-06-10T12:00:00Z" {
		t.Fatalf("ManagedMetadata().RiskEvaluatedAt = %q, want %q", parsed.RiskEvaluatedAt, "2026-06-10T12:00:00Z")
	}
	if parsed.RiskEvaluator != "claude-code" {
		t.Fatalf("ManagedMetadata().RiskEvaluator = %q, want %q", parsed.RiskEvaluator, "claude-code")
	}
	if parsed.RiskReason != "Contains shell execution patterns" {
		t.Fatalf("ManagedMetadata().RiskReason = %q, want %q", parsed.RiskReason, "Contains shell execution patterns")
	}

	doc.SetManagedFields("thirdparty--allium", parsed, false)

	marshaled, err := doc.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	output := string(marshaled)
	for _, want := range []string{
		"risk-score: 85",
		"risk-evaluated-at: \"2026-06-10T12:00:00Z\"",
		"risk-evaluator: \"claude-code\"",
		"risk-reason: \"Contains shell execution patterns\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Marshal() output missing %q\n%s", want, output)
		}
	}
}

func TestMergeManagedMetadataPreservesRiskFields(t *testing.T) {
	base := ManagedMetadata{
		OriginalName:       "allium",
		SourceRelativePath: "thirdparty/allium",
		Source:             "terrylica/cc-skills",
		InstalledVersion:   "abc123",
	}

	updates := ManagedMetadata{
		RiskScore:       42,
		RiskEvaluator:   "opencode",
		RiskReason:      "Uses eval()",
		RiskEvaluatedAt: "2026-06-10T12:00:00Z",
	}

	mergeManagedMetadata(&base, updates)

	if base.RiskScore != 42 {
		t.Fatalf("mergeManagedMetadata RiskScore = %d, want 42", base.RiskScore)
	}
	if base.RiskEvaluator != "opencode" {
		t.Fatalf("mergeManagedMetadata RiskEvaluator = %q, want %q", base.RiskEvaluator, "opencode")
	}
	if base.RiskReason != "Uses eval()" {
		t.Fatalf("mergeManagedMetadata RiskReason = %q, want %q", base.RiskReason, "Uses eval()")
	}
	if base.RiskEvaluatedAt != "2026-06-10T12:00:00Z" {
		t.Fatalf("mergeManagedMetadata RiskEvaluatedAt = %q, want %q", base.RiskEvaluatedAt, "2026-06-10T12:00:00Z")
	}
	if base.OriginalName != "allium" {
		t.Fatalf("mergeManagedMetadata cleared OriginalName, got %q", base.OriginalName)
	}
}

func TestUpdateManagedMetadataRoundTrip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "thirdparty", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path := filepath.Join(dir, SkillFileName)
	content := "---\nname: example\ndescription: test\n---\n\n# Example\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	skill := Skill{
		Dir:           dir,
		SkillFile:     path,
		RelativePath:  "thirdparty/example",
		FlattenedName: "thirdparty--example",
	}

	updates := ManagedMetadata{
		RiskScore:       72,
		RiskEvaluator:   "claude",
		RiskEvaluatedAt: "2026-06-10T12:00:00Z",
		RiskReason:      "Suspicious curl pipe bash",
	}

	if err := UpdateManagedMetadata(skill, updates); err != nil {
		t.Fatalf("UpdateManagedMetadata() error = %v", err)
	}

	doc, err := LoadDocument(path)
	if err != nil {
		t.Fatalf("LoadDocument() error = %v", err)
	}

	loaded := doc.ManagedMetadata()
	if loaded.RiskScore != 72 {
		t.Fatalf("Reloaded RiskScore = %d, want 72", loaded.RiskScore)
	}
	if loaded.RiskEvaluator != "claude" {
		t.Fatalf("Reloaded RiskEvaluator = %q, want %q", loaded.RiskEvaluator, "claude")
	}
	if loaded.RiskEvaluatedAt != "2026-06-10T12:00:00Z" {
		t.Fatalf("Reloaded RiskEvaluatedAt = %q, want %q", loaded.RiskEvaluatedAt, "2026-06-10T12:00:00Z")
	}
	if loaded.RiskReason != "Suspicious curl pipe bash" {
		t.Fatalf("Reloaded RiskReason = %q, want %q", loaded.RiskReason, "Suspicious curl pipe bash")
	}
	if doc.Name() != "example" {
		t.Fatalf("UpdateManagedMetadata unexpectedly renamed skill to %q", doc.Name())
	}
}

func TestManagedMetadata_RiskSourceHashRoundTrip(t *testing.T) {
	content := []byte("---\nname: test-skill\ndescription: test\nmetadata:\n  skill-organizer:\n    risk-source-hash: \"abc123def456\"\n---\n\n# Body\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}

	parsed := doc.ManagedMetadata()
	if parsed.RiskSourceHash != "abc123def456" {
		t.Fatalf("ManagedMetadata().RiskSourceHash = %q, want %q", parsed.RiskSourceHash, "abc123def456")
	}

	parsed.RiskSourceHash = "newhash789"
	doc.SetManagedFields("test--test-skill", parsed, false)

	marshaled, err := doc.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	output := string(marshaled)
	if !strings.Contains(output, "risk-source-hash") || !strings.Contains(output, "newhash789") {
		t.Fatalf("Marshal() output missing risk-source-hash\n%s", output)
	}

	reparsed, err := ParseDocument(marshaled)
	if err != nil {
		t.Fatalf("ParseDocument() after round-trip error = %v", err)
	}
	reparsedMeta := reparsed.ManagedMetadata()
	if reparsedMeta.RiskSourceHash != "newhash789" {
		t.Fatalf("After round-trip RiskSourceHash = %q, want %q", reparsedMeta.RiskSourceHash, "newhash789")
	}
}

func TestManagedMetadataDefaultRiskScoreIsZero(t *testing.T) {
	content := []byte("---\nname: allium\ndescription: test\nmetadata:\n  skill-organizer:\n    source: terrylica/cc-skills\n---\n\n# Body\n")

	doc, err := ParseDocument(content)
	if err != nil {
		t.Fatalf("ParseDocument() error = %v", err)
	}

	parsed := doc.ManagedMetadata()
	if parsed.RiskScore != 0 {
		t.Fatalf("ManagedMetadata().RiskScore = %d, want 0", parsed.RiskScore)
	}
	if parsed.RiskEvaluator != "" {
		t.Fatalf("ManagedMetadata().RiskEvaluator = %q, want empty", parsed.RiskEvaluator)
	}
	if parsed.RiskReason != "" {
		t.Fatalf("ManagedMetadata().RiskReason = %q, want empty", parsed.RiskReason)
	}
	if parsed.RiskEvaluatedAt != "" {
		t.Fatalf("ManagedMetadata().RiskEvaluatedAt = %q, want empty", parsed.RiskEvaluatedAt)
	}
}
