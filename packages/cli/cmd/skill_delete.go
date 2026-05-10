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
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

var (
	skillDeleteLoadResolvedLocation = loadResolvedLocation
	skillDeleteConfirm              = confirm
	skillDeleteMoveToBackup         = backuppkg.MoveSkill
	skillDeleteRegistryPath         = configpkg.RegistryPath
	skillDeleteLoadBackupConfig     = configpkg.LoadBackupConfigOrDefault
	runSkillDeleteSync              = syncpkg.Run
)

func newSkillDeleteCommand() *cobra.Command {
	var yes bool
	var noBackup bool
	retentionDays := skillDeleteRetentionDays()

	cmd := &cobra.Command{
		Use:     "delete <skill-path-or-pattern>",
		Aliases: []string{"remove", "rm"},
		Short:   fmt.Sprintf("Delete a managed source skill (it will keep it for %d days before the final deletion)", retentionDays),
		Long: fmt.Sprintf("Delete a managed source skill. Deleted skills are kept in the .old backup area for %d days before final deletion.\n\n"+
			"The skill path or pattern is relative to the skills-organized folder. Quote wildcard patterns so your shell does not expand them before the command runs.", retentionDays),
		Example: strings.Join([]string{
			"skill-organizer delete google/gws-admin-reports",
			"skill-organizer delete \"google/*\"",
			"skill-organizer delete \"google/*\" --no-backup",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := skillDeleteLoadResolvedLocation()
			if err != nil {
				return err
			}

			matched, err := resolveSkillsForDelete(location.Source, args[0])
			if err != nil {
				return err
			}
			sort.Slice(matched, func(i, j int) bool { return matched[i].RelativePath < matched[j].RelativePath })

			if !yes {
				pterm.Info.Println("Skills to delete:")
				for _, skill := range matched {
					pterm.Println("  " + skill.RelativePath)
				}
				prompt := fmt.Sprintf("Do you want to delete %s?", matched[0].RelativePath)
				if len(matched) > 1 {
					prompt = fmt.Sprintf("Do you want to delete these %d skills?", len(matched))
				}
				ok, err := skillDeleteConfirm(prompt, false)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}

			backupPaths := make([]string, 0, len(matched))
			for _, skill := range matched {
				if noBackup {
					if err := os.RemoveAll(skill.Dir); err != nil {
						return fmt.Errorf("delete skill %s: %w", skill.RelativePath, err)
					}
					continue
				}
				backupPath, err := skillDeleteMoveToBackup(skill.Dir, skill.FlattenedName, backuppkg.Metadata{
					OriginalPath:  skill.RelativePath,
					FlattenedName: skill.FlattenedName,
					DeletedAt:     time.Now().UTC().Format(time.RFC3339),
				}, time.Now().UTC())
				if err != nil {
					return err
				}
				backupPaths = append(backupPaths, backupPath)
			}

			result, err := runSkillDeleteSync(location)
			if err != nil {
				return err
			}

			if noBackup {
				pterm.Success.Printfln("Deleted skills: %d", len(matched))
			} else {
				registryPath, err := skillDeleteRegistryPath()
				if err == nil {
					backupCfg, cfgErr := skillDeleteLoadBackupConfig(registryPath)
					if cfgErr == nil {
						for _, backupPath := range backupPaths {
							pterm.Info.Printfln("Backup: %s (kept for %d days)", backupPath, backupCfg.RetentionDays)
						}
					}
				}
				pterm.Success.Printfln("Deleted skills: %d", len(matched))
			}
			printSyncResult(configFile, result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Move the skill to backup without confirmation")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Delete the skill without moving it to backup")
	return cmd
}

func resolveSkillsForDelete(sourceRoot string, input string) ([]skills.Skill, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("skill path cannot be empty")
	}
	if !strings.ContainsAny(trimmed, "*?[") {
		skill, err := skills.ResolveSourceSkill(sourceRoot, trimmed)
		if err != nil {
			return nil, err
		}
		return []skills.Skill{skill}, nil
	}
	items, err := skills.ScanSource(sourceRoot)
	if err != nil {
		return nil, err
	}
	matches := make([]skills.Skill, 0)
	pattern := filepath.ToSlash(trimmed)
	for _, item := range items {
		ok, matchErr := filepath.Match(pattern, item.RelativePath)
		if matchErr != nil {
			return nil, fmt.Errorf("invalid delete pattern %q: %w", trimmed, matchErr)
		}
		if ok {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no skills matched %q", trimmed)
	}
	return matches, nil
}

func skillDeleteRetentionDays() int {
	registryPath, err := skillDeleteRegistryPath()
	if err != nil {
		return configpkg.DefaultBackupRetentionDays
	}
	backupCfg, err := skillDeleteLoadBackupConfig(registryPath)
	if err != nil {
		return configpkg.DefaultBackupRetentionDays
	}
	if backupCfg.RetentionDays <= 0 {
		return configpkg.DefaultBackupRetentionDays
	}
	return backupCfg.RetentionDays
}
