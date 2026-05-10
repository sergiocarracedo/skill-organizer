package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type skillAddSandbox interface {
	Close()
	InstalledSkills() ([]remotepkg.InstalledSkill, error)
	Run(args ...string) (string, error)
	LoadInstalledBundle(skill remotepkg.SkillSummary) (remotepkg.SkillBundle, error)
}

var (
	skillAddLoadResolvedLocation    = loadResolvedLocation
	detectSkillsCLIFunc             = remotepkg.DetectSkillsCLI
	newSkillAddSandbox              = func() (skillAddSandbox, error) { return remotepkg.NewSandbox() }
	skillAddSourceFolderSuggestions = sourceFolderSuggestions
	chooseSkillAddTargets           = selectImportedSkillTargets
	confirmSkillAddReinstall        = confirm
	moveSkillToBackup               = backuppkg.MoveSkill
	latestSkillBundleModTime        = remotepkg.LatestFileModTime
	rewriteManagedSkillMetadata     = skills.RewriteManagedFieldsWithMetadata
	runSkillAddSync                 = syncpkg.Run
)

func newSkillAddCommand() *cobra.Command {
	var skillNames []string

	cmd := &cobra.Command{
		Use:     "add <source>",
		Aliases: []string{"install", "import"},
		Short:   "Add skills from skills.sh into the managed source tree",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := skillAddLoadResolvedLocation()
			if err != nil {
				return err
			}

			existing, err := skills.ScanSource(location.Source)
			if err != nil {
				return err
			}
			existingNames := existingSkillsByName(existing)

			runner, err := detectSkillsCLIFunc()
			if err != nil {
				return err
			}
			pterm.Info.Println("Using skills.sh cli tool to add the skills")
			pterm.Info.Printfln("Detected CLI: %s", runner.Label())

			source := strings.TrimSpace(args[0])
			sandbox, err := newSkillAddSandbox()
			if err != nil {
				return err
			}
			defer sandbox.Close()

			requestedSkillNames, approvedReinstalls, err := resolveRequestedSkillInstalls(skillNames, existingNames)
			if err != nil {
				return err
			}
			if len(skillNames) > 0 && len(requestedSkillNames) == 0 {
				pterm.Info.Println("No new skills selected for installation")
				result, err := runSkillAddSync(location)
				if err != nil {
					return err
				}
				printSyncResult(configFile, result)
				return nil
			}

			before, err := sandbox.InstalledSkills()
			if err != nil {
				return err
			}
			beforeNames := installedSkillNames(before)

			commandArgs := []string{"add", source}
			for _, name := range requestedSkillNames {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				commandArgs = append(commandArgs, "--skill", trimmed)
			}
			commandArgs = append(commandArgs, "-y", "--copy")
			if _, err := sandbox.Run(commandArgs...); err != nil {
				return err
			}
			pterm.Info.Println("Returned to " + cliBrandText("skill-organizer") + ". Importing installed skills...")

			after, err := sandbox.InstalledSkills()
			if err != nil {
				return err
			}
			selected := newlyInstalledSkills(beforeNames, after)
			if len(selected) == 0 {
				return fmt.Errorf("no skills were installed")
			}

			bundles := make(map[string]remotepkg.SkillBundle, len(selected))
			for _, installed := range selected {
				bundle, err := sandbox.LoadInstalledBundle(remotepkg.SkillSummary{
					Name:       installed.Name,
					Source:     source,
					SourceURL:  "https://github.com/" + strings.Trim(source, "/"),
					SourceType: "github",
				})
				if err != nil {
					return err
				}
				bundles[installed.Name] = bundle
			}

			suggestions, err := skillAddSourceFolderSuggestions(location.Source)
			if err != nil {
				return err
			}
			relativeTargets, err := chooseSkillAddTargets(selected, suggestions)
			if err != nil {
				return err
			}

			for _, installed := range selected {
				bundle := bundles[installed.Name]
				relative, ok := relativeTargets[installed.Name]
				if !ok {
					return fmt.Errorf("missing target folder for installed skill %s", installed.Name)
				}
				targetSkill, err := skills.ResolveSourceSkillTarget(location.Source, relative)
				if err != nil {
					return err
				}
				if existingSkill, ok := existingNames[installed.Name]; ok {
					reinstall := approvedReinstalls[installed.Name]
					if !reinstall {
						pterm.Warning.Printfln("Skill %s already exists at %s", installed.Name, existingSkill.RelativePath)
						reinstall, err = confirmSkillAddReinstall(fmt.Sprintf("Reinstall %s and replace the existing managed skill?", installed.Name), false)
						if err != nil {
							return err
						}
					}
					if !reinstall {
						pterm.Info.Printfln("Skipped reinstall for: %s", installed.Name)
						continue
					}
					backupPath, err := moveSkillToBackup(existingSkill.Dir, existingSkill.FlattenedName, backuppkg.Metadata{
						OriginalPath:  existingSkill.RelativePath,
						FlattenedName: existingSkill.FlattenedName,
						UpdatedFrom:   currentInstalledVersion(existingSkill),
						UpdatedTo:     remotepkg.ResolveVersion(bundle.Skill, bundle.Files),
					}, time.Now().UTC())
					if err != nil {
						return err
					}
					pterm.Info.Printfln("Backed up previous skill to: %s", backupPath)
					targetSkill = existingSkill
				}
				if err := writeImportedBundle(targetSkill.Dir, bundle); err != nil {
					return err
				}
				modTime, _ := latestSkillBundleModTime(bundle.Root)
				if err := rewriteManagedSkillMetadata(targetSkill, true, false, skills.ManagedMetadata{
					Source:           bundle.Skill.Source,
					SourceType:       bundle.Skill.SourceType,
					InstalledVersion: remotepkg.ResolveVersion(bundle.Skill, bundle.Files),
					InstalledAt:      time.Now().UTC().Format(time.RFC3339),
					RepoSkillPath:    bundle.Skill.RepoSkillPath,
					LastUpdatedAt:    formatOptionalTime(modTime),
				}); err != nil {
					return err
				}
				existingNames[installed.Name] = targetSkill
				pterm.Success.Printfln("Imported skill: %s -> %s", installed.Name, targetSkill.RelativePath)
			}

			result, err := runSkillAddSync(location)
			if err != nil {
				return err
			}
			printSyncResult(configFile, result)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&skillNames, "skill", nil, "Add one or more specific skills from the source")
	return cmd
}

func existingSkillsByName(items []skills.Skill) map[string]skills.Skill {
	results := make(map[string]skills.Skill, len(items))
	for _, item := range items {
		results[filepath.Base(item.RelativePath)] = item
		doc, err := skills.LoadDocument(item.SkillFile)
		if err == nil {
			metadata := doc.ManagedMetadata()
			if strings.TrimSpace(metadata.OriginalName) != "" {
				results[strings.TrimSpace(metadata.OriginalName)] = item
			}
			if strings.TrimSpace(doc.Name()) != "" {
				results[strings.TrimSpace(doc.Name())] = item
			}
		}
	}
	return results
}

func currentInstalledVersion(skill skills.Skill) string {
	doc, err := skills.LoadDocument(skill.SkillFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.ManagedMetadata().InstalledVersion)
}

func installedSkillNames(entries []remotepkg.InstalledSkill) map[string]struct{} {
	results := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		results[entry.Name] = struct{}{}
	}
	return results
}

func selectImportedSkillTargets(installed []remotepkg.InstalledSkill, suggestions []string) (map[string]string, error) {
	items := make([]editablePathSelectorItem, 0, len(installed))
	for _, item := range installed {
		items = append(items, editablePathSelectorItem{
			Key:            item.Name,
			Label:          item.Name,
			Parent:         filepath.ToSlash(item.Name),
			AlwaysSelected: true,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	selector := newEditablePathSelector(items, suggestions, editablePathSelectorOptions{
		Intro:          "Set the target folders for the imported skills.",
		BasePathLabel:  "skills-organized/",
		ShowToggleAll:  false,
		ShowCheckboxes: false,
		HelpNavigate:   "🡹/🡻: Move, 🡺: Edit folder, Enter: Continue",
	})
	if err := selector.Run(); err != nil {
		return nil, err
	}
	results := make(map[string]string, len(items))
	for _, item := range items {
		results[item.Key] = selector.ParentFor(item.Key)
	}
	return results, nil
}

func resolveRequestedSkillInstalls(requested []string, existing map[string]skills.Skill) ([]string, map[string]bool, error) {
	filtered := make([]string, 0, len(requested))
	approvedReinstalls := make(map[string]bool)
	seen := make(map[string]struct{}, len(requested))
	for _, item := range requested {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		existingSkill, ok := existing[name]
		if !ok {
			filtered = append(filtered, name)
			continue
		}
		pterm.Warning.Printfln("Skill %s already exists at %s", name, existingSkill.RelativePath)
		reinstall, err := confirmSkillAddReinstall(fmt.Sprintf("Reinstall %s and replace the existing managed skill?", name), false)
		if err != nil {
			return nil, nil, err
		}
		if !reinstall {
			pterm.Info.Printfln("Skipped reinstall for: %s", name)
			continue
		}
		approvedReinstalls[name] = true
		filtered = append(filtered, name)
	}
	return filtered, approvedReinstalls, nil
}

func newlyInstalledSkills(before map[string]struct{}, after []remotepkg.InstalledSkill) []remotepkg.InstalledSkill {
	results := make([]remotepkg.InstalledSkill, 0, len(after))
	for _, entry := range after {
		if _, ok := before[entry.Name]; ok {
			continue
		}
		results = append(results, entry)
	}
	return results
}

func writeImportedBundle(targetDir string, bundle remotepkg.SkillBundle) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create imported skill directory: %w", err)
	}
	for _, file := range bundle.Files {
		path := filepath.Join(targetDir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create imported skill parent: %w", err)
		}
		if err := os.WriteFile(path, []byte(file.Contents), 0o644); err != nil {
			return fmt.Errorf("write imported skill file: %w", err)
		}
	}
	return nil
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
