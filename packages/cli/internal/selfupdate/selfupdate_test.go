package selfupdate

import (
	"os"
	"testing"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestUpdateCommandForInstallMethod(t *testing.T) {
	tests := []struct {
		name   string
		method InstallMethod
		want   string
	}{
		{name: "npm", method: InstallMethodNPM, want: "npm i -g skill-organizer@latest"},
		{name: "homebrew", method: InstallMethodHomebrew, want: "brew upgrade skill-organizer"},
		{name: "direct", method: InstallMethodDirect, want: "download the latest release from GitHub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateCommandFor(tt.method); got != tt.want {
				t.Fatalf("updateCommandFor(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestShouldSkipPeriodicCheckForSelfUpdateCommand(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"skill-organizer", "self-update"}
	t.Cleanup(func() {
		os.Args = originalArgs
	})

	if !shouldSkipPeriodicCheck() {
		t.Fatalf("shouldSkipPeriodicCheck() = false, want true")
	}
}

func TestCachedVersionInfoReturnsCachedUpdateWhenFresh(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	state := configpkg.UpdateCache{
		LastCheckedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
		LatestVersion: "0.0.6",
		LatestPageURL: "https://example.com/releases/latest",
	}

	info, ok := cachedVersionInfo(state, "0.0.5", InstallMethodNPM, now)
	if !ok {
		t.Fatalf("cachedVersionInfo() ok = false, want true")
	}
	if !info.UpdateAvailable {
		t.Fatalf("cachedVersionInfo() UpdateAvailable = false, want true")
	}
	if info.UpdateCommand != "npm i -g skill-organizer@latest" {
		t.Fatalf("cachedVersionInfo() UpdateCommand = %q", info.UpdateCommand)
	}
	if info.LatestVersion != "0.0.6" {
		t.Fatalf("cachedVersionInfo() LatestVersion = %q", info.LatestVersion)
	}
}

func TestCachedVersionInfoExpiresAfterTwelveHours(t *testing.T) {
	now := time.Date(2026, time.May, 9, 12, 0, 0, 0, time.UTC)
	state := configpkg.UpdateCache{
		LastCheckedAt: now.Add(-13 * time.Hour).Format(time.RFC3339),
		LatestVersion: "0.0.6",
	}

	if _, ok := cachedVersionInfo(state, "0.0.5", InstallMethodDirect, now); ok {
		t.Fatalf("cachedVersionInfo() ok = true, want false")
	}
}

func TestIsRemoteNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "patch upgrade", current: "0.0.5", latest: "0.0.6", want: true},
		{name: "same version", current: "0.0.5", latest: "0.0.5", want: false},
		{name: "stable beats prerelease", current: "0.1.0-beta.1", latest: "0.1.0", want: true},
		{name: "older latest", current: "0.1.0", latest: "0.0.9", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRemoteNewer(tt.current, tt.latest); got != tt.want {
				t.Fatalf("isRemoteNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}
