package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

type Cache struct {
	root string
	ttl  time.Duration
}

type cacheEnvelope struct {
	SavedAt time.Time       `json:"savedAt"`
	Data    json.RawMessage `json:"data"`
}

func NewCache(ttlHours int) (*Cache, error) {
	appDir, err := configpkg.AppDir()
	if err != nil {
		return nil, err
	}

	if ttlHours <= 0 {
		ttlHours = configpkg.DefaultCacheTTLHours
	}

	return &Cache{
		root: filepath.Join(appDir, "cache"),
		ttl:  time.Duration(ttlHours) * time.Hour,
	}, nil
}

func (c *Cache) Load(kind string, key string, dest any) (bool, error) {
	path := c.path(kind, key)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read cache %s: %w", path, err)
	}

	var envelope cacheEnvelope
	if err := json.Unmarshal(content, &envelope); err != nil {
		return false, fmt.Errorf("parse cache %s: %w", path, err)
	}

	if time.Since(envelope.SavedAt) > c.ttl {
		return false, nil
	}

	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return false, fmt.Errorf("parse cache payload %s: %w", path, err)
	}

	return true, nil
}

func (c *Cache) Save(kind string, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache payload: %w", err)
	}

	envelope, err := json.Marshal(cacheEnvelope{SavedAt: time.Now().UTC(), Data: payload})
	if err != nil {
		return fmt.Errorf("marshal cache envelope: %w", err)
	}

	path := c.path(kind, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	if err := os.WriteFile(path, envelope, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	return nil
}

func (c *Cache) path(kind string, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(c.root, kind, hex.EncodeToString(sum[:])+".json")
}
