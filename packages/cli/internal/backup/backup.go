package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"gopkg.in/yaml.v3"
)

type Metadata struct {
	OriginalPath  string `yaml:"original-path"`
	FlattenedName string `yaml:"flattened-name"`
	DeletedAt     string `yaml:"deleted-at,omitempty"`
	UpdatedFrom   string `yaml:"updated-from,omitempty"`
	UpdatedTo     string `yaml:"updated-to,omitempty"`
}

func Root() (string, error) {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(registryPath), ".old"), nil
}

func MoveSkill(sourcePath string, flattenedName string, metadata Metadata, now time.Time) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}

	stamp := now.UTC().Format("20060102T150405Z")
	destination := filepath.Join(root, stamp+"-"+flattenedName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create backup root: %w", err)
	}
	if _, err := os.Stat(destination); err == nil {
		return "", fmt.Errorf("backup destination already exists: %s", destination)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat backup destination: %w", err)
	}

	if err := os.Rename(sourcePath, destination); err != nil {
		return "", fmt.Errorf("move skill to backup: %w", err)
	}

	metadata.OriginalPath = strings.TrimSpace(metadata.OriginalPath)
	metadata.FlattenedName = strings.TrimSpace(metadata.FlattenedName)
	if metadata.DeletedAt == "" && metadata.UpdatedFrom == "" && metadata.UpdatedTo == "" {
		metadata.DeletedAt = now.UTC().Format(time.RFC3339)
	}
	if err := writeMetadata(destination, metadata); err != nil {
		return "", err
	}

	return destination, nil
}

func PruneExpired(root string, retentionDays int, now time.Time) error {
	if retentionDays <= 0 {
		retentionDays = configpkg.DefaultBackupRetentionDays
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup root: %w", err)
	}

	cutoff := now.UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stamp, ok := parseEntryTimestamp(entry.Name())
		if !ok || !stamp.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return fmt.Errorf("remove expired backup %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func List(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup root: %w", err)
	}
	results := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(results)
	return results, nil
}

func writeMetadata(destination string, metadata Metadata) error {
	content, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal backup metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destination, ".skill-organizer-backup.yml"), content, 0o644); err != nil {
		return fmt.Errorf("write backup metadata: %w", err)
	}
	return nil
}

func parseEntryTimestamp(name string) (time.Time, bool) {
	parts := strings.SplitN(strings.TrimSpace(name), "-", 2)
	if len(parts) != 2 {
		return time.Time{}, false
	}
	parsed, err := time.Parse("20060102T150405Z", parts[0])
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
