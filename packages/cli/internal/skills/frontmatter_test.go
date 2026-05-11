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
