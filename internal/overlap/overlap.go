package overlap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"

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

type Report struct {
	Summary         string   `json:"summary"`
	Groups          []Group  `json:"groups"`
	Recommendations []string `json:"recommendations"`
}

type Group struct {
	SkillNames     []string `json:"skill_names"`
	SkillPaths     []string `json:"skill_paths"`
	Score          int      `json:"score"`
	WhyOverlap     string   `json:"why_overlap"`
	OverlapType    string   `json:"overlap_type"`
	Recommendation string   `json:"recommendation"`
}

type CommandRunner func(ctx context.Context, binary string, args []string, onStatus func(string)) (string, error)

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
	builder.WriteString("Score each overlap group from 0 to 100 where 0 means almost no overlap and 100 means near-duplicate scope.\n")
	builder.WriteString("Only report likely overlaps. If the set looks clean, return an empty groups array and say so in summary.\n\n")
	builder.WriteString("Return only valid JSON. Do not use Markdown. Do not wrap the JSON in code fences.\n")
	builder.WriteString("Use this exact shape:\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"string\",\n")
	builder.WriteString("  \"groups\": [\n")
	builder.WriteString("    {\n")
	builder.WriteString("      \"skill_names\": [\"string\"],\n")
	builder.WriteString("      \"skill_paths\": [\"string\"],\n")
	builder.WriteString("      \"score\": 0,\n")
	builder.WriteString("      \"why_overlap\": \"string\",\n")
	builder.WriteString("      \"overlap_type\": \"duplicate|partial|adjacent\",\n")
	builder.WriteString("      \"recommendation\": \"string\"\n")
	builder.WriteString("    }\n")
	builder.WriteString("  ],\n")
	builder.WriteString("  \"recommendations\": [\"string\"]\n")
	builder.WriteString("}\n\n")
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

func BuildApplyPlanPrompt(report Report) string {
	var builder strings.Builder
	builder.WriteString("You are preparing a plan only. Do not modify files. Do not execute changes. Do not apply edits.\n\n")
	builder.WriteString("Use the overlap report below to propose the minimal set of repository changes needed to apply the recommendations.\n")
	builder.WriteString("Focus on renames, clearer descriptions, clearer trigger language, or safe separations/merges.\n\n")
	builder.WriteString("Return a concise Markdown plan with these sections exactly:\n")
	builder.WriteString("## Goal\n")
	builder.WriteString("## Proposed Changes\n")
	builder.WriteString("## File Targets\n")
	builder.WriteString("## Risks\n")
	builder.WriteString("## Suggested Order\n\n")
	builder.WriteString("Constraints:\n")
	builder.WriteString("- Plan only\n")
	builder.WriteString("- No file changes\n")
	builder.WriteString("- No command execution that modifies the repository\n")
	builder.WriteString("- Prefer minimal changes\n\n")
	builder.WriteString("Overlap report:\n")
	builder.WriteString("Summary: ")
	builder.WriteString(report.Summary)
	builder.WriteString("\n\n")

	if len(report.Groups) > 0 {
		builder.WriteString("Groups:\n")
		for index, group := range report.Groups {
			builder.WriteString(fmt.Sprintf("- Group %d\n", index+1))
			builder.WriteString(fmt.Sprintf("  score: %d\n", group.Score))
			builder.WriteString(fmt.Sprintf("  overlap_type: %s\n", group.OverlapType))
			builder.WriteString(fmt.Sprintf("  skill_paths: %s\n", quoteMultiline(strings.Join(group.SkillPaths, ", "))))
			builder.WriteString(fmt.Sprintf("  why_overlap: %s\n", quoteMultiline(group.WhyOverlap)))
			builder.WriteString(fmt.Sprintf("  recommendation: %s\n", quoteMultiline(group.Recommendation)))
		}
		builder.WriteString("\n")
	}

	if len(report.Recommendations) > 0 {
		builder.WriteString("Top-level recommendations:\n")
		for _, recommendation := range report.Recommendations {
			builder.WriteString(fmt.Sprintf("- %s\n", recommendation))
		}
	}

	return builder.String()
}

func Run(ctx context.Context, tool agenttools.InstalledTool, prompt string, onStatus func(string)) (Report, error) {
	output, err := commandRunner(ctx, tool.Binary, tool.Tool.Args(prompt), onStatus)
	if err != nil {
		return Report{}, err
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return Report{}, fmt.Errorf("%s returned empty output", tool.Tool.Name)
	}

	return ParseReport(trimmed)
}

func RunRaw(ctx context.Context, tool agenttools.InstalledTool, prompt string, onStatus func(string)) (string, error) {
	output, err := commandRunner(ctx, tool.Binary, tool.Tool.Args(prompt), onStatus)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", fmt.Errorf("%s returned empty output", tool.Tool.Name)
	}

	return trimmed, nil
}

func ParseReport(output string) (Report, error) {
	clean := strings.TrimSpace(output)
	if strings.HasPrefix(clean, "```") {
		clean = stripCodeFence(clean)
	}

	var report Report
	if err := json.Unmarshal([]byte(clean), &report); err != nil {
		start := strings.Index(clean, "{")
		end := strings.LastIndex(clean, "}")
		if start == -1 || end == -1 || end < start {
			return Report{}, fmt.Errorf("parse overlap report JSON: %w", err)
		}
		if retryErr := json.Unmarshal([]byte(clean[start:end+1]), &report); retryErr != nil {
			return Report{}, fmt.Errorf("parse overlap report JSON: %w", err)
		}
	}

	report.Normalize()
	return report, nil
}

func (r *Report) Normalize() {
	r.Summary = strings.TrimSpace(r.Summary)
	filteredGroups := make([]Group, 0, len(r.Groups))
	for _, group := range r.Groups {
		group.Normalize()
		if len(group.SkillNames) == 0 && group.WhyOverlap == "" && group.Recommendation == "" {
			continue
		}
		filteredGroups = append(filteredGroups, group)
	}
	sort.Slice(filteredGroups, func(i, j int) bool {
		return filteredGroups[i].Score > filteredGroups[j].Score
	})
	r.Groups = filteredGroups

	filteredRecommendations := make([]string, 0, len(r.Recommendations))
	for _, recommendation := range r.Recommendations {
		recommendation = strings.TrimSpace(recommendation)
		if recommendation == "" {
			continue
		}
		filteredRecommendations = append(filteredRecommendations, recommendation)
	}
	r.Recommendations = filteredRecommendations

	if r.Summary == "" {
		if len(r.Groups) == 0 {
			r.Summary = "No notable overlap detected."
		} else {
			r.Summary = "Potential overlap detected across multiple skills."
		}
	}
}

func (g *Group) Normalize() {
	filteredNames := make([]string, 0, len(g.SkillNames))
	for _, name := range g.SkillNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		filteredNames = append(filteredNames, name)
	}
	g.SkillNames = filteredNames
	filteredPaths := make([]string, 0, len(g.SkillPaths))
	for _, path := range g.SkillPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		filteredPaths = append(filteredPaths, path)
	}
	g.SkillPaths = filteredPaths
	g.WhyOverlap = strings.TrimSpace(g.WhyOverlap)
	g.OverlapType = strings.TrimSpace(g.OverlapType)
	g.Recommendation = strings.TrimSpace(g.Recommendation)
	if g.Score < 0 {
		g.Score = 0
	}
	if g.Score > 100 {
		g.Score = 100
	}
}

func runCommand(ctx context.Context, binary string, args []string, onStatus func(string)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.Command(binary, args...)
	configureInterruptHandling(cmd)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("prepare %s stdout: %w", binary, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("prepare %s stderr: %w", binary, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var wg sync.WaitGroup

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start %s: %w", binary, err)
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdout, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		buffer := make([]byte, 0, 1024)
		scanner.Buffer(buffer, 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			stderr.WriteString(line)
			stderr.WriteByte('\n')
			if onStatus != nil {
				onStatus(line)
			}
		}
	}()

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- cmd.Wait()
	}()

	select {
	case err = <-waitResult:
	case <-ctx.Done():
		_ = interruptProcessTree(cmd)
		err = <-waitResult
	}
	wg.Wait()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s interrupted", binary)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("run %s: %s", binary, message)
	}

	return stdout.String(), nil
}

func stripCodeFence(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func quoteMultiline(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strconvQuote(value)
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}
