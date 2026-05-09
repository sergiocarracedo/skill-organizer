package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

const SkillFileName = "SKILL.md"

type Skill struct {
	Dir           string
	SkillFile     string
	RelativePath  string
	FlattenedName string
}

func ScanSource(root string) ([]Skill, error) {
	root, err := configpkg.ResolvePath(root)
	if err != nil {
		return nil, fmt.Errorf("resolve source root: %w", err)
	}

	var skills []Skill
	seen := map[string]string{}

	if err := scanDir(root, root, &skills, seen); err != nil {
		return nil, err
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].RelativePath < skills[j].RelativePath
	})

	return skills, nil
}

func scanDir(root, dir string, skills *[]Skill, seen map[string]string) error {
	skillFile := filepath.Join(dir, SkillFileName)
	if info, err := os.Stat(skillFile); err == nil && !info.IsDir() {
		relPath, err := filepath.Rel(root, dir)
		if err != nil {
			return fmt.Errorf("compute relative path for %q: %w", dir, err)
		}

		relPath = filepath.ToSlash(filepath.Clean(relPath))
		flattened := FlattenName(relPath)
		if other, ok := seen[flattened]; ok {
			return fmt.Errorf("flattening collision: %s and %s both map to %s", other, dir, flattened)
		}

		seen[flattened] = dir
		*skills = append(*skills, Skill{
			Dir:           dir,
			SkillFile:     skillFile,
			RelativePath:  relPath,
			FlattenedName: flattened,
		})
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat skill file %q: %w", skillFile, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read source directory %q: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if err := scanDir(root, filepath.Join(dir, entry.Name()), skills, seen); err != nil {
			return err
		}
	}

	return nil
}

func FlattenName(relativePath string) string {
	normalized := filepath.ToSlash(filepath.Clean(relativePath))
	return strings.ReplaceAll(normalized, "/", "--")
}

func ResolveSourceSkill(locationRoot, input string) (Skill, error) {
	locationRoot, err := configpkg.ResolvePath(locationRoot)
	if err != nil {
		return Skill{}, fmt.Errorf("resolve source root: %w", err)
	}

	path := strings.TrimSpace(input)
	if path == "" {
		return Skill{}, fmt.Errorf("source path cannot be empty")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(locationRoot, path)
	}
	path, err = configpkg.ResolvePath(path)
	if err != nil {
		return Skill{}, fmt.Errorf("resolve source path: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return Skill{}, fmt.Errorf("stat source path: %w", err)
	}

	if !info.IsDir() {
		if filepath.Base(path) != SkillFileName {
			return Skill{}, fmt.Errorf("source path must be a skill directory or SKILL.md file")
		}
		path = filepath.Dir(path)
	}

	relPath, err := filepath.Rel(locationRoot, path)
	if err != nil {
		return Skill{}, fmt.Errorf("compute source-relative path: %w", err)
	}

	if relPath == ".." || strings.HasPrefix(filepath.ToSlash(relPath), "../") {
		return Skill{}, fmt.Errorf("source path %q is outside configured source root %q", path, locationRoot)
	}

	skillFile := filepath.Join(path, SkillFileName)
	if _, err := os.Stat(skillFile); err != nil {
		if os.IsNotExist(err) {
			return Skill{}, fmt.Errorf("skill file not found in %q", path)
		}
		return Skill{}, fmt.Errorf("stat skill file: %w", err)
	}

	relPath = filepath.ToSlash(filepath.Clean(relPath))
	return Skill{
		Dir:           path,
		SkillFile:     skillFile,
		RelativePath:  relPath,
		FlattenedName: FlattenName(relPath),
	}, nil
}
