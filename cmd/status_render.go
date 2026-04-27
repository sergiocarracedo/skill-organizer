package cmd

import (
	"sort"
	"strings"

	"github.com/pterm/pterm"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type statusTreeNode struct {
	text     string
	children map[string]*statusTreeNode
}

func printSyncResult(configFile string, result syncpkg.Result) {
	pterm.Success.Printfln("Synchronized project config: %s", configFile)
	pterm.Info.Printfln("Enabled skills: %d", len(result.Enabled))
	pterm.Info.Printfln("Disabled skills: %d", len(result.Disabled))
	pterm.Info.Printfln("Created links: %d", len(result.Created))
	pterm.Info.Printfln("Updated links: %d", len(result.Updated))
	pterm.Info.Printfln("Removed stale links: %d", len(result.Removed))
}

func printStatusReport(configFile string, location configpkg.Location, report statuspkg.Report) error {
	pterm.DefaultSection.Println("Project")
	pterm.Println("Config: " + configFile)
	pterm.Println("Source: " + location.Source)
	pterm.Println("Target: " + location.Target)

	pterm.DefaultSection.Println("Skills")
	if len(report.Skills) == 0 {
		pterm.Println("None")
	} else {
		if err := pterm.DefaultTree.WithRoot(buildStatusTree(report.Skills)).Render(); err != nil {
			return err
		}
	}

	pterm.DefaultSection.Println("Unmanaged target entries")
	if len(report.Unmanaged) == 0 {
		pterm.Println("None")
	} else {
		for _, name := range report.Unmanaged {
			pterm.Println(name)
		}
	}

	printStatusSummary(report)

	return nil
}

func printStatusSummary(report statuspkg.Report) {
	summary := report.Summary()
	rows := []summaryRow{
		{label: "Total skills", count: summary.TotalSkills, color: pterm.FgWhite},
		{label: "Managed skills", count: summary.ManagedSkills, color: pterm.FgWhite},
		{label: "Unmanaged skills", count: summary.UnmanagedSkills, color: pterm.FgWhite},
		{label: "Synced", count: summary.Synced, color: statusColor(statuspkg.StateSynced)},
		{label: "Disabled", count: summary.Disabled, color: statusColor(statuspkg.StateDisabled)},
		{label: "Missing target", count: summary.MissingTarget, color: statusColor(statuspkg.StateMissingTarget)},
		{label: "Broken link", count: summary.BrokenLink, color: statusColor(statuspkg.StateBrokenLink)},
		{label: "Drifted", count: summary.Drifted, color: statusColor(statuspkg.StateDrifted)},
	}
	labelWidth := maxSummaryLabelWidth(rows)

	pterm.DefaultSection.Println("Summary")
	for _, row := range rows {
		pterm.Println(formatSummaryLine(row, labelWidth))
	}
}

func formatCount(value int) string {
	return pterm.NewStyle(pterm.Bold).Sprint(value)
}

type summaryRow struct {
	label string
	count int
	color pterm.Color
}

func formatSummaryLine(row summaryRow, labelWidth int) string {
	label := pterm.NewStyle(row.color, pterm.Bold).Sprint(strpad(row.label+":", labelWidth+1))
	count := pterm.NewStyle(row.color, pterm.Bold).Sprint(row.count)
	return label + " " + count
}

func maxSummaryLabelWidth(rows []summaryRow) int {
	maxWidth := 0
	for _, row := range rows {
		if len(row.label) > maxWidth {
			maxWidth = len(row.label)
		}
	}
	return maxWidth
}

func strpad(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func buildStatusTree(entries []statuspkg.SkillStatus) pterm.TreeNode {
	root := &statusTreeNode{children: map[string]*statusTreeNode{}}

	for _, entry := range entries {
		current := root
		parts := strings.Split(entry.Skill.RelativePath, "/")
		for _, part := range parts[:len(parts)-1] {
			child, ok := current.children[part]
			if !ok {
				child = &statusTreeNode{text: formatFolderName(part), children: map[string]*statusTreeNode{}}
				current.children[part] = child
			}
			current = child
		}

		leafName := parts[len(parts)-1]
		current.children[leafName] = &statusTreeNode{text: formatSkillStatus(entry, leafName)}
	}

	return toPTermTree(root)
}

func toPTermTree(node *statusTreeNode) pterm.TreeNode {
	result := pterm.TreeNode{Text: node.text}
	if len(node.children) == 0 {
		return result
	}

	keys := make([]string, 0, len(node.children))
	for key := range node.children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result.Children = make([]pterm.TreeNode, 0, len(keys))
	for _, key := range keys {
		result.Children = append(result.Children, toPTermTree(node.children[key]))
	}

	return result
}

func formatSkillStatus(entry statuspkg.SkillStatus, sourceLeaf string) string {
	state := pterm.NewStyle(statusColor(entry.State), pterm.Bold).Sprint("[" + string(entry.State) + "]")
	flattened := pterm.NewStyle(pterm.FgDarkGray).Sprint(entry.Skill.FlattenedName)
	return sourceLeaf + " -> " + flattened + " " + state
}

func formatFolderName(name string) string {
	return pterm.NewStyle(pterm.FgCyan, pterm.Bold).Sprint(name)
}

func statusColor(state statuspkg.SkillState) pterm.Color {
	switch state {
	case statuspkg.StateSynced:
		return pterm.FgGreen
	case statuspkg.StateDisabled:
		return pterm.FgWhite
	case statuspkg.StateMissingTarget:
		return pterm.FgYellow
	case statuspkg.StateBrokenLink, statuspkg.StateDrifted:
		return pterm.FgRed
	default:
		return pterm.FgWhite
	}
}
