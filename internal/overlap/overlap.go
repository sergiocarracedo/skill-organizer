package overlap

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools"
	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"
)

type SkillInfo struct {
	FlattenedName string
	RelativePath  string
	Name          string
	Description   string
	Disabled      bool
}

var commandRunner = runCommand

func CollectSkills(location configpkg.Location, includeDisabled bool) ([]SkillInfo, error) {
	if err := location.Validate(); err != nil {
		return nil, err
	}

	scanned, err := skills.ScanSource(location.Source)
	if err != nil {
		return nil, err
	}

	items := make([]SkillInfo, 0, len(scanned))
	for _, skill := range scanned {
		doc, err := skills.LoadDocument(skill.SkillFile)
		if err != nil {
			return nil, err
		}

		metadata := doc.ManagedMetadata()
		if metadata.Disabled && !includeDisabled {
			continue
		}

		name := strings.TrimSpace(doc.Name())
		if name == "" {
			name = skill.FlattenedName
		}

		items = append(items, SkillInfo{
			FlattenedName: skill.FlattenedName,
			RelativePath:  skill.RelativePath,
			Name:          name,
			Description:   strings.TrimSpace(doc.Description()),
			Disabled:      metadata.Disabled,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].RelativePath < items[j].RelativePath
	})

	return items, nil
}

func BuildPrompt(items []SkillInfo) string {
	var builder strings.Builder
	builder.WriteString("You are reviewing a set of skills for overlap and duplication.\n\n")
	builder.WriteString("Analyze the list and identify skills that appear to overlap, partially duplicate each other, or would benefit from clearer separation.\n\n")
	builder.WriteString("Return a concise Markdown report with these sections exactly:\n")
	builder.WriteString("## Summary\n")
	builder.WriteString("## Potential Overlap Groups\n")
	builder.WriteString("## Recommendations\n\n")
	builder.WriteString("For each overlap group, include:\n")
	builder.WriteString("- the skill names involved\n")
	builder.WriteString("- why they overlap\n")
	builder.WriteString("- whether the overlap looks like a duplicate, a partial overlap, or just adjacent scope\n")
	builder.WriteString("- a brief recommendation such as merge, clarify, keep separate, or rename\n\n")
	builder.WriteString("Only report likely overlaps. If the set looks clean, say so briefly.\n\n")
	builder.WriteString("Skills:\n")

	for _, item := range items {
		description := item.Description
		if description == "" {
			description = "No description provided."
		}

		builder.WriteString(fmt.Sprintf("- name: %s\n", item.Name))
		builder.WriteString(fmt.Sprintf("  path: %s\n", item.RelativePath))
		builder.WriteString(fmt.Sprintf("  flattened-name: %s\n", item.FlattenedName))
		builder.WriteString(fmt.Sprintf("  description: %s\n", quoteMultiline(description)))
	}

	return builder.String()
}

func Run(tool agenttools.InstalledTool, prompt string) (string, error) {
	output, err := commandRunner(tool.Binary, tool.Tool.Args(prompt))
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", fmt.Errorf("%s returned empty output", tool.Tool.Name)
	}

	return trimmed, nil
}

func runCommand(binary string, args []string) (string, error) {
	cmd := exec.Command(binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("run %s: %s", binary, message)
	}

	return stdout.String(), nil
}

func quoteMultiline(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strconvQuote(value)
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}
