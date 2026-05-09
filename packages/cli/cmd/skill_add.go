package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

func newSkillAddCommand() *cobra.Command {
	var skillNames []string

	cmd := &cobra.Command{
		Use:   "add <source>",
		Short: "Add skills from skills.sh into the managed source tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			runner, err := remotepkg.DetectSkillsCLI()
			if err != nil {
				return err
			}
			pterm.Info.Println("Using skills.sh cli tool to add the skills")
			pterm.Info.Printfln("Detected CLI: %s", runner.Label())

			source := strings.TrimSpace(args[0])
			selected := append([]string{}, skillNames...)
			if len(selected) == 0 {
				sandbox, err := remotepkg.NewSandbox()
				if err != nil {
					return err
				}
				defer sandbox.Close()
				available, err := sandbox.ListRepoSkills(source)
				if err != nil {
					return err
				}
				options := make([]string, 0, len(available))
				for _, item := range available {
					options = append(options, item.Name)
				}
				selected, err = selectMultiple("Select the skills to add", options, options)
				if err != nil {
					return err
				}
			}
			if len(selected) == 0 {
				return fmt.Errorf("no skills selected")
			}

			suggestions, err := sourceFolderSuggestions(location.Source)
			if err != nil {
				return err
			}
			for _, name := range selected {
				bundle, err := remotepkg.FetchSkillBundle(source, name)
				if err != nil {
					return err
				}

				defaultRelative := filepath.ToSlash(name)
				relative, err := promptTextWithSuggestionsBelow(fmt.Sprintf("Select the target folder for %s relative to skills-organized", name), defaultRelative, suggestions)
				if err != nil {
					return err
				}
				targetSkill, err := skills.ResolveSourceSkillTarget(location.Source, relative)
				if err != nil {
					return err
				}
				if err := writeImportedBundle(targetSkill.Dir, bundle); err != nil {
					return err
				}
				modTime, _ := remotepkg.LatestFileModTime(bundle.Root)
				if err := skills.RewriteManagedFieldsWithMetadata(targetSkill, true, false, skills.ManagedMetadata{
					Source:           bundle.Skill.Source,
					SourceType:       bundle.Skill.SourceType,
					InstalledVersion: remotepkg.ResolveVersion(bundle.Skill, bundle.Files),
					InstalledAt:      time.Now().UTC().Format(time.RFC3339),
					RepoSkillPath:    bundle.Skill.RepoSkillPath,
					LastUpdatedAt:    formatOptionalTime(modTime),
				}); err != nil {
					return err
				}
				pterm.Success.Printfln("Imported skill: %s -> %s", name, targetSkill.RelativePath)
			}

			result, err := syncpkg.Run(location)
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
