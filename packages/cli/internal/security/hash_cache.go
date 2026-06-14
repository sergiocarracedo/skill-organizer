package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const hashCacheFileName = ".skill-hash-cache.json"

type HashCache map[string]CachedSkillResult

type CachedSkillResult struct {
	Hash       string `json:"hash"`
	RiskScore  int    `json:"risk-score"`
	RiskReason string `json:"risk-reason"`
	Model      string `json:"model"`
	CheckedAt  string `json:"checked-at"`
}

type hashCacheFile struct {
	Version int                   `json:"version"`
	Cache   map[string]CachedSkillResult `json:"cache"`
}

var hashCacheLock sync.Mutex

func hashCachePath(sourceDir string) string {
	return filepath.Join(sourceDir, hashCacheFileName)
}

func LoadHashCache(sourceDir string) (HashCache, error) {
	path := hashCachePath(sourceDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(HashCache), nil
		}
		return nil, fmt.Errorf("read hash cache: %w", err)
	}

	var cacheFile hashCacheFile
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		return nil, fmt.Errorf("parse hash cache: %w", err)
	}

	return cacheFile.Cache, nil
}

func SaveHashCache(sourceDir string, cache HashCache) error {
	path := hashCachePath(sourceDir)
	
	hashCacheLock.Lock()
	defer hashCacheLock.Unlock()

	cacheFile := hashCacheFile{
		Version: 2,
		Cache:   cache,
	}

	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hash cache: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write hash cache: %w", err)
	}

	return nil
}

func GetCachedResult(cache HashCache, flattenedName string) (CachedSkillResult, bool) {
	result, ok := cache[flattenedName]
	return result, ok
}

func SetCachedResult(cache HashCache, flattenedName string, result CachedSkillResult) {
	cache[flattenedName] = result
}

func PruneStaleEntries(cache HashCache, currentSkills []string) {
	currentSet := make(map[string]struct{}, len(currentSkills))
	for _, name := range currentSkills {
		currentSet[name] = struct{}{}
	}

	for name := range cache {
		if _, exists := currentSet[name]; !exists {
			delete(cache, name)
		}
	}
}

func SortedKeys(cache HashCache) []string {
	keys := make([]string, 0, len(cache))
	for k := range cache {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}