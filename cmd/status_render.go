package cmd

import (
	"os"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
	syncpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/sync"
)

type statusTreeNode struct {
	text     string
	children map[string]*statusTreeNode
}

type statusTreeLine struct {
	left   string
	status string
}

const (
	statusLineWidthFallback = 96
	statusTreeIndent        = 2
)

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
		printStatusTree(report.Skills)
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
	pterm.DefaultSection.Println("Summary")
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, formatSummaryChip(row))
	}
	pterm.Println(strings.Join(parts, "  "))
}

func formatCount(value int) string {
	return pterm.NewStyle(pterm.Bold).Sprint(value)
}

type summaryRow struct {
	label string
	count int
	color pterm.Color
}

func formatSummaryChip(row summaryRow) string {
	label := pterm.NewStyle(row.color, pterm.Bold).Sprint(row.label)
	count := pterm.NewStyle(row.color, pterm.Bold).Sprint(row.count)
	return label + ":" + count
}

func printStatusTree(entries []statuspkg.SkillStatus) {
	lines := buildStatusTreeLines(entries)
	leftWidth, _ := maxStatusTreeColumnWidths(lines)

	for _, line := range lines {
		padding := leftWidth - visibleRuneWidth(stripANSI(line.left)) + 2
		if padding < 2 {
			padding = 2
		}

		row := line.left
		if line.status == "" {
			pterm.Println(row)
			continue
		}

		row += strings.Repeat(" ", padding) + line.status
		pterm.Println(row)
	}
}

func buildStatusTreeLines(entries []statuspkg.SkillStatus) []statusTreeLine {
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
		current.children[leafName] = &statusTreeNode{text: formatSkillLabel(entry, leafName)}
	}

	return flattenStatusTree(root)
}

func flattenStatusTree(node *statusTreeNode) []statusTreeLine {
	if len(node.children) == 0 {
		return nil
	}

	return flattenStatusTreeChildren(node.children, "")
}

func flattenStatusTreeChildren(children map[string]*statusTreeNode, prefix string) []statusTreeLine {
	keys := make([]string, 0, len(children))
	for key := range children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]statusTreeLine, 0, len(keys))
	for index, key := range keys {
		child := children[key]
		isLast := index == len(keys)-1
		connector := pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.TopRightDownString)
		childPrefix := prefix + pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.VerticalString) + strings.Repeat(" ", statusTreeIndent-1)
		if isLast {
			connector = pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.TopRightCornerString)
			childPrefix = prefix + strings.Repeat(" ", statusTreeIndent)
		}

		branch := connector
		if len(child.children) == 0 {
			branch += strings.Repeat(pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.HorizontalString), statusTreeIndent)
			lines = append(lines, statusTreeLine{
				left:   prefix + branch + child.text,
				status: "",
			})
			continue
		}

		branch += strings.Repeat(pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.HorizontalString), statusTreeIndent-1)
		branch += pterm.ThemeDefault.TreeStyle.Sprint(pterm.DefaultTree.RightDownLeftString)
		lines = append(lines, statusTreeLine{
			left: prefix + branch + child.text,
		})
		lines = append(lines, flattenStatusTreeChildren(child.children, childPrefix)...)
	}

	return lines
}

func formatSkillLabel(entry statuspkg.SkillStatus, sourceLeaf string) string {
	flattened := pterm.NewStyle(pterm.FgDarkGray).Sprint(entry.Skill.FlattenedName)
	state := pterm.NewStyle(statusColor(entry.State), pterm.Bold).Sprint("[" + string(entry.State) + "]")
	return sourceLeaf + " -> " + flattened + statusColumnSeparator() + state
}

func statusLineWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return statusLineWidthFallback
	}
	return width
}

func maxStatusTreeColumnWidths(lines []statusTreeLine) (int, int) {
	leftWidth := 0
	statusWidth := 0
	separator := statusColumnSeparator()

	for index, line := range lines {
		parts := strings.Split(line.left, separator)
		if len(parts) == 2 {
			lines[index].left = parts[0]
			lines[index].status = parts[1]
			line.left = parts[0]
			line.status = parts[1]
		}

		if width := visibleRuneWidth(stripANSI(line.left)); width > leftWidth {
			leftWidth = width
		}
		if width := visibleRuneWidth(stripANSI(line.status)); width > statusWidth {
			statusWidth = width
		}
	}

	return leftWidth, statusWidth
}

func statusColumnSeparator() string {
	return "\x00status\x00"
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

func stripANSI(value string) string {
	var builder strings.Builder
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		builder.WriteByte(ch)
	}
	return builder.String()
}
