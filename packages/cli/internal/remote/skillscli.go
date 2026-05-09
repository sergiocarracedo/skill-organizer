package remote

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type skillsCLIRunner struct {
	command []string
}

type skillsCLISandbox struct {
	root       string
	projectDir string
	homeDir    string
	runner     *skillsCLIRunner
}

type skillsCLIInstalledEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Scope string `json:"scope"`
	Agents []string `json:"agents"`
}

type skillsCLIMetadata struct {
	Version string `json:"version"`
	Date    string `json:"date"`
}

type skillsCLILockFile struct {
	Skills map[string]struct {
		Source       string `json:"source"`
		SourceType   string `json:"sourceType"`
		SkillPath    string `json:"skillPath"`
		ComputedHash string `json:"computedHash"`
	} `json:"skills"`
}

type SkillSummary struct {
	Provider      string
	ID            string
	Name          string
	Source        string
	SourceURL     string
	Version       string
	VersionDate   time.Time
	Hash          string
	RepoSkillPath string
}

type SkillBundle struct {
	Skill SkillSummary
	Files []File
}

type File struct {
	Path     string
	Contents string
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var listedSkillPattern = regexp.MustCompile(`^\s*[│|]\s{4}([a-z0-9][a-z0-9-]+)\s*$`)

func detectSkillsCLI() (*skillsCLIRunner, error) {
	if _, err := exec.LookPath("skills"); err == nil {
		return &skillsCLIRunner{command: []string{"skills"}}, nil
	}
	if _, err := exec.LookPath("npx"); err == nil {
		return &skillsCLIRunner{command: []string{"npx", "skills"}}, nil
	}
	return nil, fmt.Errorf("skills CLI not available: install `skills` or ensure `npx skills` can run")
}

func newSkillsCLISandbox() (*skillsCLISandbox, error) {
	runner, err := detectSkillsCLI()
	if err != nil {
		return nil, err
	}

	root, err := os.MkdirTemp("", "skill-organizer-skills-")
	if err != nil {
		return nil, fmt.Errorf("create skills sandbox: %w", err)
	}
	projectDir := filepath.Join(root, "project")
	homeDir := filepath.Join(root, "home")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sandbox project directory: %w", err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sandbox home directory: %w", err)
	}

	return &skillsCLISandbox{root: root, projectDir: projectDir, homeDir: homeDir, runner: runner}, nil
}

func (s *skillsCLISandbox) Close() {
	_ = os.RemoveAll(s.root)
}

func (s *skillsCLISandbox) run(args ...string) (string, error) {
	command := exec.Command(s.runner.command[0], append(s.runner.command[1:], args...)...)
	command.Dir = s.projectDir
	command.Env = append(os.Environ(), "HOME="+s.homeDir, "FORCE_COLOR=0", "NO_COLOR=1", "CI=1")
	output, err := command.CombinedOutput()
	text := string(output)
	if err != nil {
		return text, fmt.Errorf("run %s: %w\n%s", strings.Join(append(s.runner.command, args...), " "), err, text)
	}
	return text, nil
}

func (s *skillsCLISandbox) listRepoSkills(source string) ([]SkillSummary, error) {
	output, err := s.run("add", source, "-l")
	if err != nil {
		return nil, err
	}

	clean := stripANSI(output)
	lines := strings.Split(clean, "\n")
	results := make([]SkillSummary, 0)
	for _, line := range lines {
		matches := listedSkillPattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		name := matches[1]
		results = append(results, SkillSummary{
			Provider:  "skills.sh",
			ID:        source + "/" + name,
			Name:      name,
			Source:    source,
			SourceURL: sourceToGitHubURL(source),
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no skills found in %s", source)
	}
	return results, nil
}

func (s *skillsCLISandbox) installSkill(skill SkillSummary) (string, error) {
	args := []string{"add", skill.SourceURL, "--skill", skill.Name, "-y", "--copy"}
	return s.run(args...)
}

func (s *skillsCLISandbox) installedSkills() ([]skillsCLIInstalledEntry, error) {
	output, err := s.run("list", "--json")
	if err != nil {
		return nil, err
	}

	var entries []skillsCLIInstalledEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, fmt.Errorf("parse skills list output: %w", err)
	}
	return entries, nil
}

func (s *skillsCLISandbox) loadInstalledBundle(skill SkillSummary) (SkillBundle, error) {
	entries, err := s.installedSkills()
	if err != nil {
		return SkillBundle{}, err
	}

	var installed skillsCLIInstalledEntry
	found := false
	for _, entry := range entries {
		if entry.Name == skill.Name {
			installed = entry
			found = true
			break
		}
	}
	if !found {
		return SkillBundle{}, fmt.Errorf("installed skill %q not found in sandbox", skill.Name)
	}

	files := make([]File, 0)
	err = filepath.WalkDir(installed.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(installed.Path, path)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Contents: string(content)})
		return nil
	})
	if err != nil {
		return SkillBundle{}, fmt.Errorf("read sandbox installed skill: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	updated := skill
	metadata, _ := loadSkillsCLIMetadata(filepath.Join(installed.Path, "metadata.json"))
	if metadata.Version != "" {
		updated.Version = metadata.Version
	}
	if metadata.Date != "" {
		updated.VersionDate = parseLooseDate(metadata.Date)
	}
	lock, _ := loadSkillsCLILock(filepath.Join(s.projectDir, "skills-lock.json"))
	if entry, ok := lock.Skills[skill.Name]; ok {
		updated.Hash = entry.ComputedHash
		if entry.SkillPath != "" {
			updated.RepoSkillPath = strings.TrimSuffix(filepath.ToSlash(filepath.Dir(entry.SkillPath)), "/")
		}
		if entry.Source != "" {
			updated.Source = entry.Source
			updated.SourceURL = sourceToGitHubURL(entry.Source)
		}
	}

	return SkillBundle{Skill: updated, Files: files}, nil
}

func loadSkillsCLIMetadata(path string) (skillsCLIMetadata, error) {
	var metadata skillsCLIMetadata
	content, err := os.ReadFile(path)
	if err != nil {
		return metadata, err
	}
	err = json.Unmarshal(content, &metadata)
	return metadata, err
}

func loadSkillsCLILock(path string) (skillsCLILockFile, error) {
	var lock skillsCLILockFile
	content, err := os.ReadFile(path)
	if err != nil {
		return lock, err
	}
	err = json.Unmarshal(content, &lock)
	return lock, err
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func sourceToGitHubURL(source string) string {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "https://github.com/" + strings.Trim(trimmed, "/")
}

func parseLooseDate(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "January 2006", "Jan 2006", "2006-01-02"} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

type skillFrontmatter struct {
	Metadata map[string]any `yaml:"metadata"`
}

func skillVersionFromSkillFile(content string) string {
	var frontmatter skillFrontmatter
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return ""
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
		return ""
	}
	metadata, ok := frontmatter.Metadata["version"]
	if !ok {
		return ""
	}
	if version, ok := metadata.(string); ok {
		return strings.TrimSpace(version)
	}
	return ""
}
