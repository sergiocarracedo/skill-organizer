package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const appDirName = "skill-organizer"

func RegistryPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(base, appDirName, "skill-organizer.yml"), nil
}

func LoadAppConfig(path string) (AppConfig, error) {
	var cfg AppConfig

	content, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read app config: %w", err)
	}

	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return cfg, fmt.Errorf("parse app config: %w", err)
	}
	cfg.Normalize()

	return cfg, nil
}

func LoadAppConfigOrDefault(path string) (AppConfig, error) {
	cfg, err := LoadAppConfig(path)
	if errors.Is(err, os.ErrNotExist) {
		defaultCfg := AppConfig{}
		defaultCfg.Normalize()
		return defaultCfg, nil
	}

	return cfg, err
}

func SaveAppConfig(path string, cfg AppConfig) error {
	cfg.Normalize()

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal app config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create app config directory: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write app config: %w", err)
	}

	return nil
}

func LoadRegistry(path string) (WatchRegistry, error) {
	cfg, err := LoadAppConfig(path)
	if err != nil {
		return WatchRegistry{}, err
	}
	return cfg.WatchRegistry(), nil
}

func LoadRegistryOrEmpty(path string) (WatchRegistry, error) {
	cfg, err := LoadAppConfigOrDefault(path)
	if err != nil {
		return WatchRegistry{}, err
	}
	return cfg.WatchRegistry(), nil
}

func SaveRegistry(path string, registry WatchRegistry) error {
	cfg, err := LoadAppConfigOrDefault(path)
	if err != nil {
		return err
	}
	cfg.SetWatchRegistry(registry)
	return SaveAppConfig(path, cfg)
}

func LoadServiceConfig(path string) (ServiceConfig, error) {
	cfg, err := LoadAppConfig(path)
	if err != nil {
		return ServiceConfig{}, err
	}
	return cfg.Service, nil
}

func LoadServiceConfigOrDefault(path string) (ServiceConfig, error) {
	cfg, err := LoadAppConfigOrDefault(path)
	if err != nil {
		return ServiceConfig{}, err
	}
	return cfg.Service, nil
}

func SaveServiceConfig(path string, service ServiceConfig) error {
	cfg, err := LoadAppConfigOrDefault(path)
	if err != nil {
		return err
	}
	cfg.Service = service
	return SaveAppConfig(path, cfg)
}

func (r *WatchRegistry) Add(path string) {
	r.Watched = append(r.Watched, path)
	r.Normalize()
}

func (r *WatchRegistry) Remove(path string) bool {
	normalized := filepath.Clean(path)
	filtered := make([]string, 0, len(r.Watched))
	removed := false

	for _, watched := range r.Watched {
		if filepath.Clean(watched) == normalized {
			removed = true
			continue
		}
		filtered = append(filtered, watched)
	}

	r.Watched = filtered
	r.Normalize()
	return removed
}
