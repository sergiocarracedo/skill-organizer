---
wave: 1
depends_on: []
files_modified:
  - packages/cli/cmd/skill_overlap.go
  - packages/cli/cmd/skill_overlap_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-3
objective: "Add the --allow-overlap cobra flag and the non-zero exit code to skill-organizer skill check-overlap so that overlap findings produce exit 1 by default and exit 0 with the flag, while the report is still printed either way."
must_haves:
  - "go test ./cmd/... passes"
  - "TestCheckOverlapExitsNonZeroOnOverlap passes (report with groups, no --allow-overlap => RunE returns non-nil error containing 'overlap detected')"
  - "TestCheckOverlapAllowOverlapExitsZero passes (same report, --allow-overlap => RunE returns nil)"
  - "TestCheckOverlapExitsZeroOnEmptyReport passes (empty report, no flag => RunE returns nil)"
  - "go build ./... succeeds"
  - "skill check-overlap --help shows the new --allow-overlap flag"
---

# Plan 01: --allow-overlap flag and non-zero exit code

## Objective

Add a `--allow-overlap` flag to the `check-overlap` cobra command and wire an exit-code check that returns a non-zero exit (via a non-nil `error` from `RunE`) when overlap groups remain after filtering, unless the user passes `--allow-overlap`. The check is placed **after** `printOverlapReport` and **before** the `if overlapNoAskToApply` early-return, so the report is always printed, the apply-plan flow is still reachable, and `--no-ask-to-apply` is independent of the exit code. This delivers the exit-semantics half of REQ-3 acceptance.

## Context

The `check-overlap` command already exists in `packages/cli/cmd/skill_overlap.go`. It is wired to `agenttools.ChooseAgentTool`, `agenttools.StartSpinner`, and `agenttools.LaunchSession` (delivered in Phase 1 plan 02). The current `RunE` flow ends with a print of the report and then an `if overlapNoAskToApply` early-return; everything after that is the apply-plan prompt flow. The cobra exit-code plumbing is already wired: `main.go:11-14` calls `pterm.Error.Printfln` and `os.Exit(1)` on any error from `cmd.Execute()`, and the root command sets `SilenceUsage=true; SilenceErrors=true` (no cobra-internal printing of the error).

The shared pattern is: a package-level `bool` var + `cmd.Flags().BoolVar(...)` + a read of the var in `RunE`. The new flag mirrors `overlapNoAskToApply` exactly (same file, lines 19-26 and 198).

The accepted user error text is a terse, actionable string: `"overlap detected: N group(s) (use --allow-overlap to ignore)"`. The `pterm.Error` red banner is what every other cobra command in this repo produces on `fmt.Errorf` from `RunE` (e.g. `skill_check_updates.go`, `skill_delete.go`); we accept the noise.

## Tasks

<task id="01-01">
<name>Add overlapAllowOverlap var, register the flag, and wire the exit-code check</name>
<files>
- packages/cli/cmd/skill_overlap.go
</files>
<action>
In `packages/cli/cmd/skill_overlap.go`:

1. Add `overlapAllowOverlap bool` to the existing `var ( ... )` block at lines 19-26 (the block already holding `overlapChooseTool`, `overlapToolID`, `overlapAllSkills`, `overlapPrintPrompt`, `overlapMinType`, `overlapNoAskToApply`). Keep the existing block order; add the new var at the end of the block so the diff is minimal.

2. In `newCheckOverlapCommand()` (around line 198, right after the `overlapNoAskToApply` flag registration), add:
   ```go
   cmd.Flags().BoolVar(&overlapAllowOverlap, "allow-overlap", false,
       "Exit 0 even when overlap groups are found (the report is still printed)")
   ```

3. In the `RunE` closure, **after** the call to `printOverlapReport(tool, len(items), overlapAllSkills, report)` at line 143 and **before** the `if overlapNoAskToApply { return nil }` block at line 145, insert the exit-code check:
   ```go
   // P2 (REQ-3): non-zero exit on overlap; --allow-overlap suppresses it.
   if len(report.Groups) > 0 && !overlapAllowOverlap {
       return fmt.Errorf(
           "overlap detected: %d group(s) (use --allow-overlap to ignore)",
           len(report.Groups),
       )
   }
   ```

4. Do NOT move the apply-plan flow. The check must slot between the report print and the `if overlapNoAskToApply` block; it must not move the `if overlapNoAskToApply` block or the `BuildApplyPlanPrompt` / `LaunchSession` flow that follows.

5. Do NOT add any new imports. `fmt` is already imported.

6. Do NOT change the report schema, the apply-plan flow, the `--min-overlap-type` default, the `--no-ask-to-apply` early-return, or any of the other existing flags.
</action>
<verify>
- `go build ./...` succeeds
- `go vet ./cmd/...` passes
- `go run ./main.go skill check-overlap --help` lists `--allow-overlap` in the output
- The string `--allow-overlap` appears exactly once in `packages/cli/cmd/skill_overlap.go` (declaration + usage in RunE)
</verify>
<done>[ ]</done>
</task>

<task id="01-02">
<name>Add cmd-level tests for the exit code and --allow-overlap flag</name>
<files>
- packages/cli/cmd/skill_overlap_test.go
</files>
<action>
In `packages/cli/cmd/skill_overlap_test.go`, add three new tests at the end of the file (after the existing `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode` at line 274-394 and `TestWriteApplyPlanPromptCreatesTimestampedFile` at line 396-434, before the `containsLine` helper at line 436-443). Use the same stub-set pattern as the existing `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`:

- Stash originals of every func var the test touches: `overlapChooseTool`, `overlapToolID`, `overlapAllSkills`, `overlapPrintPrompt`, `overlapNoAskToApply`, **`overlapAllowOverlap` (new)**, `detectInstalledTools`, `loadResolvedLocationFunc`, `collectOverlapSkills`, `loadAgentSelectionConfigFunc`, `saveAgentSelectionConfigFunc`, `runOverlapAnalysis`, `confirmApplyPlan`, `confirmExternalCosts`, `saveApplyPlanPrompt`, `printInfoMessage`, `printDebugMessage`, `printWarningMessage`, `agenttools.StartSpinnerFunc`, `agenttools.LaunchSessionFunc`.
- Set every var to a stub. Reuse the existing helpers `stubSpinner{}` and `mockInstalledTool(id, binary)` already defined in the same file (lines 445-449 and 451-454).
- Restore everything in a single `t.Cleanup(...)` block.
- Set `loadAgentSelectionConfigFunc` to return `configpkg.AgentSelectionConfig{DefaultAgentTool: "opencode", AcknowledgedExternalToolCosts: true}` so the cost-ack prompt is skipped.
- Set `agenttools.LaunchSessionFunc` to return `fmt.Errorf("launchPlanSession should not be called")` so any test that accidentally falls into the apply-plan flow fails loudly.

The three new tests:

1. `TestCheckOverlapExitsNonZeroOnOverlap`:
   - Set `overlapNoAskToApply = true` and `overlapAllowOverlap = false`.
   - Set `runOverlapAnalysis` to return a report with one `partial` group: `overlap.Report{Groups: []overlap.Group{{SkillNames: []string{"alpha", "beta"}, SkillPaths: []string{"personal/alpha", "personal/beta"}, Score: 72, OverlapType: "partial", WhyOverlap: "They overlap.", Recommendation: "Separate them."}}}`.
   - Call `cmd := newCheckOverlapCommand(); err := cmd.RunE(cmd, nil)`.
   - Assert `err != nil` and `strings.Contains(err.Error(), "overlap detected")` and `strings.Contains(err.Error(), "1 group")` (to verify the count is interpolated).

2. `TestCheckOverlapAllowOverlapExitsZero`:
   - Same setup as above, except `overlapAllowOverlap = true` and `overlapNoAskToApply = true`.
   - Call `cmd.RunE(cmd, nil)`.
   - Assert `err == nil` (the report is still printed — confirmed by the `printInfoMessage` stub being called; that is a side-effect we do not need to assert explicitly).

3. `TestCheckOverlapExitsZeroOnEmptyReport`:
   - Set `overlapAllowOverlap = false` and `overlapNoAskToApply = true`.
   - Set `runOverlapAnalysis` to return an empty `overlap.Report{}` (no groups).
   - Call `cmd.RunE(cmd, nil)`.
   - Assert `err == nil`.

All three tests must restore their stashed vars in `t.Cleanup`. The `mockInstalledTool` helper and `stubSpinner` type are in the same `cmd` package and are directly accessible.

Naming: each test starts with `TestCheckOverlapExit` or `TestCheckOverlapAllowOverlap` so that `go test -run TestCheckOverlapExit` and `go test -run TestCheckOverlapAllowOverlap` select them as documented in the phase goal.

Do NOT touch any other tests in the file. Do NOT add new helper types or functions beyond what the three tests need.
</action>
<verify>
- `go test ./cmd/...` passes (all existing tests + the three new ones)
- `go test -run TestCheckOverlapExit ./cmd/...` runs `TestCheckOverlapExitsNonZeroOnOverlap` and `TestCheckOverlapExitsZeroOnEmptyReport` and both pass
- `go test -run TestCheckOverlapAllowOverlap ./cmd/...` runs `TestCheckOverlapAllowOverlapExitsZero` and it passes
- `go build ./...` succeeds
- `go vet ./cmd/...` passes
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./cmd/...` passes
- [ ] `TestCheckOverlapExitsNonZeroOnOverlap` passes
- [ ] `TestCheckOverlapAllowOverlapExitsZero` passes
- [ ] `TestCheckOverlapExitsZeroOnEmptyReport` passes
- [ ] `go build ./...` succeeds
- [ ] `skill check-overlap --help` shows the new `--allow-overlap` flag
- [ ] The flag's `var` is in the package-level `var ( ... )` block in `skill_overlap.go` (line 19-26 area)
- [ ] The exit-code check is placed **after** `printOverlapReport(...)` and **before** `if overlapNoAskToApply { return nil }`
- [ ] The error message is `fmt.Errorf("overlap detected: %d group(s) (use --allow-overlap to ignore)", len(report.Groups))`

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/cmd/skill_overlap.go packages/cli/cmd/skill_overlap_test.go`
2. Verify: `go test ./cmd/...` and `go build ./...` pass on the reverted state
3. Retry with a smaller scope: add only the var + flag (task 01-01 partial), run `go build ./...` to confirm the wire compiles, then add the exit-code check, then add the tests.

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | Exit-code check placed **after** `if overlapNoAskToApply { return nil }` instead of before it — `--no-ask-to-apply` then masks the non-zero exit. | Medium | High | The task explicitly dictates the placement (after `printOverlapReport`, before the `overlapNoAskToApply` block). `TestCheckOverlapExitsNonZeroOnOverlap` sets `overlapNoAskToApply = true` to prove the exit still fires when the early-return is active. |
| 2 | `pterm.Error` red banner on every "successful" CI failure — too noisy for a routine outcome. | Medium | Low | This is the established repo pattern (every cobra command returns `fmt.Errorf` to get exit 1). The error message is terse and actionable. A sentinel-error refactor in `main.go` is explicitly out of scope per the research. |
| 3 | Func-var leak between tests — a test sets `overlapAllowOverlap = true` and forgets `t.Cleanup`, breaking `TestCheckOverlapExitsNonZeroOnOverlap` in a later run. | Low | High | Every test stashes the original in a local var, sets the stub, and restores in a single `t.Cleanup` block. The test template is lifted from the existing `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode` which already follows this discipline. |
| 4 | Cobra flag registration uses the wrong default — `--allow-overlap` defaults to `true` accidentally, so the command never exits non-zero. | Low | High | Task explicitly sets `false` as the default. `TestCheckOverlapExitsNonZeroOnOverlap` and `TestCheckOverlapExitsZeroOnEmptyReport` both set `overlapAllowOverlap = false` explicitly to make the test independent of the default. |
| 5 | Spinner stub returns nil instead of `stubSpinner{}` — panic in `defer agenttools.ShowCursor()` or in `spinner.UpdateText(...)`. | Low | Medium | Task requires reuse of the existing `stubSpinner{}` helper at `skill_overlap_test.go:445-449` and the `agenttools.StartSpinnerFunc` swap pattern at line 346-348 of the same file. |
| 6 | The `fmt.Errorf` import was removed in a refactor and the new check breaks the build. | Very Low | Medium | `fmt` is used heavily throughout `skill_overlap.go`; removing it would already fail the existing build. No new import is needed for this change. |

## Commit Message

```
feat(cli): add --allow-overlap flag and non-zero exit to check-overlap

- New --allow-overlap cobra flag suppresses the non-zero exit on
  overlap findings (the report is still printed either way)
- New exit-code check after printOverlapReport and before the
  --no-ask-to-apply early-return: returns fmt.Errorf with the
  group count when overlap is found and the flag is absent
- Tests cover the three exit paths: groups without flag => non-zero
  error, groups with flag => nil, empty report => nil
- All existing tests in ./cmd/... still pass
```
