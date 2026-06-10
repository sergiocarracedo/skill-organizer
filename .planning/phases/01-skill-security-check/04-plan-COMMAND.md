---
wave: 3
depends_on:
  - 02-plan-REFACTOR.md
  - 03-plan-METADATA.md
files_modified:
  - packages/cli/internal/security/security.go
  - packages/cli/cmd/skill_security.go
  - packages/cli/cmd/skill.go
  - packages/cli/internal/security/security_test.go
  - packages/cli/cmd/skill_security_test.go
autonomous: true
single_layer_justified: false
objective: "Implement skill-organizer skill check-security command: selects an agent tool, analyzes all skills for security risks, stores per-skill risk scores in metadata, and prompts to disable high-risk skills (threshold >= 70). Falls back to printing the analysis prompt to stdout when no tools are detected."
must_haves:
  - "skill-organizer skill check-security --print-prompt outputs a security analysis prompt to stdout and exits 0"
  - "skill-organizer skill check-security (with a fake agent tool) stores risk-score and risk-evaluator in SKILL.md frontmatter"
  - "When risk-score >= 70, user is prompted to disable the skill (default = yes); declining writes the score anyway"
  - "When no agent tools are detected, analysis prompt is printed to stdout and exit code is 0"
  - "go test ./internal/security/... and go test ./cmd/... pass"
---

# Plan 04: check-security command

## Objective

Implement the `skill-organizer skill check-security` command end-to-end. It mirrors the flow of `skill check-overlap`: detect installed tools, select one (cached or prompt), acknowledge costs, run the agent in the foreground via spinner, parse the structured JSON response, and persist per-skill risk scores to skill metadata. If the score is >= 70 (hardcoded threshold), prompt the user to disable the skill with default = yes. If no tools are detected, print the analysis prompt to stdout and exit 0 (unlike overlap's hard error).

## Context

This plan depends on:
- **Plan 02 (Refactor)** — provides the shared `agenttools.ChooseAgentTool`, `agenttools.StartSpinner`, `agenttools.LaunchSession`, and `agenttools.SelectInstalledTool` helpers.
- **Plan 03 (Metadata)** — provides the `ManagedMetadata` risk-score fields and `updateManagedMetadata` helper.

The analysis prompt content (exact wording) is TBD per CONTEXT.md — it's being researched in a parallel sprint. For this plan, use a reasonable placeholder prompt that instructs the agent to analyze all skill files for security risks and return structured JSON per skill. The JSON schema for the agent's response is:

```json
{
  "results": [
    {
      "name": "skill-flattened-name",
      "risk-score": 25,
      "risk-reason": "No dangerous patterns found. All files are plain text."
    }
  ]
}
```

Default risk threshold: 70 (hardcoded, not configurable in P1). The `risk-evaluator` field stores the selected tool's ID + version info (e.g. `claude-code`). On user decline of the disable prompt, the risk score is still written to metadata so the re-enable gate (Plan 05) can read it.

## Tasks

<task id="command-01">
<name>Create internal/security package with prompt builder and report types</name>
<files>
  - packages/cli/internal/security/security.go
  - packages/cli/internal/security/security_test.go
</files>
<action>
Create `packages/cli/internal/security/security.go`:

1. Package declaration: `package security`

2. Import: `github.com/sergiocarracedo/skill-organizer/cli/internal/agenttools`, `context`, `encoding/json`, `fmt`, `strings`

3. Define types:
   ```go
   type SkillInfo struct {
       FlattenedName string
       RelativePath  string
       Name          string
       Description   string
   }

   type SecurityReport struct {
       Results []SkillResult `json:"results"`
   }

   type SkillResult struct {
       Name      string `json:"name"`
       RiskScore int    `json:"risk-score"`
       RiskReason string `json:"risk-reason"`
   }

   type CommandRunner func(ctx context.Context, binary string, args []string, onStatus func(string)) (string, error)
   ```

4. Package-level function variable for test injection:
   ```go
   var commandRunner = runCommand
   ```
   Copy the `runCommand` implementation from `internal/overlap/overlap.go` (it's identical logic — exec binary with pipes for stdout/stderr, process tree interrupt handling). Import `os/exec`, `bufio`, `bytes`, `io`, `sync`, `context`, and the `process_unix.go`/`process_windows.go` build-tag gated functions from the `overlap` package (or better, extract `runCommand` to a shared `internal/executil` package if feasible; otherwise duplicate the ~70 lines).

   **Note:** To avoid bloat, copy the `runCommand` function body. Do NOT split process-interrupt handling into a new file — the `overlap` package handles its own process tree cleanup. The security package's `runCommand` is self-contained.

5. `BuildPrompt(items []SkillInfo) string`:
   - Builds a prompt instructing the AI agent to perform security analysis.
   - Uses a placeholder structure (exact wording TBD per CONTEXT.md). Include:
     - "You are a security auditor for Agent Skills. Analyze each skill listed below for security risks."
     - Minimum checks (from REQUIREMENTS.md): obfuscated text/code, binary files → unevaluable, dangerous instructions (env exfiltration, secret reads, download-code patterns).
     - "Return only valid JSON. Do not use Markdown. Do not wrap in code fences."
     - "For each skill, return: name (flattened name), risk-score (0-100, 100 = max risk), risk-reason (one-line explanation)."
     - The skills list: name, path, description for each.
   - The placeholder should be at least 20 lines so a real agent can produce meaningful output.

6. `Run(ctx context.Context, tool agenttools.InstalledTool, prompt string, onStatus func(string)) (SecurityReport, error)`:
   - Call `commandRunner(ctx, tool.Binary, tool.Tool.Args(prompt), onStatus)`.
   - If empty output: error.
   - Call `ParseReport(output)`.

7. `ParseReport(output string) (SecurityReport, error)`:
   - Strip code fences (copy `stripCodeFence` from overlap).
   - Unmarshal JSON into `SecurityReport`.
   - Validate: `RiskScore` clamped to 0-100, empty name defaults to "unknown".

8. `func CollectSkills(location configpkg.Location, includeDisabled bool) ([]SkillInfo, error)`:
   - Copy the logic from `overlap.CollectSkills` — it scans source, loads each skill's document, builds `SkillInfo` list.
   - Import `configpkg` and `skills` packages.
   - Reuse `skills.ScanSource` and `skills.LoadDocument`.
   - Filter disabled skills unless `includeDisabled` is true.

Create `packages/cli/internal/security/security_test.go`:

9. `TestParseSecurityReport`:
   - Input: `{"results":[{"name":"test--skill","risk-score":85,"risk-reason":"Uses eval()"}]}`
   - Parse, assert `Results[0].RiskScore == 85`, `.Name == "test--skill"`, `.RiskReason == "Uses eval()"`

10. `TestParseSecurityReportWithCodeFences`:
    - Input with ```json prefix/suffix, assert it still parses correctly.

11. `TestBuildPromptIncludesSkills`:
    - Call `BuildPrompt` with one `SkillInfo{Name: "demo", FlattenedName: "thirdparty--demo"}`.
    - Assert output contains "thirdparty--demo" and "risk-score".

12. `TestRunWithFakeCommand`:
    - Override `commandRunner` with a fake that returns valid JSON.
    - Call `Run`, assert report is parsed correctly.
    - Restore via `t.Cleanup`.
</action>
<verify>
- `go test ./internal/security/...` passes
- `TestParseSecurityReport` passes
- `TestBuildPromptIncludesSkills` passes
</verify>
<done>[ ]</done>
</task>

<task id="command-02">
<name>Create check-security cobra command</name>
<files>
  - packages/cli/cmd/skill_security.go
  - packages/cli/cmd/skill.go
</files>
<action>
Create `packages/cli/cmd/skill_security.go`:

1. Package `cmd`, with imports: `fmt`, `os`, `strconv`, `time`, `github.com/pterm/pterm`, `github.com/spf13/cobra`, `agenttools`, `configpkg`, `securitypkg` (aliased import for `internal/security`), `skills` (internal/skills).

2. Package-level flags: `securityPrintPrompt bool`, `securityToolID string`, `securityChooseTool bool`, `securityAllSkills bool`.

3. Package-level function variables for test injection (following the established pattern):
   ```go
   var (
       securityLoadResolvedLocation = loadResolvedLocation
       securityDetectInstalledTools = agenttools.DetectInstalled
       securityLoadConfigFunc       = configpkg.LoadAgentSelectionConfigOrDefault
       securitySaveConfigFunc       = configpkg.SaveAgentSelectionConfig
       securityCollectSkills        = securitypkg.CollectSkills
       securityBuildPrompt          = securitypkg.BuildPrompt
       securityRunAnalysis          = securitypkg.Run
       securityConfirm              = confirm
       securityUpdateMetadata       = skills.UpdateManagedMetadata
       securityStartSpinner         = agenttools.StartSpinner
       securityPrintPrompt          = func(prompt string) { pterm.Println(prompt) }
       securityHideCursor           = agenttools.HideCursor
       securityShowCursor           = agenttools.ShowCursor
       securityPrintInfo            = func(format string, args ...any) { pterm.Info.Printfln(format, args...) }
       securityPrintSuccess         = func(format string, args ...any) { pterm.Success.Printfln(format, args...) }
       securityPrintWarning         = func(format string, args ...any) { pterm.Warning.Printfln(format, args...) }
   )
   ```

4. `newCheckSecurityCommand() *cobra.Command`:
   - `Use: "check-security"`
   - `Short: "Evaluate skills for security risks using an installed agent tool"`
   - `RunE:` following the flow:

   Flow:
   a. Resolve location via `securityLoadResolvedLocation()`.
   b. Collect skills via `securityCollectSkills(location, securityAllSkills)`.
   c. If no skills found, return error.
   d. Build prompt via `securityBuildPrompt(items)`.
   e. If `--print-prompt` flag is set: call `securityPrintPrompt(prompt)`, return nil.
   f. Detect installed tools via `securityDetectInstalledTools()`.
   g. If no tools installed: **print prompt to stdout and exit 0** (skip the error — differs from overlap). Use `fmt.Println(prompt); return nil`.
   h. Load config via `securityLoadConfigFunc(registryPath)`.
   i. Choose tool: `agenttools.ChooseAgentTool(installed, cfg, securityToolID, securityChooseTool, selectOption)`.
   j. If costs not acknowledged: prompt via `securityConfirm`. On accept, set `cfg.AcknowledgedExternalToolCosts = true`, save config.
   k. Print info about selected tool.
   l. Start spinner via `securityStartSpinner("Analyzing skills for security risks")`.
   m. Run analysis via `securityRunAnalysis(cmd.Context(), tool, prompt, updateText)`.
   n. Spinner success. Parse report.
   o. For each `result` in the report:
      - Build `skills.ManagedMetadata{ RiskScore: result.RiskScore, RiskEvaluatedAt: now.RFC3339, RiskEvaluator: tool.Tool.ID, RiskReason: result.RiskReason }`.
      - Call `securityUpdateMetadata(skill, updates)` — need to match result name to skill. Match by `FlattenedName`.
      - If `result.RiskScore >= 70`: print warning, prompt "Disable skill `<name>` due to high risk?" with default=yes.
        - If yes: call `skills.RewriteManagedFields(skill, false, true)` (sets disabled).
        - If no: do nothing (risk score already written).
   p. Print summary: "Checked N skills, M high-risk, L disabled."
   q. Return nil.

5. Flags:
   ```go
   cmd.Flags().BoolVar(&securityPrintPrompt, "print-prompt", false, "Print the generated security prompt without invoking an external tool")
   cmd.Flags().StringVar(&securityToolID, "tool", "", "Use a specific installed tool id (claude, codex, opencode, cursor, antigravity)")
   cmd.Flags().BoolVar(&securityChooseTool, "choose-tool", false, "Prompt to choose the agent tool again")
   cmd.Flags().BoolVar(&securityAllSkills, "include-disabled", false, "Include disabled skills in the analysis")
   ```

6. Helper to match result name to scanned skill:
   ```go
   func skillByFlattenedName(skills []securitypkg.SkillInfo, name string) (securitypkg.SkillInfo, bool) {
       for _, s := range skills {
           if s.FlattenedName == name {
               return s, true
           }
       }
       return securitypkg.SkillInfo{}, false
   }
   ```

7. Helper to build a real `skills.Skill` from `securitypkg.SkillInfo` (for calling `RewriteManagedFields` and `updateManagedMetadata`):
   - The `securitypkg.SkillInfo` has `FlattenedName` and `RelativePath`. Build `skills.Skill`:
     ```go
     func toSkill(si securitypkg.SkillInfo, location configpkg.Location) skills.Skill {
         return skills.Skill{
             Dir:           filepath.Join(location.Source, si.RelativePath),
             SkillFile:     filepath.Join(location.Source, si.RelativePath, "SKILL.md"),
             RelativePath:  si.RelativePath,
             FlattenedName: si.FlattenedName,
         }
     }
     ```

Modify `packages/cli/cmd/skill.go`:

8. Add import for `securitypkg` if not already there (it's not needed directly, just wire the command via the same pattern).

9. Add `cmd.AddCommand(newCheckSecurityCommand())` after `newCheckOverlapCommand()`.
</action>
<verify>
- `go build ./...` succeeds
- `skill-organizer skill check-security --help` shows the new command
- `go vet ./cmd/...` passes
</verify>
<done>[ ]</done>
</task>

<task id="command-03">
<name>Add tests for check-security command</name>
<files>
  - packages/cli/cmd/skill_security_test.go
</files>
<action>
Create `packages/cli/cmd/skill_security_test.go`:

Following the established test patterns from `skill_overlap_test.go`:

1. `TestCheckSecurityPrintPromptBypassesToolDetection`:
   - Set `securityPrintPrompt = true`
   - Fake `securityLoadResolvedLocation` and `securityCollectSkills` (return 1 skill)
   - Fake `securityDetectInstalledTools` to return error if called
   - Capture the printed prompt
   - Call `newCheckSecurityCommand().RunE()`
   - Assert prompt is non-empty and contains security-related keywords ("risk", "security")
   - Verify tool detection was not called

2. `TestCheckSecurityNoToolsDetectedPrintsPromptAndExits0`:
   - Fake `securityDetectInstalledTools` to return empty list
   - Fake `securityLoadResolvedLocation` and `securityCollectSkills`
   - Capture stdout output
   - Run command, assert no error returned
   - Assert prompt text appears in captured output

3. `TestCheckSecurityStoresRiskScoreOnLowRisk`:
   - Fake all dependencies (detect returns a tool, load config returns cfg with costs acknowledged, collect returns 1 skill)
   - Fake `securityRunAnalysis` to return a report with risk-score = 25
   - Fake `securityUpdateMetadata` to capture what was written
   - Run command, assert no error
   - Assert `RiskScore == 25` and `RiskEvaluator == tool.Tool.ID` in captured metadata

4. `TestCheckSecurityPromptsToDisableHighRisk`:
   - Same setup as above, but `securityRunAnalysis` returns risk-score = 85
   - Fake `securityConfirm` to return `true` (user accepts disable)
   - Fake `skills.RewriteManagedFields` via a command-level function var — add a `securityWriteDisabled` func var or reuse the command's integration
   - Assert disabled was written

5. `TestCheckSecurityWritesScoreEvenWhenDecliningDisable`:
   - Same as above but `securityConfirm` returns `false`
   - Assert risk score IS written to metadata (updateMetadata was called)
   - Assert disabled was NOT written

Use `stubSpinner{}` from `skill_overlap_test.go` or define a local one. Use `mockInstalledTool` from `skill_overlap_test.go` (it's in the same `cmd` package, so it's accessible).

IMPORTANT: All function vars must be restored in `t.Cleanup`.
</action>
<verify>
- `go test ./cmd/... -run TestCheckSecurity` passes
- All 5 tests pass:
  - TestCheckSecurityPrintPromptBypassesToolDetection
  - TestCheckSecurityNoToolsDetectedPrintsPromptAndExits0
  - TestCheckSecurityStoresRiskScoreOnLowRisk
  - TestCheckSecurityPromptsToDisableHighRisk
  - TestCheckSecurityWritesScoreEvenWhenDecliningDisable
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./internal/security/...` passes
- [ ] `go test ./cmd/... -run TestCheckSecurity` passes (5 tests)
- [ ] `go build ./...` succeeds
- [ ] `skill-organizer skill check-security --print-prompt` outputs a security analysis prompt and exits 0
- [ ] When no tools detected, prompt is printed to stdout and exit code is 0
- [ ] High-risk skills (score >= 70) trigger disable prompt with default = yes
- [ ] Risk score is persisted in SKILL.md metadata even when user declines disable
- [ ] `securitypkg.CollectSkills` returns skills with name/description/path

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/internal/security/ packages/cli/cmd/skill_security.go packages/cli/cmd/skill_security_test.go packages/cli/cmd/skill.go`
2. Remove untracked: `rm -rf packages/cli/internal/security/`
3. Verify: `go test ./...` passes
4. Retry with smaller scope (e.g., create security package first, then command separately)

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | Agent returns malformed JSON that doesn't match expected schema | Medium | Medium | `ParseReport` strips code fences, attempts JSON extraction from `{` to `}` if standard unmarshal fails. Error message includes raw output for debugging. |
| 2 | Agent tool exits non-zero but still produces valid JSON on stdout | Medium | Low | The `commandRunner` captures stdout even on non-zero exit, then Parsses it. Only empty stdout triggers error. Stderr is used for status display but not for error detection. |
| 3 | Risk score isn't persisted because `updateManagedMetadata` isn't wired correctly | Low | High | Test `TestCheckSecurityStoresRiskScoreOnLowRisk` validates the metadata was written with correct values. Integration test reads back the file. |
| 4 | User accidentally disables a safe skill (prompt fatigue) | Low | Medium | The disable prompt is only shown for score >= 70 with a clear visual warning. Default is yes (cautious), but user can always re-enable. The re-enable gate (Plan 05) provides a second chance. |

## Commit Message

```
feat(cli): implement skill check-security command

- Create internal/security package with prompt builder, report types, runner
- Add check-security cobra command mirroring overlap's agent selection flow
- Store per-skill risk scores (0-100) in SKILL.md metadata
- Prompt to disable skills with risk-score >= 70 (default = yes)
- No-tools-detected: print analysis prompt to stdout, exit 0
- --print-prompt flag to preview the analysis prompt without running agent
- Tests cover print-prompt, no-tools, low-risk, high-risk, and decline flows
```
