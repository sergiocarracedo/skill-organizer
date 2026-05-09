package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseEntryTimestamp(t *testing.T) {
	parsed, ok := parseEntryTimestamp("20260509T120000Z-skill-name")
	if !ok {
		t.Fatalf("parseEntryTimestamp() ok = false, want true")
	}
	if parsed.Format(time.RFC3339) != "2026-05-09T12:00:00Z" {
		t.Fatalf("parseEntryTimestamp() = %s", parsed.Format(time.RFC3339))
	}
}

func TestPruneExpiredRemovesOldDirectories(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "20260501T120000Z-old-skill")
	newDir := filepath.Join(root, "20260515T120000Z-new-skill")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	now := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)
	if err := PruneExpired(root, 10, now); err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old backup should be removed")
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("new backup should remain: %v", err)
	}
}

func TestWriteMetadataCreatesMetadataFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "20260509T120000Z-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := writeMetadata(dir, Metadata{OriginalPath: "thirdparty/example", FlattenedName: "thirdparty--example"}); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, ".skill-organizer-backup.yml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "original-path: thirdparty/example") {
		t.Fatalf("metadata content = %q", string(content))
	}
}
