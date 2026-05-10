package maintenance

import (
	"context"
	"fmt"
	"io"
	"time"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

const checkInterval = 12 * time.Hour
const skillUpdateReminderInterval = 30 * 24 * time.Hour

var IsServiceRunningFunc = func() (bool, error) { return false, nil }

func MaybeNotifySkillUpdates(_ context.Context, stdout io.Writer) {
	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		return
	}
	state, err := configpkg.LoadUpdatesStateOrDefault(updatesPath)
	if err != nil {
		return
	}
	if state.UpdateCount > 0 {
		_, _ = fmt.Fprintf(stdout, "\nThere are %d updates. Run skill-organizer check-updates to update skills.\n\n", state.UpdateCount)
		return
	}
	running, err := IsServiceRunningFunc()
	if err == nil && running {
		return
	}
	now := time.Now().UTC()
	checkedAt, checkedOK := parseOptionalRFC3339(state.LastCheckedAt)
	if checkedOK && now.Sub(checkedAt) < skillUpdateReminderInterval {
		return
	}
	remindedAt, remindedOK := parseOptionalRFC3339(state.LastRemindedAt)
	if remindedOK && now.Sub(remindedAt) < skillUpdateReminderInterval {
		return
	}
	_, _ = fmt.Fprintf(stdout, "\nSkill updates have not been checked in 30 days. Run skill-organizer check-updates to review updates.\n\n")
	state.LastRemindedAt = now.Format(time.RFC3339)
	_ = configpkg.SaveUpdatesState(updatesPath, state)
}

func MaybeRunBackupGC(_ context.Context) {
	cachePath, err := configpkg.CachePath()
	if err != nil {
		return
	}
	cache, err := configpkg.LoadUpdateCacheOrDefault(cachePath)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	if checkedRecently(cache.BackupGC.LastCheckedAt, now) {
		return
	}
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return
	}
	backupCfg, err := configpkg.LoadBackupConfigOrDefault(registryPath)
	if err != nil {
		return
	}
	root, err := backuppkg.Root()
	if err != nil {
		return
	}
	if err := backuppkg.PruneExpired(root, backupCfg.RetentionDays, now); err != nil {
		return
	}
	cache.BackupGC.LastCheckedAt = now.Format(time.RFC3339)
	_ = configpkg.SaveUpdateCache(cachePath, cache)
}

func checkedRecently(value string, now time.Time) bool {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return false
	}
	return now.Sub(parsed) < checkInterval
}

func parseOptionalRFC3339(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
