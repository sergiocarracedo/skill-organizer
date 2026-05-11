package maintenance

import (
	"context"
	"strings"
	"testing"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

func TestMaybeNotifySkillUpdatesStylesCommandHintWhenUpdatesExist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		t.Fatalf("UpdatesPath() error = %v", err)
	}
	if err := configpkg.SaveUpdatesState(updatesPath, configpkg.UpdatesState{UpdateCount: 2}); err != nil {
		t.Fatalf("SaveUpdatesState() error = %v", err)
	}

	var output strings.Builder
	MaybeNotifySkillUpdates(context.Background(), &output)

	rendered := output.String()
	if !strings.Contains(rendered, "There are skill 2 updates.") {
		t.Fatalf("output missing update count message: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("output missing ANSI color sequence: %q", rendered)
	}
	if !strings.Contains(rendered, "skill-organizer check-updates") {
		t.Fatalf("output missing command hint: %q", rendered)
	}
	if strings.Contains(rendered, "`skill-organizer check-updates`") {
		t.Fatalf("output still contains unstyled backticked command: %q", rendered)
	}
}

func TestMaybeNotifySkillUpdatesStylesCommandHintForReminder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	originalIsServiceRunning := IsServiceRunningFunc
	IsServiceRunningFunc = func() (bool, error) { return false, nil }
	t.Cleanup(func() {
		IsServiceRunningFunc = originalIsServiceRunning
	})

	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		t.Fatalf("UpdatesPath() error = %v", err)
	}
	checkedAt := time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	if err := configpkg.SaveUpdatesState(updatesPath, configpkg.UpdatesState{LastCheckedAt: checkedAt}); err != nil {
		t.Fatalf("SaveUpdatesState() error = %v", err)
	}

	var output strings.Builder
	MaybeNotifySkillUpdates(context.Background(), &output)

	rendered := output.String()
	if !strings.Contains(rendered, "Skill updates have not been checked in 30 days.") {
		t.Fatalf("output missing reminder message: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("output missing ANSI color sequence: %q", rendered)
	}
	if !strings.Contains(rendered, "skill-organizer check-updates") {
		t.Fatalf("output missing command hint: %q", rendered)
	}

	state, err := configpkg.LoadUpdatesState(updatesPath)
	if err != nil {
		t.Fatalf("LoadUpdatesState() error = %v", err)
	}
	if strings.TrimSpace(state.LastRemindedAt) == "" {
		t.Fatalf("expected LastRemindedAt to be recorded")
	}
}
