# Plan 01 Summary

**Completed:** 2026-06-11
**Phase:** 2 — Overlap refactor (REQ-3)

## What was built

Added the `--allow-overlap` cobra flag and a non-zero exit-code check
to `skill-organizer skill check-overlap`. By default, the command
prints the overlap report and exits non-zero when any overlap groups
remain; with `--allow-overlap` the report is still printed but the
exit code is zero. The check sits after `printOverlapReport(...)`
and before the `--no-ask-to-apply` early-return, so the report is
always visible and `--no-ask-to-apply` does not mask the new exit.

Three new tests in `packages/cli/cmd/skill_overlap_test.go` lock
down the three exit paths (groups + no flag => non-zero, groups +
flag => nil, empty report => nil). The existing
`TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
test was minimally updated to set `overlapAllowOverlap = true` so it
continues to exercise the apply-plan flow that the new check now
gates.

## Key files

- `packages/cli/cmd/skill_overlap.go` — added `overlapAllowOverlap`
  package var, registered `--allow-overlap` flag (default `false`),
  and inserted the 3-line exit-code check that returns
  `fmt.Errorf("overlap detected: %d group(s) (use --allow-overlap to ignore)", len(report.Groups))`.
- `packages/cli/cmd/skill_overlap_test.go` — added
  `TestCheckOverlapExitsNonZeroOnOverlap`,
  `TestCheckOverlapAllowOverlapExitsZero`, and
  `TestCheckOverlapExitsZeroOnEmptyReport`; minor
  `overlapAllowOverlap` wiring in the existing unsupported-tool
  test.

## Decisions made

- **Bool flag must be set after `newCheckOverlapCommand()`.** The
  plan's intended pattern of "set the var, then call
  `newCheckOverlapCommand()`, then `RunE`" silently fails for
  `--allow-overlap = true` because pflag's `newBoolValue` resets
  `*p` to the default on `BoolVar`. Each test therefore assigns
  the bool flag AFTER `newCheckOverlapCommand()` and before
  `RunE`; the same pattern was needed for
  `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`.
- **Existing test gets `overlapAllowOverlap = true`.** Strict
  reading of the plan says "Do NOT touch any other tests", but the
  must-have "go test ./cmd/... passes" is incompatible with leaving
  the existing test broken. The test's intent (exercise the
  apply-plan flow for an unsupported tool) is preserved by the
  flag-set; no assertions were changed.

## Deviations from plan

- The plan's verify check "The string `--allow-overlap` appears
  exactly once in `packages/cli/cmd/skill_overlap.go`" is
  contradicted by the plan's own action (the error message
  contains the flag name, and a comment also references it). The
  flag is registered, the error message references it, and the
  help text shows it; the behavioral intent of the verify check
  is satisfied. Recorded as a minor plan inconsistency, not a code
  change.
- The existing `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
  test was touched (stashed + set `overlapAllowOverlap = true`
  after `newCheckOverlapCommand()`) to keep `go test ./cmd/...`
  green. The plan's "Do NOT touch any other tests" guidance
  conflicts with its own "all existing tests still pass"
  must-have; the minimal patch preserves the test's intent.

## Notes for downstream

- Plan 02-02 (curated fixtures + overlap-package tests) is
  unblocked. It should land before or alongside this plan's
  follow-on because both depend on the same `overlap` package
  shape (`Report{Groups: []Group{...}}`).
- The error message string `"overlap detected: %d group(s) (use --allow-overlap to ignore)"`
  is the contract for downstream CLIs / CI integrations. A
  sentinel-error refactor in `main.go` is still out of scope per
  phase 02 RESEARCH; future plans can split the error from the
  exit code if a quieter "report-only" mode is desired.
