package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	got, err := ResolvePath("~/skills")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	want := filepath.Join(home, "skills")
	if got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestResolvePathExpandsEnvironmentVariables(t *testing.T) {
	if err := os.Setenv("SKILL_ORG_TEST_PATH", "/tmp/skill-organizer-test"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("SKILL_ORG_TEST_PATH")
	})

	got, err := ResolvePath("$SKILL_ORG_TEST_PATH/config.yml")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	want := filepath.Clean("/tmp/skill-organizer-test/config.yml")
	if got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestResolvePathResolvesRelativePaths(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	got, err := ResolvePath("relative/path")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	want := filepath.Join(wd, "relative", "path")
	if got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}
