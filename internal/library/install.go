package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

type InstallRequest struct {
	Location         configpkg.Location
	DestinationPaths map[string]string
	Bundles          []remote.SkillBundle
}

type InstalledSkill struct {
	Summary          remote.SkillSummary
	DestinationPath  string
	SourceSkill      skills.Skill
	CreatedDirectory string
}

func Install(request InstallRequest) ([]InstalledSkill, error) {
	installed := make([]InstalledSkill, 0, len(request.Bundles))
	for _, bundle := range request.Bundles {
		destinationParent, ok := request.DestinationPaths[bundle.Skill.ID]
		if !ok {
			return nil, fmt.Errorf("missing destination path for %s", bundle.Skill.ID)
		}

		skillDir := filepath.Join(request.Location.Source, filepath.FromSlash(strings.Trim(destinationParent, "/")), sanitizeSkillDirName(bundle.Skill.Name))
		if _, err := os.Stat(skillDir); err == nil {
			return nil, fmt.Errorf("skill destination already exists: %s", skillDir)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check skill destination %s: %w", skillDir, err)
		}
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return nil, fmt.Errorf("create skill directory %s: %w", skillDir, err)
		}

		for _, file := range bundle.Files {
			path := filepath.Join(skillDir, filepath.FromSlash(file.Path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, fmt.Errorf("create file directory %s: %w", path, err)
			}
			if err := os.WriteFile(path, []byte(file.Contents), 0o644); err != nil {
				return nil, fmt.Errorf("write skill file %s: %w", path, err)
			}
		}

		relPath, err := filepath.Rel(request.Location.Source, skillDir)
		if err != nil {
			return nil, fmt.Errorf("compute installed skill path for %s: %w", bundle.Skill.Name, err)
		}

		skill, err := skills.ResolveSourceSkill(request.Location.Source, relPath)
		if err != nil {
			return nil, err
		}

		if err := skills.RewriteManagedFields(skill, true, false); err != nil {
			return nil, err
		}
		if err := skills.RewriteRemoteMetadata(skill, remoteMetadataFromSummary(bundle.Skill)); err != nil {
			return nil, err
		}

		installed = append(installed, InstalledSkill{
			Summary:          bundle.Skill,
			DestinationPath:  filepath.ToSlash(strings.Trim(strings.Trim(destinationParent, "/"), " ")),
			SourceSkill:      skill,
			CreatedDirectory: skillDir,
		})
	}

	sort.Slice(installed, func(i, j int) bool {
		return installed[i].SourceSkill.RelativePath < installed[j].SourceSkill.RelativePath
	})

	return installed, nil
}

func remoteMetadataFromSummary(summary remote.SkillSummary) skills.RemoteMetadata {
	return skills.RemoteMetadata{
		Provider:      summary.Provider,
		Source:        summary.SourceURL,
		ID:            summary.ID,
		Version:       summary.Version,
		Date:          formatTime(summary.VersionDate),
		Hash:          summary.Hash,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339),
		RepoSkillPath: summary.RepoSkillPath,
	}
}

func sanitizeSkillDirName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.ReplaceAll(trimmed, string(filepath.Separator), "-")
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	if trimmed == "" {
		return "skill"
	}
	return trimmed
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
