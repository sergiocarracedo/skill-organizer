package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const manifestFileName = ".skill-organizer.manifest.yml"

type Manifest struct {
	Managed map[string]string `yaml:"managed"`
}

func manifestPath(target string) string {
	return filepath.Join(target, manifestFileName)
}

func LoadManifest(target string) (Manifest, error) {
	var manifest Manifest
	content, err := os.ReadFile(manifestPath(target))
	if err != nil {
		return manifest, fmt.Errorf("read manifest: %w", err)
	}

	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return manifest, fmt.Errorf("parse manifest: %w", err)
	}

	manifest.Normalize()
	return manifest, nil
}

func LoadManifestOrEmpty(target string) (Manifest, error) {
	manifest, err := LoadManifest(target)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return Manifest{Managed: map[string]string{}}, nil
	}
	return manifest, err
}

func SaveManifest(target string, manifest Manifest) error {
	manifest.Normalize()
	content, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	if err := os.WriteFile(manifestPath(target), content, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func (m *Manifest) Normalize() {
	if m.Managed == nil {
		m.Managed = map[string]string{}
	}

	cleaned := make(map[string]string, len(m.Managed))
	keys := make([]string, 0, len(m.Managed))
	for key := range m.Managed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if key == "" || m.Managed[key] == "" {
			continue
		}
		cleaned[key] = m.Managed[key]
	}

	m.Managed = cleaned
}
