package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractJSONArrayStripsNoiseAroundJSON(t *testing.T) {
	input := "Please select an agent first\n[{\"name\":\"asciinema-recorder\",\"path\":\"/tmp/skill\",\"scope\":\"global\",\"agents\":[\"universal\"]}]\nDone\n"
	got := extractJSONArray(input)
	want := "[{\"name\":\"asciinema-recorder\",\"path\":\"/tmp/skill\",\"scope\":\"global\",\"agents\":[\"universal\"]}]"
	if got != want {
		t.Fatalf("extractJSONArray() = %q, want %q", got, want)
	}
}

func TestInstalledSkillsFallsBackToDiscoveredSkillDirectories(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillDir := filepath.Join(home, ".agents", "skills", "remotion-best-practices")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "---\nname: remotion-best-practices\ndescription: test\n---\n\n# Test\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sandbox := &Sandbox{
		homeDir:    home,
		projectDir: project,
		runner:     &SkillsCLIRunner{command: []string{"sh", "-c"}},
	}

	got, err := sandbox.InstalledSkills()
	if err != nil {
		t.Fatalf("InstalledSkills() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("InstalledSkills() len = %d, want 1", len(got))
	}
	if got[0].Name != "remotion-best-practices" {
		t.Fatalf("InstalledSkills()[0].Name = %q, want %q", got[0].Name, "remotion-best-practices")
	}
	if got[0].Path != skillDir {
		t.Fatalf("InstalledSkills()[0].Path = %q, want %q", got[0].Path, skillDir)
	}
}

func TestResolveVersionPrefersSemanticVersionOverHash(t *testing.T) {
	got := ResolveVersion(SkillSummary{Hash: "deadbeef", Version: "0.22.4"}, []File{{Path: "SKILL.md", Contents: "---\nmetadata:\n  version: 0.22.3\n---\n"}})
	if got != "0.22.4" {
		t.Fatalf("ResolveVersion() = %q, want %q", got, "0.22.4")
	}
}

func TestResolveVersionFallsBackToSkillFileVersionBeforeHash(t *testing.T) {
	got := ResolveVersion(SkillSummary{Hash: "deadbeef"}, []File{{Path: "SKILL.md", Contents: "---\nmetadata:\n  version: 0.22.4\n---\n"}})
	if got != "0.22.4" {
		t.Fatalf("ResolveVersion() = %q, want %q", got, "0.22.4")
	}
}

func TestParseSkillFindOutputParsesRankedResults(t *testing.T) {
	output := "Install with npx skills add <owner/repo@skill>\n\nowner/repo@demo-skill 1.0K installs\n└ https://skills.sh/owner/repo/demo-skill\n\nsecond/repo@other 0.5K installs\n└ https://skills.sh/second/repo/other\n"
	results := parseSkillFindOutput(output)
	if len(results) != 2 {
		t.Fatalf("parseSkillFindOutput() len = %d, want 2", len(results))
	}
	if results[0].Skill.Source != "owner/repo" || results[0].Skill.Name != "demo-skill" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[0].PageURL != "https://skills.sh/owner/repo/demo-skill" {
		t.Fatalf("first page url = %q", results[0].PageURL)
	}
}
