package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

func newRemoteManager() *remote.Manager {
	return remote.NewManager(remote.SkillsShProvider{}, remote.GitHubProvider{})
}

func newRemoteService() (*remote.Service, error) {
	return remote.NewService()
}

func loadAppConfigForSkills() (string, configpkg.AppConfig, error) {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return "", configpkg.AppConfig{}, err
	}

	appConfig, err := configpkg.LoadAppConfigOrDefault(registryPath)
	if err != nil {
		return "", configpkg.AppConfig{}, err
	}

	return registryPath, appConfig, nil
}

func sourceSuggestions(location configpkg.Location) ([]string, error) {
	suggestions, err := sourceFolderSuggestions(location.Source)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if strings.HasPrefix(suggestion, ".old") {
			continue
		}
		filtered = append(filtered, suggestion)
	}
	return filtered, nil
}

func promptInstallDestination(location configpkg.Location) (string, error) {
	suggestions, err := sourceSuggestions(location)
	if err != nil {
		return "", err
	}

	value, err := promptTextWithSuggestionsBelow("Select the destination folder relative to organized-skills/", "", suggestions)
	if err != nil {
		return "", err
	}

	trimmed := strings.Trim(filepath.ToSlash(strings.TrimSpace(value)), "/")
	if strings.HasPrefix(trimmed, ".old") {
		return "", fmt.Errorf("destination cannot be inside organized-skills/.old")
	}
	return trimmed, nil
}

func promptInstallScope(current configpkg.Location) (configpkg.Location, error) {
	globalTarget, err := configpkg.HomeFallbackTarget()
	globalLocation := configpkg.Location{}
	globalAvailable := err == nil
	if globalAvailable {
		globalLocation = configpkg.Location{
			Source: configpkg.DefaultSourceForTarget(globalTarget),
			Target: globalTarget,
		}
	}

	options := []string{"Project"}
	if globalAvailable {
		options = append(options, "Global")
	}

	selected, err := selectOption("Where should the skill be installed?", options, options[0])
	if err != nil {
		return configpkg.Location{}, err
	}

	if selected == "Global" {
		return globalLocation, nil
	}
	return current, nil
}

func renderAuditReport(report remote.AuditReport) {
	pterm.DefaultSection.Println("Audit")
	pterm.Println("Provider: " + report.Skill.Provider)
	pterm.Println("Source: " + report.Skill.SourceURL)
	if len(report.Entries) == 0 {
		pterm.Warning.Println("No audit results available")
		return
	}

	for _, entry := range report.Entries {
		line := entry.Provider + ": " + entry.Status
		if entry.RiskLevel != "" {
			line += " [" + entry.RiskLevel + "]"
		}
		if entry.Summary != "" {
			line += " - " + entry.Summary
		}
		pterm.Println(line)
	}
}

func selectRemoteSkills(candidates []remote.SkillSummary) ([]remote.SkillSummary, error) {
	if len(candidates) == 1 {
		return candidates, nil
	}

	options := make([]string, 0, len(candidates)+1)
	options = append(options, toggleAllOption)
	byOption := make(map[string]remote.SkillSummary, len(candidates))
	for _, candidate := range candidates {
		label := candidate.Name
		if candidate.RepoSkillPath != "" {
			label += " -> " + candidate.RepoSkillPath
		}
		options = append(options, label)
		byOption[label] = candidate
	}

	selected, err := selectMultiple("Select skills to install", options, options[1:])
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, nil
	}

	includeAll := false
	for _, item := range selected {
		if item == toggleAllOption {
			includeAll = true
			break
		}
	}
	if includeAll {
		return candidates, nil
	}

	result := make([]remote.SkillSummary, 0, len(selected))
	for _, item := range selected {
		candidate, ok := byOption[item]
		if !ok {
			continue
		}
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func resolveSkillBySourcePath(location configpkg.Location, sourceRelativePath string) (skills.Skill, skills.Document, error) {
	skill, err := skills.ResolveSourceSkill(location.Source, sourceRelativePath)
	if err != nil {
		return skills.Skill{}, skills.Document{}, err
	}

	doc, err := skills.LoadDocument(skill.SkillFile)
	if err != nil {
		return skills.Skill{}, skills.Document{}, err
	}

	return skill, doc, nil
}

func remoteSummaryFromMetadata(name string, metadata skills.RemoteMetadata) remote.SkillSummary {
	return remote.SkillSummary{
		Provider:      metadata.Provider,
		ID:            metadata.ID,
		Name:          name,
		Source:        remoteSourceName(metadata),
		SourceURL:     metadata.Source,
		Version:       metadata.Version,
		VersionDate:   parseRFC3339(metadata.Date),
		Hash:          metadata.Hash,
		RepoSkillPath: metadata.RepoSkillPath,
	}
}

func remoteSourceName(metadata skills.RemoteMetadata) string {
	trimmed := strings.TrimSpace(metadata.Source)
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	return strings.Trim(trimmed, "/")
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
