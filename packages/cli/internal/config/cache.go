package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type UpdateCache struct {
	LastCheckedAt string           `yaml:"last-checked-at,omitempty"`
	LatestVersion string           `yaml:"latest-version,omitempty"`
	LatestPageURL string           `yaml:"latest-page-url,omitempty"`
	SkillUpdates  SkillUpdateCache `yaml:"skill-updates,omitempty"`
	BackupGC      TaskCache        `yaml:"backup-gc,omitempty"`
}

type UpdatesState struct {
	LastCheckedAt  string              `yaml:"last-checked-at,omitempty"`
	UpdateCount    int                 `yaml:"update-count,omitempty"`
	LastRemindedAt string              `yaml:"last-reminded-at,omitempty"`
	Pending        []SkillUpdateRecord `yaml:"pending,omitempty"`
}

type TaskCache struct {
	LastCheckedAt string `yaml:"last-checked-at,omitempty"`
}

type SkillUpdateCache struct {
	LastCheckedAt string              `yaml:"last-checked-at,omitempty"`
	Pending       []SkillUpdateRecord `yaml:"pending,omitempty"`
}

type SkillUpdateRecord struct {
	RelativePath     string `yaml:"relative-path,omitempty"`
	FlattenedName    string `yaml:"flattened-name,omitempty"`
	InstalledVersion string `yaml:"installed-version,omitempty"`
	LatestVersion    string `yaml:"latest-version,omitempty"`
	Source           string `yaml:"source,omitempty"`
	RepoSkillPath    string `yaml:"repo-skill-path,omitempty"`
	CheckedAt        string `yaml:"checked-at,omitempty"`
}

func CachePath() (string, error) {
	registryPath, err := RegistryPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(registryPath), ".cache.yml"), nil
}

func UpdatesPath() (string, error) {
	appDir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, ".updates"), nil
}

func (c *UpdateCache) Normalize() {
	c.LastCheckedAt = strings.TrimSpace(c.LastCheckedAt)
	c.LatestVersion = strings.TrimSpace(c.LatestVersion)
	c.LatestPageURL = strings.TrimSpace(c.LatestPageURL)
	c.SkillUpdates.Normalize()
	c.BackupGC.Normalize()
}

func (c *TaskCache) Normalize() {
	c.LastCheckedAt = strings.TrimSpace(c.LastCheckedAt)
}

func (c *SkillUpdateCache) Normalize() {
	c.LastCheckedAt = strings.TrimSpace(c.LastCheckedAt)
	cleaned := make([]SkillUpdateRecord, 0, len(c.Pending))
	for _, pending := range c.Pending {
		pending.Normalize()
		if pending.RelativePath == "" && pending.FlattenedName == "" {
			continue
		}
		cleaned = append(cleaned, pending)
	}
	c.Pending = cleaned
}

func (s *UpdatesState) Normalize() {
	s.LastCheckedAt = strings.TrimSpace(s.LastCheckedAt)
	s.LastRemindedAt = strings.TrimSpace(s.LastRemindedAt)
	if s.UpdateCount < 0 {
		s.UpdateCount = 0
	}
	cleaned := make([]SkillUpdateRecord, 0, len(s.Pending))
	for _, pending := range s.Pending {
		pending.Normalize()
		if pending.RelativePath == "" && pending.FlattenedName == "" {
			continue
		}
		cleaned = append(cleaned, pending)
	}
	s.Pending = cleaned
}

func (r *SkillUpdateRecord) Normalize() {
	r.RelativePath = strings.TrimSpace(r.RelativePath)
	r.FlattenedName = strings.TrimSpace(r.FlattenedName)
	r.InstalledVersion = strings.TrimSpace(r.InstalledVersion)
	r.LatestVersion = strings.TrimSpace(r.LatestVersion)
	r.Source = strings.TrimSpace(r.Source)
	r.RepoSkillPath = strings.TrimSpace(r.RepoSkillPath)
	r.CheckedAt = strings.TrimSpace(r.CheckedAt)
}

func LoadUpdateCache(path string) (UpdateCache, error) {
	var cache UpdateCache

	content, err := os.ReadFile(path)
	if err != nil {
		return cache, fmt.Errorf("read update cache: %w", err)
	}

	if err := yaml.Unmarshal(content, &cache); err != nil {
		return cache, fmt.Errorf("parse update cache: %w", err)
	}
	cache.Normalize()
	return cache, nil
}

func LoadUpdateCacheOrDefault(path string) (UpdateCache, error) {
	cache, err := LoadUpdateCache(path)
	if errors.Is(err, os.ErrNotExist) {
		cache.Normalize()
		return cache, nil
	}
	return cache, err
}

func SaveUpdateCache(path string, cache UpdateCache) error {
	cache.Normalize()

	content, err := yaml.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal update cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}

	return nil
}

func ClearUpdateCache(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove update cache: %w", err)
	}
	return nil
}

func LoadUpdatesState(path string) (UpdatesState, error) {
	var state UpdatesState

	content, err := os.ReadFile(path)
	if err != nil {
		return state, fmt.Errorf("read updates state: %w", err)
	}

	if err := yaml.Unmarshal(content, &state); err != nil {
		return state, fmt.Errorf("parse updates state: %w", err)
	}
	state.Normalize()
	return state, nil
}

func LoadUpdatesStateOrDefault(path string) (UpdatesState, error) {
	state, err := LoadUpdatesState(path)
	if errors.Is(err, os.ErrNotExist) {
		state.Normalize()
		return state, nil
	}
	return state, err
}

func SaveUpdatesState(path string, state UpdatesState) error {
	state.Normalize()

	content, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal updates state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create updates directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write updates state: %w", err)
	}

	return nil
}
