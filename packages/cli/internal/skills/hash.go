package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ComputeSkillHash computes SHA-256 of a skill's content files.
// Reads all files in the skill directory, concatenates (sorted by filename),
// and returns hex-encoded SHA-256. Excludes the SKILL.md frontmatter metadata
// block (only hashes the content body and non-metadata files).
func ComputeSkillHash(skillDir string) (string, error) {
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return "", fmt.Errorf("read skill dir %q: %w", skillDir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	h := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(skillDir, entry.Name())

		if entry.Name() == SkillFileName {
			doc, err := LoadDocument(path)
			if err != nil {
				// If we can't parse the document, fall back to hashing the full file
				data, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("read %q: %w", path, err)
				}
				h.Write(data)
				continue
			}
			h.Write([]byte(doc.Body()))
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("read %q: %w", path, err)
			}
			h.Write(data)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
