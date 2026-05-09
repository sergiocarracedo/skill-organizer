package agenttools

import (
	"fmt"
	"testing"
)

func TestDetectInstalled(t *testing.T) {
	original := lookPath
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "opencode", "agcl":
			return "/usr/bin/" + file, nil
		default:
			return "", fmt.Errorf("not found")
		}
	}
	t.Cleanup(func() {
		lookPath = original
	})

	installed, err := DetectInstalled()
	if err != nil {
		t.Fatalf("DetectInstalled() error = %v", err)
	}

	if len(installed) != 3 {
		t.Fatalf("DetectInstalled() len = %d, want 3", len(installed))
	}

	if installed[0].Tool.ID != "antigravity" {
		t.Fatalf("DetectInstalled()[0].Tool.ID = %q, want %q", installed[0].Tool.ID, "antigravity")
	}
	if installed[1].Tool.ID != "claude" {
		t.Fatalf("DetectInstalled()[1].Tool.ID = %q, want %q", installed[1].Tool.ID, "claude")
	}
	if installed[2].Tool.ID != "opencode" {
		t.Fatalf("DetectInstalled()[2].Tool.ID = %q, want %q", installed[2].Tool.ID, "opencode")
	}
	if installed[0].Binary != "agcl" {
		t.Fatalf("DetectInstalled()[0].Binary = %q, want %q", installed[0].Binary, "agcl")
	}
}
