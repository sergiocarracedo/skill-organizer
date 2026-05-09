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

func MaybeNotifySkillUpdates(_ context.Context, stdout io.Writer) {
	cachePath, err := configpkg.CachePath()
	if err != nil {
		return
	}
	cache, err := configpkg.LoadUpdateCacheOrDefault(cachePath)
	if err != nil {
		return
	}
	if len(cache.SkillUpdates.Pending) == 0 {
		return
	}
	_, _ = fmt.Fprintf(stdout, "\nSkill updates available: %d\n", len(cache.SkillUpdates.Pending))
	_, _ = fmt.Fprintf(stdout, "Run `skill-organizer skill check-updates` to review them.\n\n")
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
