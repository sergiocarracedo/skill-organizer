---
wave: 2
depends_on:
  - 02-01-plan-flag-and-exit-code.md
files_modified:
  - packages/cli/internal/overlap/testdata/overlap/conflicting/alpha/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/conflicting/beta/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/clean/alpha/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/clean/beta/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/partial/alpha/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/partial/beta/SKILL.md
  - packages/cli/internal/overlap/testdata/overlap/partial/gamma/SKILL.md
  - packages/cli/internal/overlap/overlap_test.go
autonomous: true
single_layer_justified: true
single_layer_justified_reason: "Test/fixture coverage for the overlap package — per CONTEXT.md 'Test scope: Parse + filter + exit + flag' carve-out, this plan only adds test data and test code. The user-facing CLI change (flag + exit code) lives in Plan 02-01; this plan is a horizontal test-infrastructure layer that locks in the parser/filter contract using curated fixtures."
requirement: REQ-3
objective: "Add curated SKILL.md fixtures for three overlap scenarios (conflicting, clean, partial) and the corresponding unit tests that exercise CollectSkills against each fixture and a fake-commandRunner smoke test that ties Run to a mixed-severity canned report."
must_haves:
  - "go test ./internal/overlap/... passes"
  - "TestCollectSkillsOnConflictingFixture passes (2 skills, expected names + descriptions)"
  - "TestCollectSkillsOnCleanFixture passes (2 skills, disjoint descriptions)"
  - "TestCollectSkillsOnPartialFixture passes (3 skills)"
  - "TestRunParsesReportWithMixedSeverities passes (fake commandRunner returns JSON with partial+adjacent groups, Run returns both, Normalize sorts by score)"
  - "All 7 fixture SKILL.md files exist on disk under packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/"
  - "go build ./... succeeds"
  - "All existing *_test.go files in the repo still pass"
---

# Plan 02: Curated fixtures and overlap package tests

## Objective

Add the curated `SKILL.md` fixtures committed to `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/` (the input data the agent sees in real usage) and the corresponding unit tests in `packages/cli/internal/overlap/overlap_test.go`. The tests exercise `overlap.CollectSkills` against each fixture root via a `t.TempDir()` copy, plus a fake-`commandRunner` smoke test that returns canned JSON containing both `partial` and `adjacent` groups so we lock in the contract that `Run` parses, normalizes (sorts by score, trims empty groups), and preserves both severities. This delivers the curated-fixture half of REQ-3 acceptance and locks in the JSON contract for downstream tests.

## Context

`overlap.CollectSkills` (`packages/cli/internal/overlap/overlap.go:47-88`) calls `skills.ScanSource(location.Source)` which expects a real directory tree containing `SKILL.md` files. To exercise it against hand-curated fixtures, the test reads each fixture `SKILL.md` from `testdata/overlap/<scenario>/<skill>/SKILL.md` and copies it into a `t.TempDir()` (because `ScanSource` is a real directory walker, not a virtual one). The fixture directory is brand new — Go's `testdata/` is reserved for test inputs and is automatically skipped by `go build`, so placing the files there is the correct convention (`packages/cli/internal/overlap/testdata/overlap/...`).

The frontmatter layer (`packages/cli/internal/skills/frontmatter.go:94-106`) reads only `name` and `description` from the top level, plus `metadata.skill-organizer` for managed fields; everything else is preserved verbatim and ignored by the CLI. `auto_trigger:` is an agent-only key and is preserved but not lifted into a struct field. Tests must assert on `Name()`, `Description()`, and the relative path; they must NOT assert on `auto_trigger` being lifted.

The existing fake-`commandRunner` pattern is in `TestRunParsesStructuredReport` (`overlap_test.go:83-132`); the new mixed-severities test follows it and additionally checks that `report.Normalize()` sorts groups by `Score` descending (line 233-235 of `overlap.go`).

`filterOverlapGroups` lives in the `cmd` package (`skill_overlap.go:329-338`), not the `overlap` package — its existing test is `TestFilterOverlapGroupsHidesAdjacentByDefault` (`skill_overlap_test.go:206-220`) and stays untouched. The "smoke test that ties Run to the report contract" is satisfied by a new test in the `overlap` package that asserts both severities survive `ParseReport` + `Normalize` (which is the upstream of any downstream filter).

## Tasks

<task id="02-01">
<name>Create the 7 fixture SKILL.md files under testdata/overlap/{conflicting,clean,partial}/</name>
<files>
- packages/cli/internal/overlap/testdata/overlap/conflicting/alpha/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/conflicting/beta/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/clean/alpha/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/clean/beta/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/partial/alpha/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/partial/beta/SKILL.md
- packages/cli/internal/overlap/testdata/overlap/partial/gamma/SKILL.md
</files>
<action>
Create the following 7 fixture files. Each file's frontmatter has `name:` and `description:` (the only two fields the CLI parses), plus an `auto_trigger:` block (preserved verbatim, parsed by the agent, NOT asserted on in tests). Each body is a single Markdown H1.

1. `packages/cli/internal/overlap/testdata/overlap/conflicting/alpha/SKILL.md`:
   ```markdown
   ---
   name: release-announcer
   description: Run when the user asks to draft a release announcement for a new software release. Produces a marketing-ready blurb.
   auto_trigger:
     - keywords: ["release announcement", "release notes", "ship a release"]
   ---

   # release-announcer

   Drafts release announcements from the changelog.
   ```

2. `packages/cli/internal/overlap/testdata/overlap/conflicting/beta/SKILL.md`:
   ```markdown
   ---
   name: release-notes-writer
   description: Run when the user asks to draft a release announcement for a new software release. Outputs a customer-facing summary.
   auto_trigger:
     - keywords: ["release announcement", "release notes", "write release notes"]
   ---

   # release-notes-writer

   Writes customer-facing release notes.
   ```

3. `packages/cli/internal/overlap/testdata/overlap/clean/alpha/SKILL.md`:
   ```markdown
   ---
   name: image-resizer
   description: Run when the user wants to resize, crop, or convert an image file to a different format or resolution.
   auto_trigger:
     - keywords: ["resize image", "crop image", "convert image"]
   ---

   # image-resizer

   Resizes and converts images.
   ```

4. `packages/cli/internal/overlap/testdata/overlap/clean/beta/SKILL.md`:
   ```markdown
   ---
   name: sql-query-builder
   description: Run when the user wants to compose a safe parameterized SQL query from a plain-English request and the current database schema.
   auto_trigger:
     - keywords: ["write sql", "build query", "compose query"]
   ---

   # sql-query-builder

   Builds parameterized SQL queries.
   ```

5. `packages/cli/internal/overlap/testdata/overlap/partial/alpha/SKILL.md`:
   ```markdown
   ---
   name: changelog-formatter
   description: Run when the user asks to format a raw changelog into a clean grouped list for the release notes.
   auto_trigger:
     - keywords: ["format changelog", "group changelog", "tidy changelog"]
   ---

   # changelog-formatter

   Formats raw changelogs into grouped lists.
   ```

6. `packages/cli/internal/overlap/testdata/overlap/partial/beta/SKILL.md`:
   ```markdown
   ---
   name: changelog-deduplicator
   description: Run when the user asks to dedupe and merge duplicate entries in a changelog before formatting it.
   auto_trigger:
     - keywords: ["dedupe changelog", "merge changelog", "tidy changelog"]
   ---

   # changelog-deduplicator

   Dedupes and merges changelog entries.
   ```

7. `packages/cli/internal/overlap/testdata/overlap/partial/gamma/SKILL.md`:
   ```markdown
   ---
   name: release-summary-writer
   description: Run when the user wants a one-paragraph executive summary of the latest changelog for a release announcement.
   auto_trigger:
     - keywords: ["summarize changelog", "executive summary", "release summary"]
   ---

   # release-summary-writer

   Writes one-paragraph changelog summaries.
   ```

The descriptions in `conflicting/alpha` and `conflicting/beta` are intentionally overlapping (both mention "release announcement"). The descriptions in `clean/alpha` and `clean/beta` are deliberately disjoint. The `partial` set shares keywords between alpha+beta ("tidy changelog") and beta+gamma ("dedupe changelog", "merge changelog") to mirror how a real agent would emit a multi-group report.

Do NOT add a `.gitignore` for `testdata/`. Do NOT add a metadata block to any fixture. Do NOT include YAML keys that the frontmatter layer would reject.
</action>
<verify>
- `ls packages/cli/internal/overlap/testdata/overlap/conflicting/alpha/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/conflicting/beta/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/clean/alpha/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/clean/beta/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/partial/alpha/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/partial/beta/SKILL.md` exists
- `ls packages/cli/internal/overlap/testdata/overlap/partial/gamma/SKILL.md` exists
- `go build ./...` succeeds (the testdata/ files are skipped by the build)
</verify>
<done>[ ]</done>
</task>

<task id="02-02">
<name>Add fixture-loading helpers to overlap_test.go</name>
<files>
- packages/cli/internal/overlap/overlap_test.go
</files>
<action>
Append two helper functions to the end of `packages/cli/internal/overlap/overlap_test.go` (after the existing `createSkill` helper at line 231-249 and `mockInstalledTool` helper at line 251-254):

1. `loadFixtureRoot(t *testing.T, scenario string) string`:
   - `t.Helper()`.
   - Allocate `root := t.TempDir()`.
   - Set `src := filepath.Join("testdata", "overlap", scenario)`.
   - Read `src` with `os.ReadDir`; on error, `t.Fatalf("read fixture dir %q: %v", src, err)`.
   - For each entry, call `copyDir(t, filepath.Join(src, entry.Name()), filepath.Join(root, entry.Name()))`.
   - Return `root`.

2. `copyDir(t *testing.T, src, dst string)`:
   - `t.Helper()`.
   - `os.MkdirAll(dst, 0o755)`; on error, `t.Fatalf`.
   - `skillFile := filepath.Join(src, skills.SkillFileName)` (use the existing exported `skills.SkillFileName = "SKILL.md"` constant from `internal/skills/scanner.go:13`).
   - `data, err := os.ReadFile(skillFile)`; on error, `t.Fatalf`.
   - `os.WriteFile(filepath.Join(dst, skills.SkillFileName), data, 0o644)`; on error, `t.Fatalf`.

Both helpers use `t.TempDir()` and `t.Fatalf` only — no `t.Cleanup` is needed because `t.TempDir` is cleaned up automatically by the testing framework.

The path `testdata/overlap/<scenario>` is resolved relative to the test's working directory, which `go test ./internal/overlap/...` sets to `packages/cli/internal/overlap/`. This matches the convention used by `packages/cli/e2e_test.go:418-421` for `testdata/fake-skills-cli.go`.

The `skills` package is already imported in the file (line 14: `"github.com/sergiocarracedo/skill-organizer/cli/internal/skills"`), so no new imports are needed.
</action>
<verify>
- The file compiles: `go build ./internal/overlap/...` succeeds
- `go test -run NONE ./internal/overlap/...` passes (no-op test compile check)
</verify>
<done>[ ]</done>
</task>

<task id="02-03">
<name>Add three CollectSkills fixture tests and one Run smoke test</name>
<files>
- packages/cli/internal/overlap/overlap_test.go
</files>
<action>
Append the following four tests to `packages/cli/internal/overlap/overlap_test.go`, after the helpers added in task 02-02:

1. `TestCollectSkillsOnConflictingFixture`:
   - `root := loadFixtureRoot(t, "conflicting")`.
   - Call `items, err := CollectSkills(configpkg.Location{Source: root, Target: filepath.Join(root, "target")}, false)`.
   - Assert `err == nil`, `len(items) == 2`.
   - Build a `map[string]overlap.SkillInfo` from the items by `RelativePath` and assert:
     - The skill at `conflicting/alpha` has `Name == "release-announcer"`, `Description` contains "release announcement", `Disabled == false`.
     - The skill at `conflicting/beta` has `Name == "release-notes-writer"`, `Description` contains "release announcement", `Disabled == false`.

2. `TestCollectSkillsOnCleanFixture`:
   - `root := loadFixtureRoot(t, "clean")`.
   - Call `CollectSkills(location, false)`.
   - Assert `err == nil`, `len(items) == 2`.
   - Build a map and assert:
     - `clean/alpha` has `Name == "image-resizer"`, `Description` contains "resize".
     - `clean/beta` has `Name == "sql-query-builder"`, `Description` contains "SQL".
   - The two descriptions must be **disjoint** (i.e., neither contains the other skill's keyword) — assert that with a substring check.

3. `TestCollectSkillsOnPartialFixture`:
   - `root := loadFixtureRoot(t, "partial")`.
   - Call `CollectSkills(location, false)`.
   - Assert `err == nil`, `len(items) == 3`.
   - Build a map and assert all three expected relative paths and names are present:
     - `partial/alpha` → `changelog-formatter`
     - `partial/beta` → `changelog-deduplicator`
     - `partial/gamma` → `release-summary-writer`

4. `TestRunParsesReportWithMixedSeverities`:
   - Stash the original `commandRunner`: `original := commandRunner`.
   - Replace it with a closure that returns a canned JSON containing BOTH a `partial` group and an `adjacent` group, in unsorted order so we can assert the sort-by-score behavior of `Normalize`:
     ```json
     {
       "summary": "Mixed overlap detected.",
       "groups": [
         {
           "skill_names": ["a", "b"],
           "skill_paths": ["thirdparty/a", "thirdparty/b"],
           "score": 30,
           "why_overlap": "low overlap",
           "overlap_type": "adjacent",
           "recommendation": "keep separate"
         },
         {
           "skill_names": ["c", "d"],
           "skill_paths": ["thirdparty/c", "thirdparty/d"],
           "score": 80,
           "why_overlap": "shared trigger",
           "overlap_type": "partial",
           "recommendation": "merge"
         }
       ],
       "recommendations": ["Review c and d."]
     }
     ```
   - `t.Cleanup(func() { commandRunner = original })`.
   - Call `result, err := Run(context.Background(), mockInstalledTool("codex", "codex"), "prompt", nil)`.
   - Assert `err == nil`, `len(result.Groups) == 2`.
   - Assert `result.Groups[0].OverlapType == "partial"` and `result.Groups[0].Score == 80` (proving `Normalize` sorted by score descending).
   - Assert `result.Groups[1].OverlapType == "adjacent"` and `result.Groups[1].Score == 30`.
   - This is the smoke test that locks in the JSON contract: both `partial` and `adjacent` survive `ParseReport` + `Normalize`, and the sort order is correct. The downstream `filterOverlapGroups` (in the `cmd` package) can then drop the `adjacent` group when `min-overlap-type=partial`.

The existing `mockInstalledTool` helper at the bottom of the file (line 251-254) is reused as-is. The existing imports in the file already cover `context`, `os`, `path/filepath`, `strings`, `testing`, `agenttools`, `configpkg`, `skills` — no new imports are required.

All four tests must be added at the end of the file (after the helpers from task 02-02). The existing `TestRunParsesStructuredReport` and other tests stay untouched.
</action>
<verify>
- `go test ./internal/overlap/...` passes
- `go test -run TestCollectSkillsOnConflictingFixture ./internal/overlap/...` passes
- `go test -run TestCollectSkillsOnCleanFixture ./internal/overlap/...` passes
- `go test -run TestCollectSkillsOnPartialFixture ./internal/overlap/...` passes
- `go test -run TestRunParsesReportWithMixedSeverities ./internal/overlap/...` passes
- `go build ./...` succeeds
- `go test ./...` passes (all 27 existing test files in the repo still pass)
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./internal/overlap/...` passes
- [ ] `TestCollectSkillsOnConflictingFixture` passes
- [ ] `TestCollectSkillsOnCleanFixture` passes
- [ ] `TestCollectSkillsOnPartialFixture` passes
- [ ] `TestRunParsesReportWithMixedSeverities` passes
- [ ] All 7 fixture `SKILL.md` files exist on disk under `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/`
- [ ] `go build ./...` succeeds
- [ ] All existing `*_test.go` files in the repo still pass
- [ ] `loadFixtureRoot` and `copyDir` helpers exist in `overlap_test.go`
- [ ] `commandRunner` is restored in `t.Cleanup` in `TestRunParsesReportWithMixedSeverities`
- [ ] The new tests do NOT assert on `auto_trigger` (which is preserved verbatim by the frontmatter layer but not lifted into a struct field)

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/internal/overlap/overlap_test.go` and `rm -rf packages/cli/internal/overlap/testdata/overlap/`
2. Verify: `go test ./...` and `go build ./...` pass on the reverted state
3. Retry with a smaller scope:
   - Add only the fixtures (task 02-01), run `ls -R packages/cli/internal/overlap/testdata/overlap/` to verify all 7 files are present
   - Then add only the helpers (task 02-02), run `go test -run NONE ./internal/overlap/...` to verify compilation
   - Then add the tests (task 02-03) one at a time, running the matching `go test -run TestName` after each

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | Fixture YAML includes a `metadata.skill-organizer:` block with `disabled: true`, and `CollectSkills(location, false)` filters it out, breaking the test that expects `len(items) == 2`. | Low | Medium | Task 02-01 explicitly forbids adding a metadata block. The fixtures use only `name`, `description`, and `auto_trigger`. |
| 2 | Test reads `testdata/overlap/...` via a relative path that resolves from the repo root instead of the package dir, so the test fails when run with `go test ./...` from the root. | Low | High | The path `filepath.Join("testdata", "overlap", scenario)` is a bare relative path; `go test ./internal/overlap/...` sets the working dir to the package dir, matching the existing `e2e_test.go:418-421` idiom for `testdata/fake-skills-cli.go`. |
| 3 | `commandRunner` is replaced in `TestRunParsesReportWithMixedSeverities` but the original is not restored — a subsequent test panics on a nil-binary call or sees leaked state. | Low | High | Task 02-03 explicitly requires `t.Cleanup(func() { commandRunner = original })`. The pattern is lifted from the existing `TestRunParsesStructuredReport` (line 106-108). |
| 4 | `Normalize` changes the sort order in a future refactor, and `TestRunParsesReportWithMixedSeverities` starts failing for an unrelated reason. | Low | Low | The sort-by-score-descending contract is documented in `overlap.go:233-235`; the test asserts the contract directly. If a future change intentionally changes the sort order, that test should be updated alongside the change. |
| 5 | Fixture files end up in the binary because `go build` does not skip them — linker error or warning. | Very Low | Medium | Go's spec skips `testdata/` directories during compilation (`packages/cli/testdata/fake-skills-cli.go` is committed for the same reason). The build verification step in task 02-01 catches this if it ever changes. |
| 6 | The 7 fixtures all share the same `auto_trigger` keywords, making the test indistinguishable from one that uses disjoint keywords. | Low | Low | Task 02-01 specifies disjoint keywords for `clean/` and overlapping keywords for `conflicting/`. The `TestCollectSkillsOnCleanFixture` test additionally asserts that the two descriptions are disjoint via substring checks, locking in the distinction. |
| 7 | `loadFixtureRoot` reads entries with `os.ReadDir` but skips dotfiles, so a `.DS_Store` on macOS silently removes a fixture. | Low | Low | `os.ReadDir` returns entries sorted and does NOT skip dotfiles. If a developer has a stray dotfile in the fixture tree, `copyDir` will try to read a `SKILL.md` from it and fail loudly with `t.Fatalf`. The test fails noisily, not silently. |
| 8 | `TestRunParsesReportWithMixedSeverities` is named ambiguously so that `go test -run TestRun ./internal/overlap/...` matches both old and new tests, slowing the TDD loop. | Low | Low | The test name is intentionally specific. `-run TestRunParsesReportWithMixedSeverities` selects only the new test. Use that exact name when iterating. |

## Commit Message

```
test(overlap): add curated fixtures and parser/filter smoke tests

- Add testdata/overlap/{conflicting,clean,partial}/ with 7 SKILL.md
  fixtures (overlapping and disjoint descriptions, designed for the
  agent's auto_trigger judgment)
- Add loadFixtureRoot + copyDir helpers in overlap_test.go to copy
  fixtures into t.TempDir() so skills.ScanSource can walk them
- Add TestCollectSkillsOnConflictingFixture, TestCollectSkillsOnCleanFixture,
  TestCollectSkillsOnPartialFixture: assert CollectSkills returns the
  expected name/description/path for each scenario
- Add TestRunParsesReportWithMixedSeverities: fake commandRunner
  returns JSON with both partial and adjacent groups; assert Run
  parses both and Normalize sorts by score descending (upstream of
  the cmd-package filterOverlapGroups)
- go test ./internal/overlap/... and go test ./... both pass
```