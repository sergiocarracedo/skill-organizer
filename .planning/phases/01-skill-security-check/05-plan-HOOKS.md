---
wave: 4
depends_on:
  - 04-plan-COMMAND.md
files_modified:
  - packages/cli/cmd/enable.go
  - packages/cli/cmd/skill_add.go
  - packages/cli/cmd/skill_security.go
  - packages/cli/cmd/enable_test.go
  - packages/cli/cmd/skill_add_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-4
objective: "Add re-enable gate to the enable command (shows risk reason for high-risk skills, asks confirmation before re-enabling) and a post-install hook to skill add (prompts to run check-security on newly installed skills)."
must_haves:
  - "Enabling a high-risk skill (risk-score >= 70, risk-evaluator non-empty) shows the risk reason and asks confirmation with default=no; on decline, writes disabled: true explicitly"
  - "Enabling a safe or unevaluated skill proceeds without extra prompt (no behavior change)"
  - "After skill add, user is prompted 'Run check-security for <name>?' with default=yes; on yes, runs check-security; on no, writes risk-evaluator: '' to mark unevaluated"
  - "go test ./cmd/... passes"
---

# Plan 05: Re-enable gate and skill-add hook

## Objective

Add two UX integration points around the check-security command:

1. **Re-enable gate** in `skill enable <path>`: if the skill has `risk-score >= 70` and `risk-evaluator != ""`, the command shows the `risk-reason` and asks for confirmation before enabling. Default answer is **no** (cautious). If the user declines, `disabled: true` is explicitly written to metadata (not just left as-is), so the skill stays off even if an earlier caller left it in a partial state.

2. **Post-install hook** in `skill add <source>`: after a successful install and sync, prompt the user with *"Run check-security for `<name>`?"* with **default = yes**. On yes, invoke the check-security flow inline for that one skill. On no, write `risk-evaluator: ""` to mark the skill *unevaluated* (so the next `check-security` run picks it up).

## Context

Both hooks depend on the check-security command (Plan 04) being complete. The enable gate reads the risk-score from `ManagedMetadata` and uses the existing `confirm` helper from `cmd/prompt.go`. The add hook invokes the check-security logic for a single skill.

The threshold for "high risk" is 70 (hardcoded). Only skills with `risk-evaluator != ""` are considered "evaluated" — unevaluated skills (risk-evaluator is empty) bypass the gate entirely, since there's no data to warn about.

The `confirm` helper in `prompt.go` follows: `pterm.DefaultInteractiveConfirm.WithDefaultValue(defaultValue).Show(prompt)`.

## Tasks

<task id="hooks-01">
<name>Add re-enable gate to enable command</name>
<files>
  - packages/cli/cmd/enable.go
  - packages/cli/cmd/enable_test.go
</files>
<action>
In `enable.go`:

1. Add imports: add `configpkg` (already aliased pattern from other files — `configpkg "github.com/.../cli/internal/config"`), `fmt`, `strconv`, `strings`.

2. Add package-level function variables for test injection (following established pattern):
   ```go
   var (
       enableConfirm             = confirm
       enableRewriteManagedFields = skills.RewriteManagedFields
       enableUpdateManagedFields = skills.UpdateManagedMetadata
   )
   ```

3. Modify `newEnableCommand()`'s `RunE`:
   After line 23 (`skill, err := skills.ResolveSourceSkill(location.Source, args[0])`), insert the gate:

   a. Load the skill's document: `doc, err := skills.LoadDocument(skill.SkillFile)` (if error, just log debug and skip gate — don't block enable on missing metadata).
   
   b. Read metadata: `metadata := doc.ManagedMetadata()`.

   c. Check gate conditions:
      ```go
      if metadata.RiskScore >= 70 && strings.TrimSpace(metadata.RiskEvaluator) != "" {
          // Show warning with risk info
          pterm.Warning.Printfln("This skill has a high risk score of %d/100 (evaluated by %s)", metadata.RiskScore, metadata.RiskEvaluator)
          if strings.TrimSpace(metadata.RiskReason) != "" {
              pterm.Warning.Printfln("Reason: %s", metadata.RiskReason)
          }
          
          accepted, err := enableConfirm("Are you sure you want to enable this high-risk skill?", false)
          if err != nil {
              return err
          }
          if !accepted {
              // Explicitly write disabled: true to ensure skill stays off
              if err := enableRewriteManagedFields(skill, false, true); err != nil {
                  return err
              }
              return fmt.Errorf("aborted: skill remains disabled")
          }
      }
      ```

   d. The rest of the function (lines 28-39) remains unchanged — it calls `skills.RewriteManagedFields(skill, false, false)` to set disabled=false, then syncs.

   e. Note: Import `strconv` for the `%d` formatting — `metadata.RiskScore` is an int.

In `enable_test.go` (create if it doesn't exist):

4. Add `TestEnableHighRiskSkillPromptsConfirmation`:
   - Create a temp skill directory with a SKILL.md that has risk-score=85, risk-evaluator="claude", risk-reason="Contains shell execution"
   - Fake `enableConfirm` to return `false` (user declines)
   - Call `newEnableCommand().RunE()` with the skill path
   - Assert the command returns an error (aborted)
   - Reload the skill's SKILL.md, assert `disabled: true` is still set

5. Add `TestEnableHighRiskSkillProceedsOnConfirmation`:
   - Same setup but `enableConfirm` returns `true`
   - Assert no error
   - Reload SKILL.md, assert `disabled: false` (skill was enabled)

6. Add `TestEnableLowRiskSkillBypassesGate`:
   - Create skill with risk-score=25, risk-evaluator="claude"
   - Fake `enableConfirm` to error if called (should not be called)
   - Run enable, assert no error

7. Add `TestEnableUnevaluatedSkillBypassesGate`:
   - Create skill with risk-score=0, risk-evaluator="" (unevaluated)
   - Fake `enableConfirm` to error if called
   - Run enable, assert no error

All tests: use `t.TempDir()` for test directory, restore all func vars via `t.Cleanup`.
</action>
<verify>
- `go test ./cmd/... -run TestEnableHighRisk` passes
- `go test ./cmd/... -run TestEnableLowRiskSkillBypassesGate` passes
- `go test ./cmd/... -run TestEnableUnevaluatedSkillBypassesGate` passes
</verify>
<done>[ ]</done>
</task>

<task id="hooks-02">
<name>Add post-install check-security hook to skill add</name>
<files>
  - packages/cli/cmd/skill_add.go
  - packages/cli/cmd/skill_add_test.go
</files>
<action>
In `skill_add.go`:

1. Add imports: `agenttools`, `configpkg` (follow existing import patterns).

2. Add package-level function variables:
   ```go
   var (
       skillAddConfirmRunSecurity = confirm
       skillAddRunSecurityForSkill = runCheckSecurityForSkill
       skillAddUpdateMetadata     = skills.UpdateManagedMetadata
   )
   ```

3. Add a helper function `runCheckSecurityForSkill(skill skills.Skill, location configpkg.Location) error`:
   - Load the document to get current metadata.
   - Call the internal security analysis flow for a single skill:
     a. Build a single-skill prompt using `securitypkg.BuildPrompt`.
     b. Detect and select tool (reuse the agent-selection helpers).
     c. Run analysis.
     d. Parse report, extract the first result.
     e. Write risk score to metadata via `skills.UpdateManagedMetadata`.
   - This is a simplified version of the full check-security command's RunE body, but for one skill with no interactive flags.
   - For the prompt: import `securitypkg "github.com/.../cli/internal/security"`.

   Actually, a cleaner approach: define `runCheckSecurityForSkill` to reuse existing functions from `skill_security.go` by extracting its core logic into a shared helper. But to minimize coupling, make `runCheckSecurityForSkill` a self-contained function in `skill_add.go` that:
   - Calls `securitypkg.BuildPrompt` with one `SkillInfo`
   - Calls `agenttools.DetectInstalled`
   - If tools found: selects first (no prompt in hook mode — just pick the first available), runs analysis
   - If no tools: prints warning and returns (don't block the install)

4. Modify `newSkillAddCommand()`'s `RunE`:
   After the sync result print (line 197, `printSyncResult(configFile, result)`), add a loop over newly installed skills:
   ```go
   for _, installed := range selected {
       targetSkill := existingNames[installed.Name]
       if targetSkill.SkillFile == "" {
           continue
       }
       runSecurity, err := skillAddConfirmRunSecurity(
           fmt.Sprintf("Run check-security for %q?", installed.Name), true)
       if err != nil {
           return err
       }
       if runSecurity {
           if err := skillAddRunSecurityForSkill(targetSkill, location); err != nil {
               pterm.Warning.Printfln("Security check failed for %s: %v", installed.Name, err)
           }
       } else {
           // Mark skill as unevaluated
           if err := skillAddUpdateMetadata(targetSkill, skills.ManagedMetadata{
               RiskEvaluator: "",
               RiskEvaluatedAt: time.Now().UTC().Format(time.RFC3339),
           }); err != nil {
               pterm.Warning.Printfln("Failed to mark unevaluated: %v", err)
           }
       }
   }
   ```

In `skill_add_test.go`:

5. Add `TestSkillAddHooksCheckSecurityPrompt`:
   - Set up a full skill-add scenario with fakes
   - Fake `skillAddConfirmRunSecurity` to capture the prompt and return `false`
   - Fake `skillAddRunSecurityForSkill` to error if called (should not be called on decline)
   - Fake `skillAddUpdateMetadata` to capture the unevaluated marker
   - Assert the prompt contains "Run check-security"
   - Assert `RiskEvaluator == ""` was written for the unevaluated marker

6. Add `TestSkillAddRunsSecurityOnAccept`:
   - Same setup, `skillAddConfirmRunSecurity` returns `true`
   - Fake `skillAddRunSecurityForSkill` to succeed
   - Assert `skillAddRunSecurityForSkill` was called with the correct skill

Restore all func vars via `t.Cleanup`.
</action>
<verify>
- `go test ./cmd/... -run TestSkillAddHooks` passes
- `go test ./cmd/... -run TestSkillAddRunsSecurityOnAccept` passes
</verify>
<done>[ ]</done>
</task>

<task id="hooks-03">
<name>Export required helpers from check-security command</name>
<files>
  - packages/cli/cmd/skill_security.go
</files>
<action>
The hook in `skill_add.go` needs to call the check-security flow for a single skill without duplicating logic. To enable this without tight coupling:

1. In `skill_security.go`, extract core logic into an exported (public) helper function:
   ```go
   // RunCheckSecurityForSkill performs a security analysis on a single skill
   // using the default agent selection flow. Used by the skill-add hook.
   func RunCheckSecurityForSkill(skill skills.Skill, location configpkg.Location) error {
       // Build single-skill prompt
       info := securitypkg.SkillInfo{
           FlattenedName: skill.FlattenedName,
           RelativePath:  skill.RelativePath,
       }
       // Load document to get name/description
       doc, err := skills.LoadDocument(skill.SkillFile)
       if err == nil {
           info.Name = doc.Name()
           info.Description = doc.Description()
       }
       prompt := securitypkg.BuildPrompt([]securitypkg.SkillInfo{info})

       // Detect installed tools
       installed, err := securityDetectInstalledTools()
       if err != nil {
           return fmt.Errorf("detect tools: %w", err)
       }
       if len(installed) == 0 {
           pterm.Warning.Println("No agent tools detected. Run 'skill-organizer skill check-security' manually after installing a tool.")
           return nil
       }

       // Load config, select tool (auto-pick first installed, no prompt)
       registryPath, err := configpkg.RegistryPath()
       if err != nil {
           return err
       }
       cfg, err := securityLoadConfigFunc(registryPath)
       if err != nil {
           return err
       }
       
       tool, cfg, err := agenttools.ChooseAgentTool(installed, cfg, "", false, func(_ []string, _ string) (string, error) {
           return agenttools.Label(installed[0]), nil
       })
       if err != nil {
           return err
       }

       // Acknowledge costs if needed
       if !cfg.AcknowledgedExternalToolCosts {
           accepted, err := securityConfirm("This command runs an installed external agent CLI. Continue?", false)
           if err != nil {
               return err
           }
           if !accepted {
               pterm.Warning.Println("Security check skipped by user.")
               return nil
           }
           cfg.AcknowledgedExternalToolCosts = true
           if err := securitySaveConfigFunc(registryPath, cfg); err != nil {
               return err
           }
       }

       // Run analysis  
       report, err := securityRunAnalysis(context.Background(), tool, prompt, func(_ string) {})
       if err != nil {
           return fmt.Errorf("security analysis failed: %w", err)
       }

       // Write results
       now := time.Now().UTC().Format(time.RFC3339)
       for _, result := range report.Results {
           if result.Name == skill.FlattenedName {
               updates := skills.ManagedMetadata{
                   RiskScore:      result.RiskScore,
                   RiskEvaluatedAt: now,
                   RiskEvaluator:  tool.Tool.ID,
                   RiskReason:     result.RiskReason,
               }
               if err := skills.UpdateManagedMetadata(skill, updates); err != nil {
                   return fmt.Errorf("persist risk score: %w", err)
               }
               break
           }
       }

       return nil
   }
   ```

2. Add `updateText` helper for the spinner callback (shared between the command flow and hook):
   ```go
   func limitSpinnerTextForSecurity(value string, width int) string {
       // Same logic as limitSpinnerText in skill_overlap.go
       // (It's already a standalone function, just reference it or duplicate)
   }
   ```

3. Ensure all function variables used in `RunCheckSecurityForSkill` (`securityDetectInstalledTools`, `securityLoadConfigFunc`, `securitySaveConfigFunc`, `securityConfirm`, `securityRunAnalysis`) are defined at the package level in `skill_security.go` (Task command-02 already creates them).
</action>
<verify>
- `go build ./...` succeeds
- `RunCheckSecurityForSkill` is an exported function (starts with capital letter) in the `cmd` package
- No import cycles introduced
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./cmd/... -run TestEnable` passes (all enable tests including risk gate)
- [ ] `go test ./cmd/... -run TestSkillAdd` passes (all skill add tests including hook)
- [ ] `go build ./...` succeeds
- [ ] When enabling a skill with risk-score=85 and risk-evaluator="claude", the user sees the risk reason and is prompted with default=no
- [ ] When user declines the enable gate, `disabled: true` is explicitly written in the skill's frontmatter
- [ ] When user accepts the enable gate, the skill is enabled (disabled=false) and risk-score remains in metadata
- [ ] After `skill add <source>`, user is prompted "Run check-security for `skillname`?" with default=yes
- [ ] On decline, `risk-evaluator: ""` is written to mark the skill unevaluated
- [ ] Skill-add hook imports no packages that create circular dependencies

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/cmd/enable.go packages/cli/cmd/enable_test.go packages/cli/cmd/skill_add.go packages/cli/cmd/skill_add_test.go packages/cli/cmd/skill_security.go`
2. If new test files were created: `rm packages/cli/cmd/enable_test.go` (or if it existed before, checkout)
3. Verify: `go test ./cmd/...` passes
4. Retry — the enable gate and add hook are independent of each other, so they can be done one at a time

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | Enable gate blocks legitimate use of a skill with a borderline score (70-79) | Medium | Low | Threshold is 70 (not 50). User can always override by confirming with default=no. The risk reason is shown to help the user decide. |
| 2 | Skill-add hook's agent auto-selection (first installed) picks a tool the user doesn't want to use for security analysis | Medium | Low | The hook auto-picks the first installed tool silently. If the user wants a different tool, they can run `check-security` manually with `--tool`. This is intentional — the hook is a convenience, not a full UX. |
| 3 | Enabling a skill that was disabled by security check loses the risk score (overwritten by RewriteManagedFields) | Low | Medium | `RewriteManagedFields(skill, false, false)` doesn't touch risk fields — it only sets `disabled: false`. Risk-score, risk-reason, etc. survive the enable. The round-trip test in Plan 03 validates this. |
| 4 | Skill-add hook runs the security check synchronously, making `skill add` noticeably slower | Medium | Low | The hook shows a progress indicator. Security analysis for one skill typically completes in <30 seconds. Users who want speed can decline the prompt (default is yes, but quick to press no). |

## Commit Message

```
feat(cli): add re-enable risk gate and post-install security hook

- Enable gate: if risk-score >= 70 and evaluator set, show reason and
  ask confirmation (default=no); decline writes disabled:true explicitly
- Skill-add hook: after install, prompt "Run check-security for <name>?"
  (default=yes); decline marks skill unevaluated
- Export RunCheckSecurityForSkill helper for use by skill-add hook
- Add tests for enable gate (high-risk, low-risk, unevaluated cases)
- Add tests for skill-add hook (accept, decline cases)
```
