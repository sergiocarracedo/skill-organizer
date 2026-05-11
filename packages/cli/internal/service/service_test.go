package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallUserUnitUsesCurrentExecutablePath(t *testing.T) {
	root := t.TempDir()
	unitPath := filepath.Join(root, "skill-organizer.service")

	originalExecutablePath := serviceExecutablePath
	originalRunUserSystemctl := runUserSystemctl
	serviceExecutablePath = func() (string, error) {
		return "/home/linuxbrew/.linuxbrew/bin/skill-organizer", nil
	}
	runUserSystemctl = func(args ...string) error { return nil }
	t.Cleanup(func() {
		serviceExecutablePath = originalExecutablePath
		runUserSystemctl = originalRunUserSystemctl
	})

	if err := installUserUnit(unitPath); err != nil {
		t.Fatalf("installUserUnit() error = %v", err)
	}

	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), `ExecStart="/home/linuxbrew/.linuxbrew/bin/skill-organizer" watch`) {
		t.Fatalf("unit file does not contain expected ExecStart\n%s", string(content))
	}
}

func TestControlLinuxUserSystemdStartRefreshesUnitBeforeStarting(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "systemd", "user"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	originalHome := os.Getenv("HOME")
	originalExecutablePath := serviceExecutablePath
	originalRunUserSystemctl := runUserSystemctl
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	serviceExecutablePath = func() (string, error) {
		return "/home/linuxbrew/.linuxbrew/bin/skill-organizer", nil
	}
	var calls []string
	runUserSystemctl = func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() {
		_ = os.Setenv("HOME", originalHome)
		serviceExecutablePath = originalExecutablePath
		runUserSystemctl = originalRunUserSystemctl
	})

	status, err := controlLinuxUserSystemd("start")
	if err != nil {
		t.Fatalf("controlLinuxUserSystemd(start) error = %v", err)
	}
	if status != "started" {
		t.Fatalf("status = %q, want %q", status, "started")
	}
	if len(calls) < 3 || calls[0] != "daemon-reload" || calls[1] != "enable skill-organizer.service" || calls[2] != "start skill-organizer.service" {
		t.Fatalf("systemctl calls = %#v", calls)
	}
	content, err := os.ReadFile(filepath.Join(home, ".config", "systemd", "user", "skill-organizer.service"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), `ExecStart="/home/linuxbrew/.linuxbrew/bin/skill-organizer" watch`) {
		t.Fatalf("unit file does not contain refreshed ExecStart\n%s", string(content))
	}
}
