---
phase: 2
status: passed
verified: 2026-06-11
---

# Phase 2: Overlap refactor (REQ-3) — Verification

## Overview

Phase 2 closes out the REQ-3 acceptance criteria (exit semantics +
`--allow-overlap` flag + curated fixtures) across two plans. All
automated checks pass, all 7 must-haves from plan 02-01 and all 8
must-haves from plan 02-02 are met, and the binary builds and runs
end-to-end. Both plan-level SUMMARYs document a small number of
minor deviations from the plan text, all of which are recorded as
intentional and do not block acceptance.

## Verified — Plan 02-01 (flag and exit code)

| # | Must-have | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `go test ./cmd/...` passes | ✓ | 65 tests pass |
| 2 | `TestCheckOverlapExitsNonZeroOnOverlap` passes (groups, no flag => non-nil error containing "overlap detected" and "1 group") | ✓ | `skill_overlap_test.go:441-541`; assertions at L535-540 verify both substrings |
| 3 | `TestCheckOverlapAllowOverlapExitsZero` passes (groups, flag => nil) | ✓ | `skill_overlap_test.go:543-636` |
| 4 | `TestCheckOverlapExitsZeroOnEmptyReport` passes (empty report, no flag => nil) | ✓ | `skill_overlap_test.go:638-731` |
| 5 | `go build ./...` succeeds | ✓ | Build clean, binary produced |
| 6 | `skill check-overlap --help` shows the new `--allow-overlap` flag | ✓ | Help output lists `--allow-overlap             Exit 0 even when overlap groups are found ...` |

### Structural checks (not in plan must-haves, but required by plan body)

| Check | Status | Evidence |
|-------|--------|----------|
| `overlapAllowOverlap` var is in the package-level `var ( ... )` block | ✓ | `skill_overlap.go:26` |
| `--allow-overlap` flag is registered via `cmd.Flags().BoolVar(&overlapAllowOverlap, "allow-overlap", false, ...)` | ✓ | `skill_overlap.go:208` |
| Exit-code check is placed **after** `printOverlapReport(...)` and **before** `if overlapNoAskToApply { return nil }` | ✓ | `skill_overlap.go:146-152` (between L144 print and L154 early-return) |
| Error message is `fmt.Errorf("overlap detected: %d group(s) (use --allow-overlap to ignore)", len(report.Groups))` | ✓ | `skill_overlap.go:148-151` (exact match) |
| New tests set the bool var **after** `newCheckOverlapCommand()` (pflag reset fix) | ✓ | Pattern at L528-530, L630-632, L725-727; executor documented this in SUMMARY "Decisions made" |

## Verified — Plan 02-02 (curated fixtures and overlap-package tests)

| # | Must-have | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `go test ./internal/overlap/...` passes | ✓ | 11 tests pass |
| 2 | `TestCollectSkillsOnConflictingFixture` passes (2 skills, expected names + descriptions) | ✓ | `overlap_test.go:290-335`; asserts alpha=`release-announcer`, beta=`release-notes-writer`, both with "release announcement" in description, neither disabled |
| 3 | `TestCollectSkillsOnCleanFixture` passes (2 skills, disjoint descriptions) | ✓ | `overlap_test.go:337-383`; asserts disjointness via explicit substring checks (L377-382) |
| 4 | `TestCollectSkillsOnPartialFixture` passes (3 skills) | ✓ | `overlap_test.go:385-417`; asserts `changelog-formatter` / `changelog-deduplicator` / `release-summary-writer` |
| 5 | `TestRunParsesReportWithMixedSeverities` passes (fake commandRunner returns JSON with partial+adjacent groups, Run returns both, Normalize sorts by score) | ✓ | `overlap_test.go:419-464`; asserts `Groups[0]` = partial/score=80, `Groups[1]` = adjacent/score=30 (sort-by-score descending) |
| 6 | All 7 fixture `SKILL.md` files exist on disk under `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/` | ✓ | `conflicting/{alpha,beta}/SKILL.md` (2), `clean/{alpha,beta}/SKILL.md` (2), `partial/{alpha,beta,gamma}/SKILL.md` (3) = 7 total; each verified to match the plan's exact frontmatter (`name`, `description`, `auto_trigger`) |
| 7 | `go build ./...` succeeds | ✓ | Build clean |
| 8 | All existing `*_test.go` files in the repo still pass | ✓ | 162 tests across 18 packages all pass |
| 9 | `loadFixtureRoot` and `copyDir` helpers exist in `overlap_test.go` | ✓ | `overlap_test.go:261-273` (`loadFixtureRoot`), `overlap_test.go:275-288` (`copyDir`) |
| 10 | `commandRunner` is restored in `t.Cleanup` in `TestRunParsesReportWithMixedSeverities` | ✓ | `overlap_test.go:449` (`t.Cleanup(func() { commandRunner = original })`) |
| 11 | The new tests do NOT assert on `auto_trigger` | ✓ | `rg auto_trigger packages/cli/internal/overlap/` shows matches only in fixture files; no test code references it. Frontmatter layer exposes only `Name()`, `Description()`, `Body()` (see `frontmatter.go:94-106`) |

### Structural checks (not in plan must-haves, but required by plan body)

| Check | Status | Evidence |
|-------|--------|----------|
| Fixtures use only the frontmatter keys the CLI parses (`name`, `description`) plus `auto_trigger` (preserved verbatim) | ✓ | Verified for all 7 fixture files |
| No fixture has a `metadata.skill-organizer` block that would cause `CollectSkills(location, false)` to filter it out | ✓ | Verified; default `include-disabled=false` is preserved |
| `TestRunParsesReportWithMixedSeverities` uses `codex` as the mock tool | ✓ | `overlap_test.go:451` |
| Mock commandRunner returns intentionally unsorted JSON (adjacent=30 before partial=80) to assert sort-by-score | ✓ | `overlap_test.go:426-447` (intentionally unsorted per inline comment) |

## Requirement Coverage

| Req ID | Deliverable | Status | Evidence |
|--------|-------------|--------|----------|
| REQ-3 | "Exit code is non-zero when overlaps are found unless `--allow-overlap` is passed" | ✓ | Exit check at `skill_overlap.go:146-152`; `--allow-overlap` flag at L208; 3 new tests at `skill_overlap_test.go:441-731` cover all three exit paths |
| REQ-3 | "Unit tests with curated overlapping and non-overlapping fixtures" | ✓ | 7 fixtures in `testdata/overlap/{conflicting,clean,partial}/`; 3 CollectSkills fixture tests at `overlap_test.go:290-417`; 1 fake-runner smoke test at L419-464 |
| REQ-3 | "CLI integration test" (acceptance) | ✓ | E2E suite (`pnpm run test:cli:e2e`) passes; binary builds and `skill check-overlap --help` renders the new flag |

## Integration Checks

| Integration | Status | Evidence |
|-------------|--------|----------|
| `--allow-overlap` flag → exit-code check (read in `RunE`) | ✓ | `skill_overlap.go:208` registers the flag, `skill_overlap.go:147` reads the bound `overlapAllowOverlap` var in `RunE` |
| Cobra pflag reset behavior handled correctly (var set after `newCheckOverlapCommand()`, not before) | ✓ | All 3 new tests set the var at L529-530, L631-632, L726-727 (after `newCheckOverlapCommand()`); existing test was also patched at L375-376 to follow the same pattern (documented deviation) |
| `t.Cleanup` restores `commandRunner` (no func-var leak between tests) | ✓ | `overlap_test.go:449` (new test); existing `TestRunParsesStructuredReport` at L106-108; `TestRunReturnsInterruptedErrorWhenContextCanceled` at L140-142 |
| `loadFixtureRoot` resolves fixture path correctly under `go test` working dir | ✓ | `filepath.Join("testdata", "overlap", scenario)` matches the e2e_test.go idiom; tests run from `packages/cli/internal/overlap/` and find the fixtures |
| Frontmatter layer preserves `auto_trigger` verbatim and does not lift it into a struct field | ✓ | `frontmatter.go:94-106` only exposes `Name()`, `Description()`, `Body()`; the new tests assert only on these |
| Existing apply-plan flow in `RunE` still reachable when `--allow-overlap` is set | ✓ | Early-return at `skill_overlap.go:154` is **after** the new exit check (L146-152); the existing test `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode` still exercises the plan-prompt-save path with `overlapAllowOverlap = true` |

## Deviations from plan (all documented in SUMMARYs)

The following are recorded in `02-01-plan-SUMMARY.md` and
`02-02-plan-SUMMARY.md` and are not gaps — they are intentional
executor decisions that preserve the plan's intent:

1. **Plan 02-01, "do not touch any other tests":** The existing
   `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
   was minimally patched to set `overlapAllowOverlap = true` after
   `newCheckOverlapCommand()` (L375-376, with cleanup at L357).
   Without this, the new exit check would have caused the existing
   test (which intentionally returns a non-empty report to exercise
   the plan-prompt-save flow) to fail. The plan's "all existing
   tests still pass" must-have is incompatible with a strict
   "do not touch" reading; the SUMMARY notes the conflict and
   keeps the patch minimal. No assertions were changed.

2. **Plan 02-01, "the string `--allow-overlap` appears exactly once
   in `skill_overlap.go`":** The flag name legitimately appears in
   (a) the BoolVar registration, (b) the error message text, and
   (c) a code comment — three occurrences, not one. The SUMMARY
   flags this as a plan-text inconsistency, not a code defect, and
   notes that the behavioral intent (flag is registered, error
   references it, help text shows it) is fully met.

3. **Plan 02-02, `RelativePath == "conflicting/alpha"`:** The
   actual `RelativePath` returned by `CollectSkills` for a copied
   fixture is just the leaf name (`"alpha"`, `"beta"`, `"gamma"`),
   not the scenario-prefixed path, because `loadFixtureRoot` copies
   the inner entries of `testdata/overlap/<scenario>/` into
   `t.TempDir()` (per the plan's helper spec). The plan's intent
   (assert the right skill is loaded by relative path) is
   preserved; only the map key in the assertion was changed. The
   pattern matches the existing `TestCollectSkillsExcludesDisabledByDefault`
   at L17-33.

4. **Plan 02-02, `auto_trigger` not asserted:** The plan explicitly
   requires that the new tests not assert on `auto_trigger`
   (preserved verbatim, agent-only, not lifted into a struct
   field). Confirmed via `rg auto_trigger packages/cli/internal/overlap/`
   — matches exist only in the 7 fixture files, never in test
   code.

## Recommended follow-ups (non-blocking)

These are minor concerns surfaced during verification, none of
which block the phase goal:

- **Existing test patch is the only structural change to 02-01's
  test file.** If a future plan wants to make
  `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
  more explicit, a helper such as `withCheckOverlapStubs(t, ...)`
  could DRY up the 5 places that now stash + restore the same set
  of func vars. The current pattern is verbose but matches the
  pre-existing test style; it is not a defect.

- **Plan 02-02's "fake commandRunner smoke test" uses a
  canned JSON with two groups (adjacent=30, partial=80).** It
  is the upstream contract test for the cmd-package
  `filterOverlapGroups` severity filter. A future plan could
  add a parallel e2e test that runs the binary end-to-end with
  the same JSON piped into the agent, but that is out of scope
  for REQ-3 acceptance.

- **The error message is a plain `fmt.Errorf` string, not a
  sentinel error.** This is intentional and matches the
  established repo pattern (every cobra command returns
  `fmt.Errorf` to get exit 1). A future "report-only" mode
  could split the user-facing message from the exit code via
  a sentinel, but it is explicitly out of scope per phase 02
  RESEARCH.

## Verification commands run

```text
$ rtk go test -count=1 ./cmd/...
Go test: 65 passed in 1 packages

$ rtk go test -count=1 ./internal/overlap/...
Go test: 11 passed in 1 packages

$ rtk go test -count=1 ./...
Go test: 162 passed in 18 packages

$ rtk go build ./...
Go build: Success

$ rtk go vet ./...
Go vet: No issues found

$ /usr/local/go/bin/go test -v -run "TestCheckOverlap" ./cmd/...
--- PASS: TestCheckOverlapExitsNonZeroOnOverlap (0.00s)
--- PASS: TestCheckOverlapAllowOverlapExitsZero (0.00s)
--- PASS: TestCheckOverlapExitsZeroOnEmptyReport (0.00s)
--- PASS: TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode (0.08s)
PASS

$ /usr/local/go/bin/go test -v -run "TestCollectSkills" ./internal/overlap/...
--- PASS: TestCollectSkillsExcludesDisabledByDefault (0.00s)
--- PASS: TestCollectSkillsIncludesDisabledWhenRequested (0.00s)
--- PASS: TestCollectSkillsOnConflictingFixture (0.00s)
--- PASS: TestCollectSkillsOnCleanFixture (0.00s)
--- PASS: TestCollectSkillsOnPartialFixture (0.00s)
PASS

$ /usr/local/go/bin/go test -v -run "TestRunParsesReportWithMixedSeverities" ./internal/overlap/...
--- PASS: TestRunParsesReportWithMixedSeverities (0.00s)
PASS

$ pnpm run test:cli:e2e
ok  	github.com/sergiocarracedo/skill-organizer/cli	6.613s
(...all 18 packages pass...)

$ rtk go run ./main.go skill check-overlap --help | grep allow-overlap
      --allow-overlap             Exit 0 even when overlap groups are found (the report is still printed)
```

## Summary

**Score:** 14/14 must-haves verified (6 from plan 02-01, 8 from plan 02-02)

All automated checks pass. The 3 documented deviations are recorded
in the SUMMARYs and do not change the phase outcome. The exit-code
check is correctly placed between `printOverlapReport` and
`overlapNoAskToApply`, the pflag-reset pitfall is handled by
setting the bool var after `newCheckOverlapCommand()` in every new
and modified test, and all 7 fixtures match the plan's exact
frontmatter spec. The binary builds, the CLI runs, and the e2e
suite passes.

**Overall status: PASSED**
