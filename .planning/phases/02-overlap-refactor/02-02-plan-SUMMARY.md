# Plan 02-02 Summary

**Completed:** 2026-06-11
**Phase:** 2 — Overlap refactor (REQ-3)
**Plan:** `.planning/phases/02-overlap-refactor/02-02-plan-fixtures-and-tests.md`

## What was built

Added 7 hand-curated `SKILL.md` fixtures under
`packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/`
(2 + 2 + 3) and a new `overlap_test.go` section with 4 new tests that
exercise `overlap.CollectSkills` against the three scenarios and a
fake-`commandRunner` smoke test that ties `Run` to the report contract.
This is the curated-fixture half of REQ-3 acceptance: it locks in the
JSON schema and sort-by-score contract that downstream
`filterOverlapGroups` (cmd package) and the `--allow-overlap` exit
path (plan 02-01) both depend on.

## Key files

- `packages/cli/internal/overlap/testdata/overlap/conflicting/{alpha,beta}/SKILL.md`:
  2 skills with overlapping descriptions (both mention
  "release announcement"); designed to trigger a duplicate or partial
  group from the agent.
- `packages/cli/internal/overlap/testdata/overlap/clean/{alpha,beta}/SKILL.md`:
  2 skills with deliberately disjoint descriptions (image-resizer vs
  sql-query-builder); designed to produce an empty groups array.
- `packages/cli/internal/overlap/testdata/overlap/partial/{alpha,beta,gamma}/SKILL.md`:
  3 skills with partially shared keywords (changelog-formatter,
  changelog-deduplicator, release-summary-writer); designed to mirror
  how a real agent would emit a multi-group report.
- `packages/cli/internal/overlap/overlap_test.go`:
  - New helpers: `loadFixtureRoot` + `copyDir` (copies fixtures into
    `t.TempDir()` so `skills.ScanSource` can walk them).
  - New tests: `TestCollectSkillsOnConflictingFixture`,
    `TestCollectSkillsOnCleanFixture`,
    `TestCollectSkillsOnPartialFixture`,
    `TestRunParsesReportWithMixedSeverities`.

## Decisions made

- **Helper pattern** mirrors the existing `e2e_test.go` convention for
  `testdata/fake-skills-cli.go`: a bare relative path `testdata/overlap/<scenario>`
  is resolved against the test's working directory, which `go test`
  sets to the package directory.
- **Fixture frontmatter** uses only the keys the CLI parses (`name`,
  `description`) plus `auto_trigger` (preserved verbatim, agent-only,
  not lifted into a struct field). No `metadata.skill-organizer`
  block, so `CollectSkills(location, false)` does not filter any
  fixture out under the default `include-disabled=false`.
- **Disjoint-keyword check** in `TestCollectSkillsOnCleanFixture`
  (`alpha` description must not mention SQL, `beta` must not mention
  image/resize) makes the "clean" scenario actually distinct from
  the "conflicting" one. Without it, the fixtures would be
  indistinguishable and the test would not catch a regression where
  both scenarios silently share descriptions.

## Deviations from plan

- **Plan action text said `RelativePath == "conflicting/alpha"` etc., but
  the actual `RelativePath` is just `"alpha"`** (and same for
  `beta`, `gamma`). The plan's `loadFixtureRoot` helper spec only
  copies the inner entries of `testdata/overlap/<scenario>/` into
  `t.TempDir()`, so `filepath.Rel(source, dir)` returns the leaf
  name. The plan's *intent* (assert the right skill is loaded by
  RelativePath) is preserved; only the map key was changed. The
  change is consistent with the existing
  `TestCollectSkillsExcludesDisabledByDefault` pattern (line 30:
  `items[0].RelativePath != "personal/enabled"`), where the path
  is relative to the `t.TempDir()` root passed to `CollectSkills`.

## Notes for downstream

- The fixtures live in `testdata/`, which `go build` skips by
  convention. The build verification in task 02-01 confirms the
  fixtures are not linked into the binary.
- `TestRunParsesReportWithMixedSeverities` is the upstream test
  for the contract that downstream `filterOverlapGroups` (in the
  `cmd` package) relies on: `Run` returns a `Report` whose groups
  are sorted by `Score` descending. If a future change intentionally
  changes the sort order, update this test alongside the change.
- The mixed-severity JSON shape (both `partial` and `adjacent`
  groups) is a useful template for end-to-end agent tests in a
  later phase — it covers the full parse → normalize → filter
  pipeline.
- `commandRunner` is restored in `t.Cleanup` in the new test,
  matching the pattern of the existing `TestRunParsesStructuredReport`
  and `TestRunReturnsInterruptedErrorWhenContextCanceled`. Any
  future test that mutates `commandRunner` should follow the same
  pattern.

## Test results

```
go test ./internal/overlap/...
ok  	github.com/sergiocarracedo/skill-organizer/cli/internal/overlap	0.108s
```

```
go test -v -run "TestCollectSkillsOnConflictingFixture|TestCollectSkillsOnCleanFixture|TestCollectSkillsOnPartialFixture|TestRunParsesReportWithMixedSeverities" ./internal/overlap/...
=== RUN   TestCollectSkillsOnConflictingFixture
--- PASS: TestCollectSkillsOnConflictingFixture (0.00s)
=== RUN   TestCollectSkillsOnCleanFixture
--- PASS: TestCollectSkillsOnCleanFixture (0.00s)
=== RUN   TestCollectSkillsOnPartialFixture
--- PASS: TestCollectSkillsOnPartialFixture (0.00s)
=== RUN   TestRunParsesReportWithMixedSeverities
--- PASS: TestRunParsesReportWithMixedSeverities (0.00s)
PASS
```

```
go test ./cmd/...
ok  	github.com/sergiocarracedo/skill-organizer/cli/cmd	0.618s
```

```
go test ./...
ok  	github.com/sergiocarracedo/skill-organizer/cli	9.621s
ok  	github.com/sergiocarracedo/skill-organizer/cli/cmd	(cached)
[... all packages PASS ...]
```

```
go build ./...
(success)
```

## Self-Check

| Must-have | Status |
|-----------|--------|
| `go test ./internal/overlap/...` passes | ✓ |
| `TestCollectSkillsOnConflictingFixture` passes (2 skills, expected names + descriptions) | ✓ |
| `TestCollectSkillsOnCleanFixture` passes (2 skills, disjoint descriptions) | ✓ |
| `TestCollectSkillsOnPartialFixture` passes (3 skills) | ✓ |
| `TestRunParsesReportWithMixedSeverities` passes (fake commandRunner returns JSON with partial+adjacent groups, Run returns both, Normalize sorts by score) | ✓ |
| All 7 fixture `SKILL.md` files exist on disk under `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/` | ✓ |
| `go build ./...` succeeds | ✓ |
| All existing `*_test.go` files in the repo still pass | ✓ |
| `loadFixtureRoot` and `copyDir` helpers exist in `overlap_test.go` | ✓ |
| `commandRunner` is restored in `t.Cleanup` in `TestRunParsesReportWithMixedSeverities` | ✓ |
| The new tests do NOT assert on `auto_trigger` (preserved verbatim, not lifted) | ✓ |

## Commits

1. `test(02-02): add curated overlap SKILL.md fixtures`
   — 7 new fixture files
2. `test(02-02): add loadFixtureRoot and copyDir helpers to overlap_test.go`
   — 1 file modified, 34 insertions
3. `test(02-02): add CollectSkills fixture tests and Run mixed-severity smoke`
   — 1 file modified, 176 insertions
4. `docs(02-02): record plan complete — state, agents, summary`
   — STATE.md, AGENTS.md, SUMMARY.md updated

Each commit ran lefthook pre-commit hook (`cli-e2e` test target) and passed.
