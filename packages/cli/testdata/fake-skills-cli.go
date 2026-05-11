package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type state struct {
	Installed map[string]installedSkill `yaml:"installed"`
}

type installedSkill struct {
	Name          string `yaml:"name"`
	Source        string `yaml:"source"`
	Path          string `yaml:"path"`
	RepoSkillPath string `yaml:"repoSkillPath"`
	Hash          string `yaml:"hash"`
	Version       string `yaml:"version,omitempty"`
	Date          string `yaml:"date,omitempty"`
}

type listedSkill struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Scope  string   `json:"scope"`
	Agents []string `json:"agents"`
}

type lockFile struct {
	Skills map[string]lockEntry `json:"skills"`
}

type lockEntry struct {
	Source       string `json:"source"`
	SourceType   string `json:"sourceType"`
	SkillPath    string `json:"skillPath"`
	ComputedHash string `json:"computedHash"`
}

type metadataFile struct {
	Version string `json:"version,omitempty"`
	Date    string `json:"date,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		exitf("usage: skills <command>")
	}
	st, statePath := loadState()
	var err error
	switch os.Args[1] {
	case "add":
		err = runAdd(st)
	case "list":
		err = runList(st)
	case "find":
		err = runFind()
	default:
		err = fmt.Errorf("unsupported fake skills command %q", os.Args[1])
	}
	if err != nil {
		exitf(err.Error())
	}
	if err := saveState(statePath, st); err != nil {
		exitf(err.Error())
	}
}

func runAdd(st *state) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skills add <source>")
	}
	source := normalizeSourceArg(os.Args[2])
	if hasFlag("-l") || hasFlag("--list") {
		return listRepoSkills(source)
	}
	skillNames := flagValues("--skill")
	if len(skillNames) == 0 {
		return nil
	}
	for _, name := range skillNames {
		if err := installSkill(st, source, strings.TrimSpace(name)); err != nil {
			return err
		}
		fmt.Printf("Installed %s from %s\n", strings.TrimSpace(name), source)
	}
	return nil
}

func runList(st *state) error {
	entries := make([]listedSkill, 0, len(st.Installed))
	names := make([]string, 0, len(st.Installed))
	for name := range st.Installed {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := st.Installed[name]
		entries = append(entries, listedSkill{Name: entry.Name, Path: entry.Path, Scope: "global", Agents: []string{"universal"}})
	}
	output, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func runFind() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: skills find <query>")
	}
	query := strings.TrimSpace(os.Args[2])
	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}
	base := strings.TrimSpace(os.Getenv("SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES"))
	if base == "" {
		return fmt.Errorf("SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES is required")
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	type result struct {
		Source string
		Name   string
	}
	results := make([]result, 0)
	needle := strings.ToLower(query)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sourceRoot := filepath.Join(base, entry.Name())
		lock, err := loadLock(filepath.Join(sourceRoot, "skills-lock.json"))
		if err != nil {
			continue
		}
		for name, item := range lock.Skills {
			haystacks := []string{strings.ToLower(name), strings.ToLower(filepath.Base(strings.TrimSuffix(filepath.Dir(item.SkillPath), "/")))}
			matched := false
			for _, haystack := range haystacks {
				if strings.Contains(haystack, needle) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			results = append(results, result{Source: item.Source, Name: name})
		}
	}
	if len(results) == 0 {
		fmt.Printf("No skills found for %q\n", query)
		return nil
	}
	sort.Slice(results, func(i, j int) bool {
		left := results[i].Source + "@" + results[i].Name
		right := results[j].Source + "@" + results[j].Name
		return left < right
	})
	for _, item := range results {
		fmt.Printf("%s@%s 1.0K installs\n", item.Source, item.Name)
		fmt.Printf("└ https://skills.sh/%s/%s\n\n", item.Source, item.Name)
	}
	return nil
}

func listRepoSkills(source string) error {
	root := fixtureRoot(source)
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return fmt.Errorf("read fixture skills: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fmt.Printf("|    %s\n", entry.Name())
	}
	return nil
}

func installSkill(st *state, source string, name string) error {
	if name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}
	root := fixtureRoot(source)
	skillRoot := filepath.Join(root, "skills", name)
	if _, err := os.Stat(filepath.Join(skillRoot, "SKILL.md")); err != nil {
		return fmt.Errorf("fixture skill not found: %s/%s", source, name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installRoot := filepath.Join(home, ".agents", "skills", name)
	if err := os.RemoveAll(installRoot); err != nil {
		return err
	}
	if err := copyDir(skillRoot, installRoot); err != nil {
		return err
	}
	lock, _ := loadLock(filepath.Join(workDir(), "skills-lock.json"))
	fixtureLock, err := loadLock(filepath.Join(root, "skills-lock.json"))
	if err != nil {
		return err
	}
	entry, ok := fixtureLock.Skills[name]
	if !ok {
		return fmt.Errorf("fixture lock entry not found for %s", name)
	}
	if lock.Skills == nil {
		lock.Skills = map[string]lockEntry{}
	}
	lock.Skills[name] = entry
	if err := saveLock(filepath.Join(workDir(), "skills-lock.json"), lock); err != nil {
		return err
	}
	metadata, _ := loadMetadata(filepath.Join(skillRoot, "metadata.json"))
	st.Installed[name] = installedSkill{
		Name:          name,
		Source:        source,
		Path:          installRoot,
		RepoSkillPath: strings.TrimSuffix(filepath.ToSlash(filepath.Dir(entry.SkillPath)), "/"),
		Hash:          entry.ComputedHash,
		Version:       metadata.Version,
		Date:          metadata.Date,
	}
	return nil
}

func workDir() string {
	if value := strings.TrimSpace(os.Getenv("SKILL_ORGANIZER_FAKE_SKILLS_WORKDIR")); value != "" {
		return value
	}
	wd, err := os.Getwd()
	if err == nil {
		return wd
	}
	return "."
}

func loadState() (*state, string) {
	path := strings.TrimSpace(os.Getenv("SKILL_ORGANIZER_FAKE_SKILLS_STATE"))
	if path == "" {
		exitf("SKILL_ORGANIZER_FAKE_SKILLS_STATE is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &state{Installed: map[string]installedSkill{}}, path
		}
		exitf(err.Error())
	}
	var st state
	if err := yaml.Unmarshal(content, &st); err != nil {
		exitf(err.Error())
	}
	if st.Installed == nil {
		st.Installed = map[string]installedSkill{}
	}
	return &st, path
}

func saveState(path string, st *state) error {
	content, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func loadLock(path string) (lockFile, error) {
	var lock lockFile
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			lock.Skills = map[string]lockEntry{}
			return lock, nil
		}
		return lock, err
	}
	err = json.Unmarshal(content, &lock)
	if lock.Skills == nil {
		lock.Skills = map[string]lockEntry{}
	}
	return lock, err
}

func saveLock(path string, lock lockFile) error {
	content, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o644)
}

func loadMetadata(path string) (metadataFile, error) {
	var metadata metadataFile
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return metadata, nil
		}
		return metadata, err
	}
	err = json.Unmarshal(content, &metadata)
	return metadata, err
}

func fixtureRoot(source string) string {
	base := strings.TrimSpace(os.Getenv("SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES"))
	public := strings.TrimSpace(os.Getenv("SKILL_ORGANIZER_FAKE_PUBLIC_REPOS"))
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		if public == "" {
			exitf("SKILL_ORGANIZER_FAKE_PUBLIC_REPOS is required")
		}
		return filepath.Join(public, sanitizeSource(normalizeSourceArg(source)))
	}
	if base == "" {
		exitf("SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES is required")
	}
	return filepath.Join(base, sanitizeSource(source))
}

func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args[3:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func flagValues(flag string) []string {
	values := []string{}
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		if args[i] != flag || i+1 >= len(args) {
			continue
		}
		values = append(values, args[i+1])
		i++
	}
	return values
}

func normalizeSourceArg(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	return strings.Trim(trimmed, "/")
}

func sanitizeSource(source string) string {
	replacer := strings.NewReplacer("/", "__", ":", "__", ".", "_", "-", "-")
	return replacer.Replace(strings.TrimSpace(source))
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
