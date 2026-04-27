package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

type Result struct {
	SourceSkills []skills.Skill
	Enabled      []skills.Skill
	Disabled     []skills.Skill
	Created      []string
	Updated      []string
	Removed      []string
	Manifest     Manifest
}

func Run(location configpkg.Location) (Result, error) {
	if err := location.Validate(); err != nil {
		return Result{}, err
	}

	allSkills, err := skills.ScanSource(location.Source)
	if err != nil {
		return Result{}, err
	}

	desired := make(map[string]skills.Skill, len(allSkills))
	disabled := make([]skills.Skill, 0)
	enabled := make([]skills.Skill, 0)

	for _, skill := range allSkills {
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return Result{}, fmt.Errorf("load source skill %q: %w", skill.SkillFile, err)
		}

		metadata := doc.ManagedMetadata()
		if metadata.OriginalName == "" {
			metadata.OriginalName = doc.Name()
		}

		if err := skills.RewriteManagedFields(skill, true, metadata.Disabled); err != nil {
			return Result{}, err
		}

		if metadata.Disabled {
			disabled = append(disabled, skill)
			continue
		}

		enabled = append(enabled, skill)
		desired[skill.FlattenedName] = skill
	}

	if err := os.MkdirAll(location.Target, 0o755); err != nil {
		return Result{}, fmt.Errorf("create target directory: %w", err)
	}

	manifest, err := LoadManifestOrEmpty(location.Target)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		SourceSkills: allSkills,
		Enabled:      enabled,
		Disabled:     disabled,
		Manifest:     Manifest{Managed: map[string]string{}},
	}

	managedNames := make([]string, 0, len(desired))
	for name := range desired {
		managedNames = append(managedNames, name)
	}
	sort.Strings(managedNames)

	for _, name := range managedNames {
		skill := desired[name]
		changed, action, err := reconcileTargetEntry(location.Target, skill)
		if err != nil {
			return Result{}, err
		}
		if changed {
			switch action {
			case "created":
				result.Created = append(result.Created, name)
			case "updated":
				result.Updated = append(result.Updated, name)
			}
		}

		result.Manifest.Managed[name] = skill.RelativePath
	}

	staleNames := staleManagedEntries(manifest, desired)
	for _, name := range staleNames {
		path := filepath.Join(location.Target, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("remove stale managed target %q: %w", path, err)
		}
		result.Removed = append(result.Removed, name)
	}

	if err := SaveManifest(location.Target, result.Manifest); err != nil {
		return Result{}, err
	}

	return result, nil
}

func reconcileTargetEntry(target string, skill skills.Skill) (bool, string, error) {
	linkPath := filepath.Join(target, skill.FlattenedName)
	relTarget, err := filepath.Rel(target, skill.Dir)
	if err != nil {
		return false, "", fmt.Errorf("compute symlink target for %q: %w", skill.Dir, err)
	}

	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return false, "", fmt.Errorf("target entry %q exists and is not a managed symlink", linkPath)
		}

		current, err := os.Readlink(linkPath)
		if err != nil {
			return false, "", fmt.Errorf("read symlink %q: %w", linkPath, err)
		}

		if current == relTarget {
			return false, "", nil
		}

		if err := os.Remove(linkPath); err != nil {
			return false, "", fmt.Errorf("remove outdated symlink %q: %w", linkPath, err)
		}
		if err := os.Symlink(relTarget, linkPath); err != nil {
			return false, "", fmt.Errorf("update symlink %q: %w", linkPath, err)
		}
		return true, "updated", nil
	} else if !os.IsNotExist(err) {
		return false, "", fmt.Errorf("stat target entry %q: %w", linkPath, err)
	}

	if err := os.Symlink(relTarget, linkPath); err != nil {
		return false, "", fmt.Errorf("create symlink %q: %w", linkPath, err)
	}

	return true, "created", nil
}

func staleManagedEntries(manifest Manifest, desired map[string]skills.Skill) []string {
	stale := make([]string, 0)
	for name := range manifest.Managed {
		if _, ok := desired[name]; ok {
			continue
		}
		stale = append(stale, name)
	}
	sort.Strings(stale)
	return stale
}
