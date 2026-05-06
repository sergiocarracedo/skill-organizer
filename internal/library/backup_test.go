package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func TestGarbageCollectBackupsRemovesExpiredEntries(t *testing.T) {
	root := t.TempDir()
	backupRoot := BackupRoot(root)
	oldDir := filepath.Join(backupRoot, "2026-01-01T00-00-00Z")
	newDir := filepath.Join(backupRoot, "2026-01-19T00-00-00Z")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	now := time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC)
	if err := GarbageCollectBackups(root, 10, now); err != nil {
		t.Fatalf("GarbageCollectBackups() error = %v", err)
	}

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old backup still exists")
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("new backup missing: %v", err)
	}
}

func TestMoveToBackupMovesSkillDirectory(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "thirdparty", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	skill := skills.Skill{Dir: skillDir, RelativePath: "thirdparty/demo"}
	destination, err := MoveToBackup(root, skill, time.Date(2026, 1, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MoveToBackup() error = %v", err)
	}

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("original skill directory still exists")
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("backup destination missing: %v", err)
	}
}
