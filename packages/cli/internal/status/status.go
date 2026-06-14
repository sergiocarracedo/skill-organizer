package status

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type SkillState string

const (
	StateSynced        SkillState = "synced"
	StateDisabled      SkillState = "disabled"
	StateMissingTarget SkillState = "missing-target"
	StateBrokenLink    SkillState = "broken-link"
	StateDrifted       SkillState = "drifted"
)

type SkillStatus struct {
	Skill                skills.Skill
	State                SkillState
	LinkPath             string
	LinkTarget           string
	InstalledVersion     string
	InstalledDate        string
	AvailableVersion     string
	AvailableCheckedDate string
	RiskScore            int
	RiskEvaluatedAt      string
	RiskEvaluator        string
	RiskSourceHash       string
}

type Report struct {
	Skills    []SkillStatus
	Unmanaged []string
}

type Summary struct {
	TotalSkills     int
	ManagedSkills   int
	UnmanagedSkills int
	Synced          int
	Disabled        int
	MissingTarget   int
	BrokenLink      int
	Drifted         int
	Safe            int
	Warning         int
	Danger          int
}

func (r Report) Summary() Summary {
	summary := Summary{
		TotalSkills:     len(r.Skills),
		UnmanagedSkills: len(r.Unmanaged),
	}

	for _, entry := range r.Skills {
		switch entry.State {
		case StateSynced:
			summary.Synced++
		case StateDisabled:
			summary.Disabled++
		case StateMissingTarget:
			summary.MissingTarget++
		case StateBrokenLink:
			summary.BrokenLink++
		case StateDrifted:
			summary.Drifted++
		}

		if entry.RiskEvaluator == "" {
			continue
		}
		switch {
		case entry.RiskScore >= 70:
			summary.Danger++
		case entry.RiskScore >= 30:
			summary.Warning++
		default:
			summary.Safe++
		}
	}

	// Disabled skills are tracked in source, but they do not produce target entries.
	summary.ManagedSkills = summary.TotalSkills - summary.Disabled
	return summary
}

func Build(location configpkg.Location) (Report, error) {
	if err := location.Validate(); err != nil {
		return Report{}, err
	}

	scanned, err := skills.ScanSource(location.Source)
	if err != nil {
		return Report{}, err
	}

	manifest, err := syncpkg.LoadManifestOrEmpty(location.Target)
	if err != nil {
		return Report{}, err
	}

	updateLookup, err := loadPendingUpdates()
	if err != nil {
		return Report{}, err
	}

	report := Report{}
	managedNames := make(map[string]struct{}, len(manifest.Managed))
	for name := range manifest.Managed {
		managedNames[name] = struct{}{}
	}

	for _, skill := range scanned {
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return Report{}, err
		}

		entry := SkillStatus{Skill: skill, LinkPath: filepath.Join(location.Target, skill.FlattenedName)}
		metadata := doc.ManagedMetadata()
		entry.InstalledVersion = metadata.InstalledVersion
		entry.InstalledDate = metadata.LastUpdatedAt
		entry.RiskScore = metadata.RiskScore
		entry.RiskEvaluatedAt = metadata.RiskEvaluatedAt
		entry.RiskEvaluator = metadata.RiskEvaluator
		entry.RiskSourceHash = metadata.RiskSourceHash
		if update, ok := lookupPendingUpdate(updateLookup, skill); ok {
			entry.AvailableVersion = update.LatestVersion
			entry.AvailableCheckedDate = update.CheckedAt
		}
		if metadata.Disabled {
			entry.State = StateDisabled
			report.Skills = append(report.Skills, entry)
			continue
		}

		info, err := os.Lstat(entry.LinkPath)
		if os.IsNotExist(err) {
			entry.State = StateMissingTarget
			report.Skills = append(report.Skills, entry)
			continue
		}
		if err != nil {
			return Report{}, fmt.Errorf("stat target entry %q: %w", entry.LinkPath, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			entry.State = StateDrifted
			report.Skills = append(report.Skills, entry)
			continue
		}

		entry.LinkTarget, err = os.Readlink(entry.LinkPath)
		if err != nil {
			return Report{}, fmt.Errorf("read target symlink %q: %w", entry.LinkPath, err)
		}

		expectedTarget, err := filepath.Rel(location.Target, skill.Dir)
		if err != nil {
			return Report{}, fmt.Errorf("compute expected target for %q: %w", skill.Dir, err)
		}

		if entry.LinkTarget != expectedTarget {
			entry.State = StateDrifted
		} else {
			if _, ok := managedNames[skill.FlattenedName]; ok {
				entry.State = StateSynced
			} else {
				entry.State = StateBrokenLink
			}
		}

		report.Skills = append(report.Skills, entry)
	}

	entries, err := os.ReadDir(location.Target)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return Report{}, fmt.Errorf("read target directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".skill-organizer.manifest.yml" {
			continue
		}
		if _, ok := managedNames[name]; ok {
			continue
		}
		if !isUnmanagedSkillDir(location.Target, entry) {
			continue
		}
		report.Unmanaged = append(report.Unmanaged, name)
	}

	sort.Strings(report.Unmanaged)
	sort.Slice(report.Skills, func(i, j int) bool {
		return report.Skills[i].Skill.RelativePath < report.Skills[j].Skill.RelativePath
	})

	return report, nil
}

func loadPendingUpdates() (map[string]configpkg.SkillUpdateRecord, error) {
	updatesPath, err := configpkg.UpdatesPath()
	if err != nil {
		return nil, fmt.Errorf("resolve updates path: %w", err)
	}
	state, err := configpkg.LoadUpdatesStateOrDefault(updatesPath)
	if err != nil {
		return nil, err
	}
	lookup := make(map[string]configpkg.SkillUpdateRecord, len(state.Pending)*2)
	for _, pending := range state.Pending {
		if pending.RelativePath != "" {
			lookup[pending.RelativePath] = pending
		}
		if pending.FlattenedName != "" {
			lookup[pending.FlattenedName] = pending
		}
	}
	return lookup, nil
}

func lookupPendingUpdate(lookup map[string]configpkg.SkillUpdateRecord, skill skills.Skill) (configpkg.SkillUpdateRecord, bool) {
	if update, ok := lookup[skill.RelativePath]; ok {
		return update, true
	}
	update, ok := lookup[skill.FlattenedName]
	return update, ok
}

func isUnmanagedSkillDir(targetRoot string, entry os.DirEntry) bool {
	if !entry.IsDir() {
		return false
	}

	info, err := os.Stat(filepath.Join(targetRoot, entry.Name(), skills.SkillFileName))
	if err != nil {
		return false
	}

	return !info.IsDir()
}
