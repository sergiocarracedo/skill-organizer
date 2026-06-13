package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeSkillHash_Deterministic(t *testing.T) {
	dir := t.TempDir()
	createSkill(t, dir, "test-skill")

	hash1, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() error = %v", err)
	}
	if hash1 == "" {
		t.Fatalf("ComputeSkillHash() returned empty hash")
	}

	hash2, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() second call error = %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("ComputeSkillHash() not deterministic: %q != %q", hash1, hash2)
	}
}

func TestComputeSkillHash_ChangesWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	createSkill(t, dir, "test-skill")

	hash1, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() error = %v", err)
	}

	// Modify the content
	path := filepath.Join(dir, SkillFileName)
	content := "---\nname: test-skill\ndescription: modified\n---\n\n# Modified\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	hash2, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() after modification error = %v", err)
	}
	if hash1 == hash2 {
		t.Fatalf("ComputeSkillHash() did not change after content modification")
	}
}

func TestComputeSkillHash_ExcludesMetadata(t *testing.T) {
	dir := t.TempDir()
	createSkill(t, dir, "test-skill")

	// Get hash before any metadata is added
	hashBefore, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() error = %v", err)
	}

	// Add managed metadata (risk score, etc.) via UpdateManagedMetadata
	skill := Skill{
		Dir:           dir,
		SkillFile:     filepath.Join(dir, SkillFileName),
		RelativePath:  "test-skill",
		FlattenedName: "test--test-skill",
	}
	updates := ManagedMetadata{
		RiskScore:       85,
		RiskEvaluator:   "claude-code",
		RiskEvaluatedAt: "2026-06-10T12:00:00Z",
		RiskReason:      "Contains shell patterns",
	}
	if err := UpdateManagedMetadata(skill, updates); err != nil {
		t.Fatalf("UpdateManagedMetadata() error = %v", err)
	}

	// Hash should be the same since metadata is in the frontmatter, not the body
	hashAfter, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() after metadata update error = %v", err)
	}
	if hashBefore != hashAfter {
		t.Fatalf("ComputeSkillHash() changed after metadata update (frontmatter should be excluded)\nbefore: %q\nafter:  %q", hashBefore, hashAfter)
	}
}

func TestComputeSkillHash_IncludesExtraFiles(t *testing.T) {
	dir := t.TempDir()
	createSkill(t, dir, "test-skill")

	hashBefore, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() error = %v", err)
	}

	// Add an extra file
	extra := filepath.Join(dir, "README.md")
	if err := os.WriteFile(extra, []byte("# Extra content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	hashAfter, err := ComputeSkillHash(dir)
	if err != nil {
		t.Fatalf("ComputeSkillHash() after extra file error = %v", err)
	}
	if hashBefore == hashAfter {
		t.Fatalf("ComputeSkillHash() did not change after adding extra file")
	}
}
