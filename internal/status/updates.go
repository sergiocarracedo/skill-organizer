package status

import (
	"strings"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
)

func AttachUpdates(report *Report) error {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return err
	}
	appConfig, err := configpkg.LoadAppConfigOrDefault(registryPath)
	if err != nil {
		return err
	}

	cache, err := remote.NewCache(appConfig.Updates.CacheTTLHours)
	if err != nil {
		return err
	}
	manager := remote.NewManager(remote.SkillsShProvider{}, remote.GitHubProvider{})

	for i := range report.Skills {
		entry := &report.Skills[i]
		if entry.Remote.Provider == "" {
			continue
		}

		provider, err := manager.Provider(entry.Remote.Provider)
		if err != nil {
			continue
		}

		current := remoteSummaryFromMetadata(entry)
		cacheKey := entry.Remote.Provider + ":" + entry.Remote.ID + ":update"
		var cached remote.UpdateInfo
		if hit, err := cache.Load("updates", cacheKey, &cached); err == nil && hit {
			cached.Cached = true
			entry.Update = &cached
			continue
		}

		available, hasUpdate, err := provider.CheckUpdate(current)
		if err != nil {
			continue
		}

		update := remote.UpdateInfo{Current: current, Available: available, HasUpdate: hasUpdate}
		if err := cache.Save("updates", cacheKey, update); err == nil {
			entry.Update = &update
		}
	}

	return nil
}

func remoteSummaryFromMetadata(entry *SkillStatus) remote.SkillSummary {
	return remote.SkillSummary{
		Provider:      entry.Remote.Provider,
		ID:            entry.Remote.ID,
		Name:          entry.Skill.FlattenedName,
		Source:        sourceName(entry.Remote.Source),
		SourceURL:     entry.Remote.Source,
		Version:       entry.Remote.Version,
		VersionDate:   parseTime(entry.Remote.Date),
		Hash:          entry.Remote.Hash,
		RepoSkillPath: entry.Remote.RepoSkillPath,
	}
}

func sourceName(source string) string {
	trimmed := source
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	return strings.Trim(trimmed, "/")
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
