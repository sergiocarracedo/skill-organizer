package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/library"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSkillUpdateCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "update [source-relative-path ...]",
		Short: "Update installed remote-managed skills",
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			_, appConfig, err := loadAppConfigForSkills()
			if err != nil {
				return err
			}

			installed, err := library.InstalledRemoteSkills(location.Source)
			if err != nil {
				return err
			}
			if len(installed) == 0 {
				if jsonOutput {
					return printJSON([]pendingSkillUpdate{})
				}
				pterm.Info.Println("No remote-managed skills found")
				return nil
			}

			service, err := newRemoteService()
			if err != nil {
				return err
			}
			manager := service.Manager()
			updates := make([]pendingSkillUpdate, 0)
			for _, skill := range installed {
				if len(args) > 0 && !contains(args, skill.RelativePath) {
					continue
				}

				doc, err := skills.LoadDocument(skill.SkillFile)
				if err != nil {
					return err
				}
				metadata := doc.RemoteMetadata()
				provider, err := manager.Provider(metadata.Provider)
				if err != nil {
					return err
				}

				current := remoteSummaryFromMetadata(doc.Name(), metadata)
				updateInfo, err := service.Update(provider, current)
				if err != nil {
					pterm.Warning.Printfln("Update check failed for %s: %v", skill.RelativePath, err)
					continue
				}
				if !updateInfo.HasUpdate {
					continue
				}

				updates = append(updates, pendingSkillUpdate{Skill: skill, Current: current, Available: updateInfo.Available, Provider: provider})
			}

			if len(updates) == 0 {
				pterm.Success.Println("All selected skills are up to date")
				if jsonOutput {
					return printJSON([]pendingSkillUpdate{})
				}
				return nil
			}
			if jsonOutput {
				return printJSON(updates)
			}

			selected, err := selectUpdates(updates)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				pterm.Info.Println("No skills selected for update")
				return nil
			}

			for _, update := range selected {
				bundle, err := service.FetchSkill(update.Provider, update.Available)
				if err != nil {
					return err
				}

				showDiff, err := confirm(fmt.Sprintf("Display diff for %s before updating?", update.Skill.RelativePath), false)
				if err != nil {
					return err
				}
				if showDiff {
					diff, err := diffSkill(update.Skill, bundle)
					if err != nil {
						return err
					}
					if diff == "" {
						pterm.Info.Println("No content diff detected")
					} else {
						pterm.DefaultSection.Println("Diff")
						pterm.Println(diff)
					}
				}

				ok, err := confirm(fmt.Sprintf("Update %s from %s to %s?", update.Skill.RelativePath, update.Current.Version, update.Available.Version), true)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}

				if _, err := library.MoveToBackup(location.Source, update.Skill, time.Now().UTC()); err != nil {
					return err
				}

				_, err = library.Install(library.InstallRequest{
					Location:         location,
					DestinationPaths: map[string]string{bundle.Skill.ID: filepath.ToSlash(filepath.Dir(update.Skill.RelativePath))},
					Bundles:          []remote.SkillBundle{bundle},
				})
				if err != nil {
					return err
				}

				pterm.Success.Printfln("Updated %s: %s -> %s", update.Skill.RelativePath, update.Current.Version, update.Available.Version)
			}

			if err := library.GarbageCollectBackups(location.Source, appConfig.Backups.RetentionDays, time.Now().UTC()); err != nil {
				return err
			}

			result, err := syncpkg.Run(location)
			if err != nil {
				return err
			}
			printSyncResult(configFile, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print available updates as JSON")
	return cmd
}

type pendingSkillUpdate struct {
	Skill     skills.Skill         `json:"skill"`
	Current   remote.SkillSummary  `json:"current"`
	Available remote.SkillSummary  `json:"available"`
	Provider  remote.Provider      `json:"-"`
}

func selectUpdates(updates []pendingSkillUpdate) ([]pendingSkillUpdate, error) {
	options := []string{toggleAllOption}
	byOption := make(map[string]pendingSkillUpdate, len(updates))
	for _, update := range updates {
		label := fmt.Sprintf("%s [%s] -> [%s]", update.Skill.RelativePath, update.Current.Version, update.Available.Version)
		options = append(options, label)
		byOption[label] = update
	}

	selected, err := selectMultiple("Select skills to update", options, options[1:])
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, nil
	}

	for _, item := range selected {
		if item == toggleAllOption {
			return updates, nil
		}
	}

	result := make([]pendingSkillUpdate, 0, len(selected))
	for _, item := range selected {
		update, ok := byOption[item]
		if !ok {
			continue
		}
		result = append(result, update)
	}
	return result, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func diffSkill(current skills.Skill, bundle remote.SkillBundle) (string, error) {
	existingFiles := map[string]string{}
	err := filepath.Walk(current.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(current.Dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		existingFiles[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		return "", err
	}

	newFiles := map[string]string{}
	for _, file := range bundle.Files {
		newFiles[filepath.ToSlash(file.Path)] = file.Contents
	}

	pathsMap := map[string]struct{}{}
	for path := range existingFiles {
		pathsMap[path] = struct{}{}
	}
	for path := range newFiles {
		pathsMap[path] = struct{}{}
	}

	paths := make([]string, 0, len(pathsMap))
	for path := range pathsMap {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var builder strings.Builder
	for _, path := range paths {
		before, beforeOK := existingFiles[path]
		after, afterOK := newFiles[path]
		if beforeOK && afterOK && before == after {
			continue
		}
		switch {
		case !beforeOK:
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(""),
				B:        difflib.SplitLines(after),
				FromFile: path,
				ToFile:   path,
				Context:  3,
			})
			if err != nil {
				return "", err
			}
			builder.WriteString(diff)
		case !afterOK:
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(before),
				B:        difflib.SplitLines(""),
				FromFile: path,
				ToFile:   path,
				Context:  3,
			})
			if err != nil {
				return "", err
			}
			builder.WriteString(diff)
		default:
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(before),
				B:        difflib.SplitLines(after),
				FromFile: path,
				ToFile:   path,
				Context:  3,
			})
			if err != nil {
				return "", err
			}
			builder.WriteString(diff)
		}
		if !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String()), nil
}
