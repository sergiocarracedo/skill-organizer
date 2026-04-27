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
		"# Body",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Marshal() output missing %q\n%s", want, output)
		}
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
