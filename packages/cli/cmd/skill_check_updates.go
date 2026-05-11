package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	backuppkg "github.com/sergiocarracedo/skill-organizer/cli/internal/backup"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	remotepkg "github.com/sergiocarracedo/skill-organizer/cli/internal/remote"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
	versionfmtpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/versionfmt"
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

type skillUpdateCheckFailure struct {
	RelativePath string
	Reason       string
}

type skillUpdateSkip struct {
	RelativePath string
	Reason       string
}

type skillUpdateScanResult struct {
	Candidates []skillUpdateCandidate
	Checked    int
	Failures   []skillUpdateCheckFailure
	Skipped    []skillUpdateSkip
}

var (
	fetchSkillBundleFunc     = remotepkg.FetchSkillBundle
	updatesPathFunc          = configpkg.UpdatesPath
	startCheckUpdatesSpinner = startDefaultSpinner
	printCheckUpdatesWarning = func(format string, args ...any) {
		pterm.Warning.Printfln(format, args...)
	}
)

func newCheckUpdatesCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "check-updates",
		Aliases: []string{"updates", "upgrade"},
		Short:   "Check imported skills for upstream updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configFile, location, err := loadResolvedLocation()
			if err != nil {
				return err
			}

			spinner, err := startCheckUpdatesSpinner("Checking for skill updates (checked 0/0). Found: 0. Skipped: 0")
			if err != nil {
				return err
			}
			progressStep := func(prefix string, checked int, total int, found int, skipped int, relativePath string) {
				spinner.UpdateText(renderCheckUpdatesProgressText(prefix, checked, total, found, skipped, relativePath))
			}
			scan, err := collectUpdateCandidates(cmd.Context(), location, func(checked int, total int, found int, skipped int) {
				progressStep("checked", checked, total, found, skipped, "")
			}, func(checked int, total int, found int, skipped int, relativePath string) {
				progressStep("checking", checked+1, total, found, skipped, relativePath)
			})
			if err != nil {
				spinner.Fail("Checking for skill updates failed")
				return err
			}
			spinner.Success(fmt.Sprintf("Checked %d skills. Found: %d. Skipped: %d", scan.Checked, len(scan.Candidates), len(scan.Skipped)))
			for _, failure := range scan.Failures {
				printCheckUpdatesWarning("Could not check updates for %s: %s", failure.RelativePath, failure.Reason)
			}
			for _, skipped := range scan.Skipped {
				printCheckUpdatesWarning("Skipped %s: %s", skipped.RelativePath, skipped.Reason)
			}
			if len(scan.Skipped) > 0 {
				pterm.Info.Println("Tip: use skill-organizer skill add to import skills with tracked source/version metadata.")
				pterm.Info.Println("Tip: run skill-organizer skill try-find-metadata to try recovering missing metadata for older imports.")
			}

			candidates := scan.Candidates
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
				if err := selector.Run(cmd.Context()); err != nil {
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
				pterm.Info.Printfln("Updating %s", item.Skill.RelativePath)
				pterm.Success.Printfln("Updated skill: %s (%s -> %s)", item.Skill.RelativePath, item.Installed, item.Latest)
				pterm.Info.Printfln("Backup: %s", backupPath)
			}
			pterm.Info.Printfln("Updated skills: %d", len(selected))

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

func collectUpdateCandidates(ctx context.Context, location configpkg.Location, progress func(checked int, total int, found int, skipped int), active func(checked int, total int, found int, skipped int, relativePath string)) (skillUpdateScanResult, error) {
	scanned, err := skills.ScanSource(location.Source)
	if err != nil {
		return skillUpdateScanResult{}, err
	}
	results := make([]skillUpdateCandidate, 0)
	failures := make([]skillUpdateCheckFailure, 0)
	skipped := make([]skillUpdateSkip, 0)
	total := len(scanned)
	checked := 0
	if progress != nil {
		progress(0, total, 0, 0)
	}
	for _, skill := range scanned {
		if err := ctx.Err(); err != nil {
			return skillUpdateScanResult{}, err
		}
		if active != nil {
			active(checked, total, len(results), len(skipped), skill.RelativePath)
		}
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return skillUpdateScanResult{}, err
		}
		metadata := doc.ManagedMetadata()
		skipReason := updateSkipReason(metadata)
		if skipReason != "" {
			skipped = append(skipped, skillUpdateSkip{RelativePath: skill.RelativePath, Reason: skipReason})
			checked++
			if progress != nil {
				progress(checked, total, len(results), len(skipped))
			}
			continue
		}
		upstreamName := strings.TrimSpace(metadata.OriginalName)
		if upstreamName == "" {
			upstreamName = strings.TrimSpace(doc.Name())
		}
		normalizedSource := normalizeSkillUpdateSource(metadata.Source)
		bundle, err := fetchSkillBundleFunc(ctx, normalizedSource, upstreamName)
		if err != nil {
			failures = append(failures, skillUpdateCheckFailure{RelativePath: skill.RelativePath, Reason: err.Error()})
			checked++
			if progress != nil {
				progress(checked, total, len(results), len(skipped))
			}
			continue
		}
		installedBundle, err := remotepkg.LoadBundleFromDir(skill.Dir)
		if err != nil {
			return skillUpdateScanResult{}, err
		}
		installed := resolveInstalledSkillVersion(metadata, installedBundle)
		latest := resolveLatestSkillVersion(bundle)
		if installed == "" || latest == "" || installed == latest {
			checked++
			if progress != nil {
				progress(checked, total, len(results), len(skipped))
			}
			continue
		}
		results = append(results, skillUpdateCandidate{
			Skill:          skill,
			Metadata:       metadata,
			InstalledPath:  skill.Dir,
			Installed:      installed,
			Latest:         latest,
			Source:         normalizedSource,
			Bundle:         bundle,
			InstalledFiles: installedBundle,
		})
		checked++
		if progress != nil {
			progress(checked, total, len(results), len(skipped))
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Skill.RelativePath < results[j].Skill.RelativePath })
	sort.Slice(skipped, func(i, j int) bool { return skipped[i].RelativePath < skipped[j].RelativePath })
	return skillUpdateScanResult{Candidates: results, Checked: checked, Failures: failures, Skipped: skipped}, nil
}

func resolveInstalledSkillVersion(metadata skills.ManagedMetadata, installedBundle remotepkg.SkillBundle) string {
	if version := strings.TrimSpace(remotepkg.SkillVersionFromSkillFile(skillFileContents(installedBundle.Files))); version != "" {
		return version
	}
	installed := strings.TrimSpace(metadata.InstalledVersion)
	if installed != "" {
		return installed
	}
	return strings.TrimSpace(remotepkg.ResolveVersion(installedBundle.Skill, installedBundle.Files))
}

func resolveLatestSkillVersion(bundle remotepkg.SkillBundle) string {
	if version := strings.TrimSpace(remotepkg.SkillVersionFromSkillFile(skillFileContents(bundle.Files))); version != "" {
		return version
	}
	return strings.TrimSpace(remotepkg.ResolveVersion(bundle.Skill, bundle.Files))
}

func skillFileContents(files []remotepkg.File) string {
	for _, file := range files {
		if file.Path == "SKILL.md" {
			return file.Contents
		}
	}
	return ""
}

func normalizeSkillUpdateSource(source string) string {
	trimmed := strings.TrimSpace(source)
	trimmed = strings.TrimSuffix(trimmed, ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	return strings.Trim(trimmed, "/")
}

func updateSkipReason(metadata skills.ManagedMetadata) string {
	if strings.TrimSpace(metadata.Source) == "" {
		return "missing metadata.skill-organizer.source"
	}
	return ""
}

type skillUpdateSelector struct {
	items           []skillUpdateCandidate
	selected        map[int]bool
	active          int
	lastRenderLines int
	diffActive      bool
	diffOffset      int
}

func newSkillUpdateSelector(items []skillUpdateCandidate) *skillUpdateSelector {
	selected := make(map[int]bool, len(items))
	for i := range items {
		selected[i] = true
	}
	return &skillUpdateSelector{items: items, selected: selected}
}

func (s *skillUpdateSelector) Run(ctx context.Context) error {
	defer showTerminalCursor()
	if _, err := fmt.Fprintln(os.Stdout, "Select skills to update. Press d to inspect the diff for the highlighted skill."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return err
	}
	s.render()
	stopInterruptForwarding := forwardContextInterrupt(ctx)
	defer stopInterruptForwarding()
	return keyboard.Listen(func(key keys.Key) (bool, error) {
		if s.diffActive {
			return s.handleDiffKey(key)
		}
		switch {
		case isInterruptKey(key):
			_, _ = fmt.Fprintln(os.Stdout)
			return true, fmt.Errorf("interrupted")
		case isConfirmKey(key):
			_, _ = fmt.Fprintln(os.Stdout)
			return true, nil
		case isUpKey(key):
			if s.active > 0 {
				s.active--
				s.render()
			}
			return false, nil
		case isDownKey(key):
			if s.active < len(s.items)-1 {
				s.active++
				s.render()
			}
			return false, nil
		case isToggleKey(key):
			s.selected[s.active] = !s.selected[s.active]
			s.render()
			return false, nil
		case isOpenDiffKey(key):
			s.openDiff()
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
	lines := []string{checkUpdatesHelpLine("Space: Toggle, 🡹/🡻: Move, d: Diff, Enter: Continue, Ctrl+C: Abort")}
	for i, item := range s.items {
		prefix := "  "
		if i == s.active {
			prefix = activeSelectorPrefix
		}
		marker := styledSelectionMarker(s.selected[i])
		line := fmt.Sprintf("%s%s %s [%s] -> [%s]", prefix, marker, item.Skill.RelativePath, displayVersion(item.Installed), displayVersion(item.Latest))
		if updatedAt := displayUpdateDate(item.Bundle.Skill.VersionDate); updatedAt != "" {
			line += " [updated " + updatedAt + "]"
		}
		line += " [" + item.Source + "]"
		lines = append(lines, line)
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

func (s *skillUpdateSelector) openDiff() {
	s.diffActive = true
	s.diffOffset = 0
	enterAlternateScreen()
	s.renderDiff()
}

func (s *skillUpdateSelector) closeDiff() {
	if !s.diffActive {
		return
	}
	s.diffActive = false
	s.diffOffset = 0
	exitAlternateScreen()
	s.render()
}

func (s *skillUpdateSelector) handleDiffKey(key keys.Key) (bool, error) {
	lines := s.diffLines()
	switch {
	case isUpKey(key):
		if s.diffOffset > 0 {
			s.diffOffset--
			s.renderDiff()
		}
		return false, nil
	case isDownKey(key):
		if s.diffOffset+diffContentHeight() < len(lines) {
			s.diffOffset++
			s.renderDiff()
		}
		return false, nil
	case isPageUpKey(key):
		if s.diffOffset > 0 {
			s.diffOffset -= diffContentHeight()
			if s.diffOffset < 0 {
				s.diffOffset = 0
			}
			s.renderDiff()
		}
		return false, nil
	case isPageDownKey(key):
		if s.diffOffset+diffContentHeight() < len(lines) {
			s.diffOffset += diffContentHeight()
			maxOffset := len(lines) - diffContentHeight()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if s.diffOffset > maxOffset {
				s.diffOffset = maxOffset
			}
			s.renderDiff()
		}
		return false, nil
	case isBackKey(key):
		s.closeDiff()
		return false, nil
	case isInterruptKey(key):
		s.closeDiff()
		_, _ = fmt.Fprintln(os.Stdout)
		return true, fmt.Errorf("interrupted")
	default:
		return false, nil
	}
}

func (s *skillUpdateSelector) diffLines() []string {
	item := s.items[s.active]
	lines := diffLines(item.InstalledFiles.Files, item.Bundle.Files)
	if len(lines) == 0 {
		return []string{"No textual diff available."}
	}
	return lines
}

func (s *skillUpdateSelector) renderDiff() {
	hideTerminalCursor()
	item := s.items[s.active]
	lines := s.diffLines()
	height := diffViewportHeight()
	contentHeight := height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	maxOffset := len(lines) - contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.diffOffset > maxOffset {
		s.diffOffset = maxOffset
	}
	max := s.diffOffset + contentHeight
	if max > len(lines) {
		max = len(lines)
	}

	fmt.Print("\033[H\033[2J")
	fmt.Println(diffHeaderLine(item))
	shown := 0
	for _, line := range lines[s.diffOffset:max] {
		fmt.Println(colorizeDiffLine(line))
		shown++
	}
	for shown < contentHeight {
		fmt.Println()
		shown++
	}
	fmt.Print(diffFooterLine())
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
		text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(oldContent),
			B:        difflib.SplitLines(newContent),
			FromFile: "a/" + path,
			ToFile:   "b/" + path,
			Context:  3,
		})
		if err != nil {
			continue
		}
		lines = append(lines, strings.Split(strings.TrimSuffix(text, "\n"), "\n")...)
		lines = append(lines, "")
	}
	return lines
}

func diffViewportHeight() int {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || height <= 0 {
		return 24
	}
	return height
}

func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return statusLineWidthFallback
	}
	return width
}

func diffContentHeight() int {
	height := diffViewportHeight() - 2
	if height < 1 {
		return 1
	}
	return height
}

func diffHeaderLine(item skillUpdateCandidate) string {
	return pterm.NewStyle(pterm.Bold, pterm.FgLightWhite).Sprint(
		fmt.Sprintf("Diff: %s [%s -> %s]", item.Skill.RelativePath, displayVersion(item.Installed), displayVersion(item.Latest)),
	)
}

func diffFooterLine() string {
	return pterm.NewStyle(pterm.BgDarkGray, pterm.FgLightWhite, pterm.Bold).Sprint(" " + checkUpdatesHelpLine("Esc: Back  🡹/🡻: Scroll  PgUp/PgDown: Page  Ctrl+C: Abort") + " ")
}

func colorizeDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		return pterm.NewStyle(pterm.FgYellow, pterm.Bold).Sprint(line)
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return pterm.NewStyle(pterm.FgCyan, pterm.Bold).Sprint(line)
	case strings.HasPrefix(line, "+"):
		return pterm.NewStyle(pterm.FgGreen).Sprint(line)
	case strings.HasPrefix(line, "-"):
		return pterm.NewStyle(pterm.FgRed).Sprint(line)
	default:
		return line
	}
}

func isInterruptKey(key keys.Key) bool {
	return key.Code == keys.CtrlC || key.Code == keys.Break || key.String() == "ctrl+c"
}

func isConfirmKey(key keys.Key) bool {
	return key.Code == keys.Enter
}

func isUpKey(key keys.Key) bool {
	return key.Code == keys.Up
}

func isDownKey(key keys.Key) bool {
	return key.Code == keys.Down
}

func isPageUpKey(key keys.Key) bool {
	return key.Code == keys.PgUp
}

func isPageDownKey(key keys.Key) bool {
	return key.Code == keys.PgDown
}

func isToggleKey(key keys.Key) bool {
	return key.Code == keys.Space || (key.Code == keys.RuneKey && len(key.Runes) == 1 && key.Runes[0] == ' ')
}

func isOpenDiffKey(key keys.Key) bool {
	return key.Code == keys.RuneKey && len(key.Runes) == 1 && (key.Runes[0] == 'd' || key.Runes[0] == 'D')
}

func isBackKey(key keys.Key) bool {
	return key.Code == keys.Escape || key.Code == keys.Esc
}

func checkUpdatesHelpLine(text string) string {
	baseStyle := pterm.NewStyle(pterm.FgDarkGray)
	keyStyle := pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	replacer := strings.NewReplacer(
		"🡹/🡻", keyStyle.Sprint("🡹/🡻"),
		"🡸/🡺", keyStyle.Sprint("🡸/🡺"),
		"PgUp/PgDown", keyStyle.Sprint("PgUp/PgDown"),
		"Space", keyStyle.Sprint("Space"),
		"Enter", keyStyle.Sprint("Enter"),
		"Esc", keyStyle.Sprint("Esc"),
		"Ctrl+C", keyStyle.Sprint("Ctrl+C"),
		"Tab", keyStyle.Sprint("Tab"),
		"Home/End", keyStyle.Sprint("Home/End"),
		"d", keyStyle.Sprint("d"),
	)
	return replacer.Replace(baseStyle.Sprint(text))
}

func enterAlternateScreen() {
	fmt.Print("\033[?1049h")
}

func exitAlternateScreen() {
	fmt.Print("\033[?1049l")
}

func styledProgressState(value string) string {
	return pterm.NewStyle(pterm.FgMagenta, pterm.Bold).Sprint(value)
}

func styledProgressCount(value int) string {
	return pterm.NewStyle(pterm.FgLightGreen, pterm.Bold).Sprint(fmt.Sprintf("%d", value))
}

func styledSkippedCount(value int) string {
	return pterm.NewStyle(pterm.FgYellow, pterm.Bold).Sprint(fmt.Sprintf("%d", value))
}

func styledProgressPath(value string) string {
	return pterm.NewStyle(pterm.FgLightMagenta).Sprint(value)
}

func renderCheckUpdatesProgressText(prefix string, checked int, total int, found int, skipped int, relativePath string) string {
	baseText := fmt.Sprintf("Checking for skill updates (%s %d/%d). Found: %d. Skipped: %d", prefix, checked, total, found, skipped)
	message := fmt.Sprintf("Checking for skill updates (%s %d/%d). Found: %s. Skipped: %s", styledProgressState(prefix), checked, total, styledProgressCount(found), styledSkippedCount(skipped))

	path := strings.Join(strings.Fields(strings.TrimSpace(relativePath)), " ")
	if path == "" {
		return message
	}

	maxWidth := terminalWidth() - 12
	if maxWidth < 20 {
		maxWidth = 20
	}
	prefixWidth := visibleRuneWidth(baseText + " - ")
	availablePathWidth := maxWidth - prefixWidth
	if availablePathWidth <= 0 {
		return message
	}

	return message + " - " + styledProgressPath(limitSpinnerText(path, availablePathWidth))
}

func forwardContextInterrupt(ctx context.Context) func() {
	if ctx == nil {
		return func() {}
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = keyboard.SimulateKeyPress(keys.CtrlC)
		case <-stopped:
		}
	}()
	return func() {
		close(stopped)
	}
}

func displayVersion(value string) string {
	return versionfmtpkg.DisplayVersion(value)
}

func displayUpdateDate(value time.Time) string {
	return versionfmtpkg.DisplayTime(value)
}

func refreshSkillUpdateCache(candidates []skillUpdateCandidate) error {
	updatesPath, err := updatesPathFunc()
	if err != nil {
		return err
	}
	state, err := configpkg.LoadUpdatesStateOrDefault(updatesPath)
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
	state.LastCheckedAt = now
	state.UpdateCount = len(candidates)
	state.Pending = pending
	if len(candidates) == 0 {
		state.LastRemindedAt = ""
	}
	return configpkg.SaveUpdatesState(updatesPath, state)
}
