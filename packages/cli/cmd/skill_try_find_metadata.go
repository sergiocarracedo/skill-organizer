package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type metadataRepairResult struct {
	Skill  skills.Skill
	Source string
	Name   string
	Reason string
	Found  bool
}

var (
	findSkillMatchesFunc              = remotepkg.FindSkills
	tryFindMetadataFetchBundleFunc    = remotepkg.FetchSkillBundle
	tryFindMetadataLatestModTimeFunc  = remotepkg.LatestFileModTime
	tryFindMetadataRewriteFieldsFunc  = skills.RewriteManagedFieldsWithMetadata
	tryFindMetadataLoadResolvedConfig = loadResolvedLocation
	tryFindMetadataRunSync            = syncpkg.Run
	startTryFindMetadataSpinner       = startDefaultSpinner
)

func newTryFindMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "try-find-metadata",
		Short: "Try to identify missing upstream metadata for managed skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, location, err := tryFindMetadataLoadResolvedConfig()
			if err != nil {
				return err
			}

			scanned, err := skills.ScanSource(location.Source)
			if err != nil {
				return err
			}

			candidates := make([]skills.Skill, 0)
			for _, skill := range scanned {
				doc, loadErr := skills.LoadDocument(skill.SkillFile)
				if loadErr != nil {
					return loadErr
				}
				if updateSkipReason(doc.ManagedMetadata()) == "" {
					continue
				}
				candidates = append(candidates, skill)
			}
			if len(candidates) == 0 {
				pterm.Info.Println("No managed skills with missing update metadata found")
				return nil
			}

			spinner, err := startTryFindMetadataSpinner("Trying to find missing skill metadata (checked 0/0). Found: 0. Skipped: 0")
			if err != nil {
				return err
			}

			results := make([]metadataRepairResult, 0, len(candidates))
			updatedCount := 0
			skippedCount := 0
			for _, skill := range candidates {
				spinner.UpdateText(renderTryFindMetadataProgressText("checking", len(results)+1, len(candidates), updatedCount, skippedCount))
				pterm.Info.Printfln("Checking %s", skill.RelativePath)
				result, resolveErr := tryRepairSkillMetadata(cmd.Context(), skill)
				if resolveErr != nil {
					spinner.Fail("Trying to find missing skill metadata failed")
					return resolveErr
				}
				results = append(results, result)
				if result.Found {
					updatedCount++
					spinner.UpdateText(renderTryFindMetadataProgressText("checked", len(results), len(candidates), updatedCount, skippedCount))
					pterm.Success.Printfln("Updated metadata for %s using %s@%s", result.Skill.RelativePath, result.Source, result.Name)
					continue
				}
				skippedCount++
				spinner.UpdateText(renderTryFindMetadataProgressText("checked", len(results), len(candidates), updatedCount, skippedCount))
				pterm.Warning.Printfln("Skipped %s: %s", result.Skill.RelativePath, result.Reason)
			}
			spinner.Success(fmt.Sprintf("Checked %d skills. Found: %d. Skipped: %d", len(candidates), updatedCount, skippedCount))
			if updatedCount == 0 {
				pterm.Info.Println("No missing metadata could be resolved")
				return nil
			}

			result, err := tryFindMetadataRunSync(location)
			if err != nil {
				return err
			}
			printSyncResult(configFile, result)
			return nil
		},
	}
	return cmd
}

func tryRepairSkillMetadata(ctx context.Context, skill skills.Skill) (metadataRepairResult, error) {
	doc, err := skills.LoadDocument(skill.SkillFile)
	if err != nil {
		return metadataRepairResult{}, err
	}
	metadata := doc.ManagedMetadata()
	match, reason, err := findBestMetadataMatch(ctx, skill, doc, metadata)
	if err != nil {
		return metadataRepairResult{}, err
	}
	if reason != "" {
		return metadataRepairResult{Skill: skill, Reason: reason}, nil
	}
	modTime, _ := tryFindMetadataLatestModTimeFunc(match.Bundle.Root)
	updated := metadata
	if strings.TrimSpace(updated.OriginalName) == "" {
		updated.OriginalName = match.Result.Skill.Name
	}
	resolvedSource := strings.TrimSpace(match.Bundle.Skill.Source)
	if resolvedSource == "" {
		resolvedSource = match.Result.Skill.Source
	}
	resolvedSource = normalizeSkillUpdateSource(resolvedSource)
	if strings.TrimSpace(updated.Source) == "" {
		updated.Source = resolvedSource
	}
	resolvedSourceType := strings.TrimSpace(match.Bundle.Skill.SourceType)
	if resolvedSourceType == "" {
		resolvedSourceType = match.Result.Skill.SourceType
	}
	if strings.TrimSpace(updated.SourceType) == "" {
		updated.SourceType = resolvedSourceType
	}
	resolvedRepoPath := strings.TrimSpace(match.Bundle.Skill.RepoSkillPath)
	if resolvedRepoPath == "" {
		resolvedRepoPath = match.Result.Skill.RepoSkillPath
	}
	if strings.TrimSpace(updated.RepoSkillPath) == "" {
		updated.RepoSkillPath = resolvedRepoPath
	}
	if strings.TrimSpace(updated.InstalledVersion) == "" {
		updated.InstalledVersion = resolvedBundleVersion(match.Bundle)
	}
	if strings.TrimSpace(updated.LastUpdatedAt) == "" {
		updated.LastUpdatedAt = formatOptionalTime(modTime)
	}
	if err := tryFindMetadataRewriteFieldsFunc(skill, true, metadata.Disabled, updated); err != nil {
		return metadataRepairResult{}, err
	}
	return metadataRepairResult{
		Skill:  skill,
		Source: match.Result.Skill.Source,
		Name:   match.Result.Skill.Name,
		Found:  true,
	}, nil
}

type metadataMatch struct {
	Result remotepkg.SkillSearchResult
	Bundle remotepkg.SkillBundle
	Score  int
}

func findBestMetadataMatch(ctx context.Context, skill skills.Skill, doc skills.Document, metadata skills.ManagedMetadata) (metadataMatch, string, error) {
	queries := metadataSearchQueries(skill, doc, metadata)
	seen := make(map[string]struct{})
	matches := make([]metadataMatch, 0)
	for _, query := range queries {
		results, err := findSkillMatchesFunc(ctx, query)
		if err != nil {
			continue
		}
		for _, result := range results {
			key := result.Skill.Source + "@" + result.Skill.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			bundle, fetchErr := tryFindMetadataFetchBundleFunc(ctx, result.Skill.Source, result.Skill.Name)
			if fetchErr != nil {
				continue
			}
			result.Skill.RepoSkillPath = strings.TrimSpace(bundle.Skill.RepoSkillPath)
			result.Skill.Source = normalizeSkillUpdateSource(result.Skill.Source)
			result.Skill.SourceType = strings.TrimSpace(bundle.Skill.SourceType)
			if result.Skill.SourceType == "" {
				result.Skill.SourceType = "github"
			}
			score := scoreMetadataMatch(skill, doc, metadata, result, bundle)
			if score <= 0 {
				continue
			}
			matches = append(matches, metadataMatch{Result: result, Bundle: bundle, Score: score})
		}
	}
	if len(matches) == 0 {
		return metadataMatch{}, "could not identify upstream source", nil
	}
	if exact, ok := resolveExactSkillContentMatch(doc, matches); ok {
		return exact, "", nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			left := matches[i].Result.Skill.Source + "@" + matches[i].Result.Skill.Name
			right := matches[j].Result.Skill.Source + "@" + matches[j].Result.Skill.Name
			return left < right
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > 1 && matches[0].Score == matches[1].Score {
		return metadataMatch{}, "multiple upstream matches looked equally plausible", nil
	}
	return matches[0], "", nil
}

func resolveExactSkillContentMatch(localDoc skills.Document, matches []metadataMatch) (metadataMatch, bool) {
	localComparable, err := comparableSkillDocument(localDoc)
	if err != nil {
		return metadataMatch{}, false
	}
	exact := make([]metadataMatch, 0, len(matches))
	for _, match := range matches {
		remoteDoc, parseErr := skills.ParseDocument([]byte(skillFileContents(match.Bundle.Files)))
		if parseErr != nil {
			continue
		}
		remoteComparable, compareErr := comparableSkillDocument(remoteDoc)
		if compareErr != nil {
			continue
		}
		if localComparable != remoteComparable {
			continue
		}
		exact = append(exact, match)
	}
	if len(exact) != 1 {
		return metadataMatch{}, false
	}
	return exact[0], true
}

func metadataSearchQueries(skill skills.Skill, doc skills.Document, metadata skills.ManagedMetadata) []string {
	queries := []string{
		strings.TrimSpace(metadata.OriginalName),
		strings.TrimSpace(doc.Name()),
		filepath.Base(skill.RelativePath),
	}
	seen := make(map[string]struct{}, len(queries))
	filtered := make([]string, 0, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if _, ok := seen[query]; ok {
			continue
		}
		seen[query] = struct{}{}
		filtered = append(filtered, query)
	}
	return filtered
}

func scoreMetadataMatch(skill skills.Skill, doc skills.Document, metadata skills.ManagedMetadata, result remotepkg.SkillSearchResult, bundle remotepkg.SkillBundle) int {
	remoteDoc, err := skills.ParseDocument([]byte(skillFileContents(bundle.Files)))
	if err != nil {
		return 0
	}
	localBody := normalizeComparableText(doc.Body())
	remoteBody := normalizeComparableText(remoteDoc.Body())
	localName := normalizeComparableToken(doc.Name())
	remoteName := normalizeComparableToken(remoteDoc.Name())
	folderName := normalizeComparableToken(filepath.Base(skill.RelativePath))
	originalName := normalizeComparableToken(metadata.OriginalName)
	resultName := normalizeComparableToken(result.Skill.Name)
	score := 0
	if resultName != "" && resultName == originalName {
		score += 6
	}
	if resultName != "" && resultName == folderName {
		score += 5
	}
	if remoteName != "" && remoteName == localName {
		score += 6
	}
	if remoteName != "" && remoteName == resultName {
		score += 4
	}
	if localBody != "" && remoteBody != "" && localBody == remoteBody {
		score += 10
	}
	if strings.TrimSpace(bundle.Skill.RepoSkillPath) != "" {
		score += 3
	}
	if resolvedBundleVersion(bundle) != "" {
		score += 2
	}
	if score < 8 {
		return 0
	}
	return score
}

func resolvedBundleVersion(bundle remotepkg.SkillBundle) string {
	if version := strings.TrimSpace(remotepkg.SkillVersionFromSkillFile(skillFileContents(bundle.Files))); version != "" {
		return version
	}
	for _, file := range bundle.Files {
		if file.Path != "metadata.json" {
			continue
		}
		if version := strings.TrimSpace(remotepkg.SkillVersionFromMetadataFile(file.Contents)); version != "" {
			return version
		}
	}
	return strings.TrimSpace(remotepkg.ResolveVersion(bundle.Skill, bundle.Files))
}

func normalizeComparableToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "--", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func normalizeComparableText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSpace(strings.ToLower(value))
	return value
}

func comparableSkillDocument(doc skills.Document) (string, error) {
	content, err := doc.WithoutManagedMetadata().Marshal()
	if err != nil {
		return "", err
	}
	return normalizeComparableText(string(content)), nil
}

func renderTryFindMetadataProgressText(prefix string, checked int, total int, found int, skipped int) string {
	return fmt.Sprintf("Trying to find missing skill metadata (%s %d/%d). Found: %s. Skipped: %s", styledProgressState(prefix), checked, total, styledProgressCount(found), styledSkippedCount(skipped))
}
