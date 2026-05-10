package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type skillUpdateCandidate struct {
	Skill          skills.Skill
	Metadata       skills.ManagedMetadata
	InstalledPath  string
	Installed      string
	Latest         string
	Source         string
	Bundle         remotepkg.SkillBundle
	InstalledFiles remotepkg.SkillBundle
}

var (
	fetchSkillBundleFunc = remotepkg.FetchSkillBundle
	cachePathFunc        = configpkg.CachePath
)

func newCheckUpdatesCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "check-updates",
		Aliases: []string{"updates", "upgrade"},
		Short:   "Check imported skills for upstream updates",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			candidates, err := collectUpdateCandidates(location)
			if err != nil {
				return err
			}
			if len(candidates) == 0 {
				pterm.Info.Println("No skill updates found")
				if err := refreshSkillUpdateCache(nil); err != nil {
					pterm.Warning.Printfln("Could not update skill-update cache: %v", err)
				}
				return nil
			}

			selected := candidates
			if !yes {
				selector := newSkillUpdateSelector(candidates)
				if err := selector.Run(); err != nil {
					return err
				}
				selected = selector.Selected()
			}
			if len(selected) == 0 {
				pterm.Info.Println("No skills selected for update")
				return refreshSkillUpdateCache(candidates)
			}

			for _, item := range selected {
				backupPath, err := backuppkg.MoveSkill(item.Skill.Dir, item.Skill.FlattenedName, backuppkg.Metadata{
					OriginalPath:  item.Skill.RelativePath,
					FlattenedName: item.Skill.FlattenedName,
					UpdatedFrom:   item.Installed,
					UpdatedTo:     item.Latest,
				}, time.Now().UTC())
				if err != nil {
					return err
				}
				if err := writeImportedBundle(item.Skill.Dir, item.Bundle); err != nil {
					return err
				}
				modTime, _ := remotepkg.LatestFileModTime(item.Bundle.Root)
				if err := skills.RewriteManagedFieldsWithMetadata(item.Skill, true, item.Metadata.Disabled, skills.ManagedMetadata{
					Source:           item.Metadata.Source,
					SourceType:       item.Metadata.SourceType,
					InstalledVersion: item.Latest,
					InstalledAt:      item.Metadata.InstalledAt,
					RepoSkillPath:    item.Metadata.RepoSkillPath,
					LastUpdatedAt:    formatOptionalTime(modTime),
				}); err != nil {
					return err
				}
				pterm.Success.Printfln("Updated skill: %s (%s -> %s)", item.Skill.RelativePath, item.Installed, item.Latest)
				pterm.Info.Printfln("Backup: %s", backupPath)
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}
			if err := refreshSkillUpdateCache(candidates); err != nil {
				pterm.Warning.Printfln("Could not update skill-update cache: %v", err)
			}
			printSyncResult(configFile, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Update all detected skills without interactive selection")
	return cmd
}

func collectUpdateCandidates(location configpkg.Location) ([]skillUpdateCandidate, error) {
	scanned, err := skills.ScanSource(location.Source)
	if err != nil {
		return nil, err
	}
	results := make([]skillUpdateCandidate, 0)
	for _, skill := range scanned {
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return nil, err
		}
		metadata := doc.ManagedMetadata()
		if strings.TrimSpace(metadata.Source) == "" || strings.TrimSpace(metadata.RepoSkillPath) == "" {
			continue
		}
		upstreamName := strings.TrimSpace(metadata.OriginalName)
		if upstreamName == "" {
			upstreamName = strings.TrimSpace(doc.Name())
		}
		bundle, err := fetchSkillBundleFunc(metadata.Source, upstreamName)
		if err != nil {
			continue
		}
		installedBundle, err := remotepkg.LoadBundleFromDir(skill.Dir)
		if err != nil {
			return nil, err
		}
		installed := strings.TrimSpace(metadata.InstalledVersion)
		latest := remotepkg.ResolveVersion(bundle.Skill, bundle.Files)
		if installed == "" || latest == "" || installed == latest {
			continue
		}
		results = append(results, skillUpdateCandidate{
			Skill:          skill,
			Metadata:       metadata,
			InstalledPath:  skill.Dir,
			Installed:      installed,
			Latest:         latest,
			Source:         metadata.Source,
			Bundle:         bundle,
			InstalledFiles: installedBundle,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Skill.RelativePath < results[j].Skill.RelativePath })
	return results, nil
}

type skillUpdateSelector struct {
	items           []skillUpdateCandidate
	selected        map[int]bool
	active          int
	lastRenderLines int
}

func newSkillUpdateSelector(items []skillUpdateCandidate) *skillUpdateSelector {
	selected := make(map[int]bool, len(items))
	for i := range items {
		selected[i] = true
	}
	return &skillUpdateSelector{items: items, selected: selected}
}

func (s *skillUpdateSelector) Run() error {
	defer showTerminalCursor()
	if _, err := fmt.Fprintln(os.Stdout, "Select skills to update. Press d to inspect the diff for the highlighted skill."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return err
	}
	s.render()
	return keyboard.Listen(func(key keys.Key) (bool, error) {
		switch key.Code {
		case keys.CtrlC:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, fmt.Errorf("interrupted")
		case keys.Enter:
			_, _ = fmt.Fprintln(os.Stdout)
			return true, nil
		case keys.Up:
			if s.active > 0 {
				s.active--
				s.render()
			}
			return false, nil
		case keys.Down:
			if s.active < len(s.items)-1 {
				s.active++
				s.render()
			}
			return false, nil
		case keys.Space:
			s.selected[s.active] = !s.selected[s.active]
			s.render()
			return false, nil
		case keys.RuneKey:
			if len(key.Runes) == 1 && (key.Runes[0] == 'd' || key.Runes[0] == 'D') {
				if err := s.showDiff(); err != nil {
					return false, err
				}
				s.render()
			}
			return false, nil
		default:
			return false, nil
		}
	})
}

func (s *skillUpdateSelector) Selected() []skillUpdateCandidate {
	selected := make([]skillUpdateCandidate, 0, len(s.items))
	for i, item := range s.items {
		if s.selected[i] {
			selected = append(selected, item)
		}
	}
	return selected
}

func (s *skillUpdateSelector) render() {
	hideTerminalCursor()
	lines := []string{"Space: Toggle, Up/Down: Move, d: Diff, Enter: Continue"}
	for i, item := range s.items {
		prefix := "  "
		if i == s.active {
			prefix = "> "
		}
		marker := styledSelectionMarker(s.selected[i])
		lines = append(lines, fmt.Sprintf("%s%s %s [%s] -> [%s] [%s]", prefix, marker, item.Skill.RelativePath, item.Installed, item.Latest, item.Source))
		}
	if s.lastRenderLines > 0 {
		fmt.Printf("\033[%dA", s.lastRenderLines)
	}
	fmt.Print("\r\033[J")
	for i, line := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print(line)
	}
	if len(lines) > 0 {
		fmt.Print("\n")
	}
	s.lastRenderLines = len(lines)
}

func (s *skillUpdateSelector) showDiff() error {
	showTerminalCursor()
	defer hideTerminalCursor()
	item := s.items[s.active]
	lines := diffLines(item.InstalledFiles.Files, item.Bundle.Files)
	if len(lines) == 0 {
		lines = []string{"No textual diff available."}
	}
	offset := 0
	render := func() {
		fmt.Print("\r\033[J")
		fmt.Printf("Diff: %s [%s -> %s]\n", item.Skill.RelativePath, item.Installed, item.Latest)
		fmt.Println("Up/Down: Scroll, Esc/Enter: Back")
		max := offset + 20
		if max > len(lines) {
			max = len(lines)
		}
		for _, line := range lines[offset:max] {
			fmt.Println(line)
		}
	}
	render()
	return keyboard.Listen(func(key keys.Key) (bool, error) {
		switch key.Code {
		case keys.Up:
			if offset > 0 {
				offset--
				render()
			}
			return false, nil
		case keys.Down:
			if offset+20 < len(lines) {
				offset++
				render()
			}
			return false, nil
		case keys.Enter, keys.Escape:
			fmt.Print("\r\033[J")
			return true, nil
		case keys.CtrlC:
			return true, fmt.Errorf("interrupted")
		default:
			return false, nil
		}
	})
}

func diffLines(oldFiles []remotepkg.File, newFiles []remotepkg.File) []string {
	oldMap := make(map[string]string, len(oldFiles))
	newMap := make(map[string]string, len(newFiles))
	paths := make([]string, 0, len(oldFiles)+len(newFiles))
	seen := map[string]struct{}{}
	for _, file := range oldFiles {
		oldMap[file.Path] = file.Contents
		if _, ok := seen[file.Path]; !ok {
			seen[file.Path] = struct{}{}
			paths = append(paths, file.Path)
		}
	}
	for _, file := range newFiles {
		newMap[file.Path] = file.Contents
		if _, ok := seen[file.Path]; !ok {
			seen[file.Path] = struct{}{}
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	lines := make([]string, 0)
	for _, path := range paths {
		oldContent, oldOK := oldMap[path]
		newContent, newOK := newMap[path]
		if oldOK && newOK && oldContent == newContent {
			continue
		}
		lines = append(lines, "--- "+path)
		lines = append(lines, "+++ "+path)
		if oldOK {
			for _, line := range strings.Split(strings.TrimSuffix(oldContent, "\n"), "\n") {
				lines = append(lines, "- "+line)
			}
		}
		if newOK {
			for _, line := range strings.Split(strings.TrimSuffix(newContent, "\n"), "\n") {
				lines = append(lines, "+ "+line)
			}
		}
		lines = append(lines, "")
	}
	return lines
}

func refreshSkillUpdateCache(candidates []skillUpdateCandidate) error {
	cachePath, err := cachePathFunc()
	if err != nil {
		return err
	}
	cache, err := configpkg.LoadUpdateCacheOrDefault(cachePath)
	if err != nil {
		return err
	}
	pending := make([]configpkg.SkillUpdateRecord, 0, len(candidates))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range candidates {
		pending = append(pending, configpkg.SkillUpdateRecord{
			RelativePath:     item.Skill.RelativePath,
			FlattenedName:    item.Skill.FlattenedName,
			InstalledVersion: item.Installed,
			LatestVersion:    item.Latest,
			Source:           item.Source,
			RepoSkillPath:    item.Metadata.RepoSkillPath,
			CheckedAt:        now,
		})
	}
	cache.SkillUpdates = configpkg.SkillUpdateCache{LastCheckedAt: now, Pending: pending}
	return configpkg.SaveUpdateCache(cachePath, cache)
}
