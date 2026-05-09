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

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"gopkg.in/yaml.v3"
)

type SkillsCLIRunner struct {
	command []string
}

type Sandbox struct {
	root       string
	projectDir string
	homeDir    string
	runner     *SkillsCLIRunner
}

type skillsCLIInstalledEntry struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Scope  string   `json:"scope"`
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
	SourceType    string
	Version       string
	VersionDate   time.Time
	Hash          string
	RepoSkillPath string
}

type SkillBundle struct {
	Skill SkillSummary
	Root  string
	Files []File
}

type File struct {
	Path     string
	Contents string
}

type SkillsUpdate struct {
	Skill           SkillSummary
	InstalledPath   string
	InstalledBundle SkillBundle
	LatestBundle    SkillBundle
}

type skillFrontmatter struct {
	Metadata map[string]any `yaml:"metadata"`
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var listedSkillPattern = regexp.MustCompile(`^\s*[│|]\s{4}([a-z0-9][a-z0-9-]+)\s*$`)

func DetectSkillsCLI() (*SkillsCLIRunner, error) {
	if _, err := exec.LookPath("skills"); err == nil {
		return &SkillsCLIRunner{command: []string{"skills"}}, nil
	}
	if _, err := exec.LookPath("npx"); err == nil {
		return &SkillsCLIRunner{command: []string{"npx", "skills"}}, nil
	}
	return nil, fmt.Errorf("skills CLI not available. Install `skills` or ensure `npx skills` can run")
}

func (r *SkillsCLIRunner) Label() string {
	if r == nil {
		return ""
	}
	return strings.Join(r.command, " ")
}

func (r *SkillsCLIRunner) RunIn(dir string, env []string, args ...string) (string, error) {
	command := exec.Command(r.command[0], append(r.command[1:], args...)...)
	if strings.TrimSpace(dir) != "" {
		command.Dir = dir
	}
	if len(env) > 0 {
		command.Env = append(os.Environ(), env...)
	}
	output, err := command.CombinedOutput()
	text := string(output)
	if err != nil {
		return text, fmt.Errorf("run %s: %w\n%s", strings.Join(append(r.command, args...), " "), err, text)
	}
	return text, nil
}

func NewSandbox() (*Sandbox, error) {
	runner, err := DetectSkillsCLI()
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

	return &Sandbox{root: root, projectDir: projectDir, homeDir: homeDir, runner: runner}, nil
}

func (s *Sandbox) Close() {
	_ = os.RemoveAll(s.root)
}

func (s *Sandbox) Run(args ...string) (string, error) {
	return s.runner.RunIn(s.projectDir, []string{"HOME=" + s.homeDir, "FORCE_COLOR=0", "NO_COLOR=1", "CI=1"}, args...)
}

func (s *Sandbox) ListRepoSkills(source string) ([]SkillSummary, error) {
	output, err := s.Run("add", source, "-l")
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
			Provider:   "skills.sh",
			ID:         source + "/" + name,
			Name:       name,
			Source:     source,
			SourceURL:  sourceToGitHubURL(source),
			SourceType: sourceTypeFromSource(source),
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no skills found in %s", source)
	}
	return results, nil
}

func (s *Sandbox) InstallSkill(skill SkillSummary) (string, error) {
	args := []string{"add", skill.SourceURL, "--skill", skill.Name, "-y", "--copy"}
	return s.Run(args...)
}

func (s *Sandbox) InstalledSkills() ([]skillsCLIInstalledEntry, error) {
	output, err := s.Run("list", "--json")
	if err != nil {
		return nil, err
	}

	var entries []skillsCLIInstalledEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, fmt.Errorf("parse skills list output: %w", err)
	}
	return entries, nil
}

func (s *Sandbox) LoadInstalledBundle(skill SkillSummary) (SkillBundle, error) {
	entries, err := s.InstalledSkills()
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

	bundle, err := LoadBundleFromDir(installed.Path)
	if err != nil {
		return SkillBundle{}, err
	}

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
			updated.SourceType = sourceTypeFromSource(entry.Source)
		}
		if entry.SourceType != "" {
			updated.SourceType = strings.TrimSpace(entry.SourceType)
		}
	}

	bundle.Skill = updated
	bundle.Root = installed.Path
	return bundle, nil
}

func LoadBundleFromDir(root string) (SkillBundle, error) {
	files := make([]File, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Contents: string(content)})
		return nil
	})
	if err != nil {
		return SkillBundle{}, fmt.Errorf("read skill bundle: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return SkillBundle{Root: root, Files: files}, nil
}

func LatestFileModTime(root string) (time.Time, error) {
	var latest time.Time
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime().UTC()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("scan skill modtimes: %w", err)
	}
	return latest, nil
}

func CheckForSkillUpdate(ctx SkillSummary) (SkillsUpdate, bool, error) {
	if strings.TrimSpace(ctx.Source) == "" || strings.TrimSpace(ctx.RepoSkillPath) == "" {
		return SkillsUpdate{}, false, nil
	}
	sandbox, err := NewSandbox()
	if err != nil {
		return SkillsUpdate{}, false, err
	}
	defer sandbox.Close()

	output, err := sandbox.Run("add", ctx.SourceURL, "--skill", ctx.Name, "-y", "--copy")
	if err != nil {
		return SkillsUpdate{}, false, err
	}
	_ = output
	latestBundle, err := sandbox.LoadInstalledBundle(ctx)
	if err != nil {
		return SkillsUpdate{}, false, err
	}
	installedBundle, err := LoadBundleFromDir(ctx.ID)
	if err != nil {
		return SkillsUpdate{}, false, nil
	}
	if latestBundle.Skill.Hash == "" || latestBundle.Skill.Hash == ctx.Hash || latestBundle.Skill.Hash == ctx.Version {
		return SkillsUpdate{}, false, nil
	}
	return SkillsUpdate{Skill: ctx, InstalledPath: ctx.ID, InstalledBundle: installedBundle, LatestBundle: latestBundle}, true, nil
}

func FetchSkillBundle(source string, name string) (SkillBundle, error) {
	sandbox, err := NewSandbox()
	if err != nil {
		return SkillBundle{}, err
	}
	defer sandbox.Close()

	summary := SkillSummary{
		Provider:   "skills.sh",
		Name:       strings.TrimSpace(name),
		Source:     strings.TrimSpace(source),
		SourceURL:  sourceToGitHubURL(source),
		SourceType: sourceTypeFromSource(source),
	}
	if _, err := sandbox.InstallSkill(summary); err != nil {
		return SkillBundle{}, err
	}
	return sandbox.LoadInstalledBundle(summary)
}

func ResolveVersion(skill SkillSummary, files []File) string {
	if strings.TrimSpace(skill.Hash) != "" {
		return strings.TrimSpace(skill.Hash)
	}
	if strings.TrimSpace(skill.Version) != "" {
		return strings.TrimSpace(skill.Version)
	}
	for _, file := range files {
		if file.Path == "SKILL.md" {
			if version := SkillVersionFromSkillFile(file.Contents); version != "" {
				return version
			}
			break
		}
	}
	return "unknown"
}

func CacheDir() (string, error) {
	appDir, err := configpkg.AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "cache", "skills"), nil
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

func sourceTypeFromSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://github.com/") || regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(trimmed) {
		return "github"
	}
	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		return "url"
	}
	return "unknown"
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

func SkillVersionFromSkillFile(content string) string {
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
