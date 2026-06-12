package telemetry

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestIdentityIs32HexChars(t *testing.T) {
	// 32 known bytes -> 64 hex chars... wait, 16 bytes -> 32 hex
	// chars. The test asserts 32 hex chars (the production path is
	// 16 random bytes from crypto/rand, hex-encoded).
	input := []byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}
	want := hex.EncodeToString(input) // 32 hex chars
	if len(want) != 32 {
		t.Fatalf("setup: hex encoding length = %d, want 32", len(want))
	}

	first, err := generateID(bytes.NewReader(append([]byte{}, input...)))
	if err != nil {
		t.Fatalf("first generateID() = %v", err)
	}
	second, err := generateID(bytes.NewReader(append([]byte{}, input...)))
	if err != nil {
		t.Fatalf("second generateID() = %v", err)
	}

	if first != want {
		t.Fatalf("first generateID() = %q, want %q", first, want)
	}
	if second != want {
		t.Fatalf("second generateID() = %q, want %q", second, want)
	}
	if first != second {
		t.Fatalf("generateID is not deterministic on the same input: first=%q second=%q", first, second)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if !pattern.MatchString(first) {
		t.Fatalf("generateID() = %q, does not match 32-hex regex", first)
	}
	if len(first) != 32 {
		t.Fatalf("generateID() len = %d, want 32", len(first))
	}
}

func TestIdentityLoadOrCreateCreatesIfMissing(t *testing.T) {
	appDir := t.TempDir()

	id, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("LoadOrCreate() = %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if !pattern.MatchString(id.InstallID) {
		t.Fatalf("LoadOrCreate().InstallID = %q, does not match 32-hex regex", id.InstallID)
	}
	if !pattern.MatchString(id.HostID) {
		t.Fatalf("LoadOrCreate().HostID = %q, does not match 32-hex regex", id.HostID)
	}

	installPath := filepath.Join(appDir, installIDFile)
	hostPath := filepath.Join(appDir, hostIDFile)
	if _, err := os.Stat(installPath); err != nil {
		t.Fatalf("install_id file missing: %v", err)
	}
	if _, err := os.Stat(hostPath); err != nil {
		t.Fatalf("host_id file missing: %v", err)
	}

	installBytes, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read install_id = %v", err)
	}
	if strings.TrimSpace(string(installBytes)) != id.InstallID {
		t.Fatalf("install_id file content = %q, want %q", string(installBytes), id.InstallID)
	}
	hostBytes, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("read host_id = %v", err)
	}
	if strings.TrimSpace(string(hostBytes)) != id.HostID {
		t.Fatalf("host_id file content = %q, want %q", string(hostBytes), id.HostID)
	}
}

func TestIdentityLoadOrCreateReusesIfPresent(t *testing.T) {
	appDir := t.TempDir()

	first, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("first LoadOrCreate() = %v", err)
	}
	second, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("second LoadOrCreate() = %v", err)
	}

	if first.InstallID != second.InstallID {
		t.Fatalf("install_id changed across calls: first=%q second=%q", first.InstallID, second.InstallID)
	}
	if first.HostID != second.HostID {
		t.Fatalf("host_id changed across calls: first=%q second=%q", first.HostID, second.HostID)
	}
}

func TestRotateHostIDChangesHostIDOnly(t *testing.T) {
	appDir := t.TempDir()

	original, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("LoadOrCreate() = %v", err)
	}

	newHost, err := RotateHostID(appDir)
	if err != nil {
		t.Fatalf("RotateHostID() = %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if !pattern.MatchString(newHost) {
		t.Fatalf("RotateHostID() = %q, does not match 32-hex regex", newHost)
	}
	if newHost == original.HostID {
		t.Fatalf("RotateHostID() = %q, want different from %q", newHost, original.HostID)
	}

	after, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("second LoadOrCreate() = %v", err)
	}
	if after.InstallID != original.InstallID {
		t.Fatalf("install_id changed across rotation: before=%q after=%q", original.InstallID, after.InstallID)
	}
	if after.HostID != newHost {
		t.Fatalf("host_id after rotation = %q, want %q", after.HostID, newHost)
	}
}

func TestLoadIDFileRegeneratesCorrupted(t *testing.T) {
	appDir := t.TempDir()
	installPath := filepath.Join(appDir, installIDFile)
	if err := os.WriteFile(installPath, []byte("not-hex-garbage"), 0o644); err != nil {
		t.Fatalf("seed install_id = %v", err)
	}

	id, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("LoadOrCreate() = %v", err)
	}
	if id.InstallID == "not-hex-garbage" {
		t.Fatalf("LoadOrCreate().InstallID returned the corrupted value")
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if !pattern.MatchString(id.InstallID) {
		t.Fatalf("regenerated InstallID = %q, does not match 32-hex regex", id.InstallID)
	}

	// File on disk should now contain the regenerated value.
	contents, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read install_id = %v", err)
	}
	if strings.TrimSpace(string(contents)) != id.InstallID {
		t.Fatalf("install_id on disk = %q, want %q", string(contents), id.InstallID)
	}
}

func TestLoadOrCreateCreatesAppDir(t *testing.T) {
	nested := filepath.Join(t.TempDir(), "nested", "config")
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Fatalf("setup: nested dir already exists")
	}

	if _, err := LoadOrCreate(nested); err != nil {
		t.Fatalf("LoadOrCreate() on missing dir = %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("app dir was not created: %v", err)
	}
}

func TestRotateHostIDRegeneratesOnCall(t *testing.T) {
	appDir := t.TempDir()

	first, err := RotateHostID(appDir)
	if err != nil {
		t.Fatalf("first RotateHostID() = %v", err)
	}
	second, err := RotateHostID(appDir)
	if err != nil {
		t.Fatalf("second RotateHostID() = %v", err)
	}
	if first == second {
		t.Fatalf("RotateHostID() returned same value twice: %q", first)
	}

	loaded, err := LoadOrCreate(appDir)
	if err != nil {
		t.Fatalf("LoadOrCreate() = %v", err)
	}
	if loaded.HostID != second {
		t.Fatalf("LoadOrCreate().HostID = %q, want %q (the most recent rotation)", loaded.HostID, second)
	}
}
