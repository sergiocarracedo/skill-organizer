package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func BackupRoot(sourceRoot string) string {
	return filepath.Join(sourceRoot, ".old")
}

func MoveToBackup(sourceRoot string, skill skills.Skill, now time.Time) (string, error) {
	stamp := now.UTC().Format("2006-01-02T15-04-05Z")
	destination := filepath.Join(BackupRoot(sourceRoot), stamp, filepath.FromSlash(skill.RelativePath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Rename(skill.Dir, destination); err != nil {
		return "", fmt.Errorf("move skill to backup: %w", err)
	}
	return destination, nil
}

func RemoveSkill(skill skills.Skill) error {
	if err := os.RemoveAll(skill.Dir); err != nil {
		return fmt.Errorf("remove skill directory: %w", err)
	}
	return nil
}

func GarbageCollectBackups(sourceRoot string, retentionDays int, now time.Time) error {
	if retentionDays <= 0 {
		retentionDays = configpkg.DefaultBackupRetentionDays
	}

	entries, err := os.ReadDir(BackupRoot(sourceRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup directory: %w", err)
	}

	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stamp, err := time.Parse("2006-01-02T15-04-05Z", entry.Name())
		if err != nil {
			continue
		}
		if stamp.After(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(BackupRoot(sourceRoot), entry.Name())); err != nil {
			return fmt.Errorf("remove expired backup %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func InstalledRemoteSkills(sourceRoot string) ([]skills.Skill, error) {
	allSkills, err := skills.ScanSource(sourceRoot)
	if err != nil {
		return nil, err
	}

	filtered := make([]skills.Skill, 0, len(allSkills))
	for _, skill := range allSkills {
		if strings.HasPrefix(filepath.ToSlash(skill.RelativePath), ".old/") {
			continue
		}
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return nil, err
		}
		if doc.RemoteMetadata().Provider == "" {
			continue
		}
		filtered = append(filtered, skill)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].RelativePath < filtered[j].RelativePath
	})
	return filtered, nil
}
