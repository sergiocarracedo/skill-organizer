# Phase 2 — Overlap refactor (REQ-3) — Context

**Gathered:** 2026-06-10
**Mode:** standard
**Status:** Ready for planning

<domain>
## Phase Boundary

`skill-organizer check-overlap` keeps its existing agent-driven
detection, but exits with a meaningful code, supports a
`--allow-overlap` flag, and gains curated-fixture test coverage.
The "refactor" deliverable from the original P2 scope (extracting the
shared agent-selection helper) was already shipped as a side-effect of
Phase 1 plan 02 — the overlap command already calls
`agenttools.ChooseAgentTool`, `agenttools.StartSpinner`, and
`agenttools.LaunchSession`. P2 therefore focuses on the test
coverage, exit semantics, and `--allow-overlap` flag that REQ-3
acceptance demands.

</domain>

<decisions>
## Implementation Decisions

### Detection source
- **Keep agent-driven detection.** The current `BuildPrompt` and
  `Run` flow in `packages/cli/internal/overlap/overlap.go` is the
  detection source. No deterministic local rule is added. (REQ-3
  spec says "curated fixture tests" — the fixtures cover the
  parser, filter, exit, and flag; the agent itself is verified by a
  single smoke test using a fake `commandRunner`.)

### Trigger-condition semantics
- **Both triggers and description, in the prompt.** `BuildPrompt`
  continues to pass `name`, `path`, `flattened-name`, and
  `description` for each skill. The Agent Skills spec's
  `auto_trigger` is read by the agent from the skill's frontmatter
  via the description (the existing fixture pattern shows
  `auto_trigger` as a sibling YAML key). The agent decides what
  counts as a conflicting trigger; the user-facing output reflects
  the agent's judgment.

### Output structure
- **Keep the existing `Report.Groups` shape.** The JSON schema
  stays:
  ```json
  {
    "summary": "string",
    "groups": [
      {
        "skill_names": ["string"],
        "skill_paths": ["string"],
        "score": 0,
        "why_overlap": "string",
        "overlap_type": "duplicate|partial|adjacent",
        "recommendation": "string"
      }
    ],
    "recommendations": ["string"]
  }
  ```
  A group of size 2 is reportable as a pair; groups of size 3+ are
  reportable as multiple pairs. REQ-3's "pairs" wording is satisfied
  because every group of size N contains `N*(N-1)/2` implied pairs.

### Exit code & `--allow-overlap` flag
- **Default behavior:** exit 0 if no overlap groups remain after
  filtering; exit 1 if any group remains.
- **`--allow-overlap`:** suppresses the non-zero exit. Always
  returns 0 regardless of overlap presence. The report is still
  printed.
- The flag is added to `skill_overlap.go` and wired into
  `newCheckOverlapCommand()`.

### Filtering & severity thresholds
- **Keep `--min-overlap-type`** with current values
  (`adjacent|partial|duplicate`) and **default `partial`**. No new
  flags.
- **Disabled skills** are filtered out by default; `--include-disabled`
  is the existing opt-in. No change.
- The agent's `score` (0-100) is preserved in the output but does
  not gate the exit code — only group presence does.

### Test fixture shape
- **Curated fixtures live in `packages/cli/internal/overlap/testdata/overlap/`.**
  Each fixture is a directory containing `SKILL.md` files
  representing overlapping and non-overlapping skills. Loaded via
  `t.TempDir()` + `skills.ScanSource` in the unit test, mirroring
  the existing e2e fixture style in `packages/cli/e2e_test.go`.
- At minimum: a `conflicting/` directory (2 skills with overlapping
  triggers) and a `clean/` directory (2 skills with no overlap).
  Optional extras: a `partial/` directory (3 skills, one shared
  trigger) for the partial-severity case.

### Test scope
- **Parse + filter + exit + flag.** Fixture tests cover
  `ParseReport` (curated JSON input), `filterOverlapGroups`
  (severity filter), exit code (with/without `--allow-overlap`),
  and the report printer. Agent detection is verified by a single
  smoke test that stubs `commandRunner` to return canned JSON,
  modeled on the existing `TestRunWithFakeCommand` in the security
  package.

### Agent's Discretion
- Exact wording of the curated JSON fixtures (skill names, paths,
  descriptions) — as long as they exercise overlapping and
  non-overlapping cases.
- The shape of the per-test data structures (slice, map, table-driven).
- How to wire `--allow-overlap` into the cobra command (flag
  variable + a single early-return before the exit-code check).

</decisions>

<specifics>
## Specific Ideas

- The user explicitly wants to keep the current overlap-detection
  flow as-is and have P2 focus on the parts REQ-3 acceptance still
  requires: exit code, `--allow-overlap`, and curated fixtures.
- The "refactor" deliverable in the original P2 scope is moot
  because P1 plan 02 already shipped it. Carry-forward to P2 from
  the P1 CONTEXT is the only meaningful refactor work, and it's
  already done.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `packages/cli/internal/overlap/overlap.go` — the package
  containing `BuildPrompt`, `Run`, `ParseReport`, `CollectSkills`,
  `Group`, `Report`, `filterOverlapGroups`, `parseMinOverlapType`.
  All helpers exist and are tested.
- `packages/cli/cmd/skill_overlap.go` — the cobra command. Already
  uses `agenttools.ChooseAgentTool`, `agenttools.StartSpinner`, and
  the shared `loadAgentSelectionConfigFunc`. New work: add
  `--allow-overlap` flag and the exit-code check.
- `packages/cli/internal/overlap/overlap.go:201-221` — `ParseReport`
  + clamping. The JSON schema is the contract; do not change.
- `packages/cli/internal/overlap/overlap.go:223-285` — `Normalize`
  and per-group normalization including `filterOverlapGroups`.
- `packages/cli/internal/security/security_test.go` — model for
  the fixture tests (`TestParseSecurityReport`,
  `TestRunWithFakeCommand`). Reuse the same shape.
- `.planning/phases/01-skill-security-check-CONTEXT.md` —
  carry-forward context. Confirms the shared `agenttools` package
  is the deliverable that P1 already shipped.
- `.planning/REQUIREMENTS.md` REQ-3 — the acceptance criteria being
  closed out in P2.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `overlap.ParseReport` (line 201) — JSON parsing, clamping,
  code-fence stripping. Reused by fixture tests.
- `overlap.filterOverlapGroups` (line 417) — severity filter by
  `overlap_type`. Reused by fixture tests.
- `overlap.Group.Normalize` (line 257) — clamps score, trims
  strings. Reused by fixture tests.
- `overlap.buildApplyPlanPrompt` (line 129) — already produces a
  plan-mode prompt for the agent. Untouched in P2.
- `agenttools.ChooseAgentTool` — already used by
  `skill_overlap.go:98`. Untouched in P2.
- `agenttools.StartSpinner`, `agenttools.LaunchSession` — already
  used by `skill_overlap.go`. Untouched in P2.
- `configpkg.AgentSelectionConfig` — the renamed
  `OverlapConfig`. `skill_overlap.go` already reads it via
  `loadAgentSelectionConfigFunc`. Untouched in P2.
- `commandRunner` (in `overlap.go:45`) — the swappable
  function variable for the agent. The smoke test will use the
  same `t.Cleanup` swap pattern used in the security package.

### Established Patterns
- **Func-var test injection:** every external dependency in
  `skill_overlap.go` is a package-level func var
  (`loadAgentSelectionConfigFunc`, `saveAgentSelectionConfigFunc`,
  `collectOverlapSkills`, `printOverlapPromptFunc`,
  `startSpinnerFunc`-via-`agenttools`, etc.). The new
  `--allow-overlap` flag is just a `var`, not a func var — but
  the exit-code logic reads it after the report is rendered.
- **Cobra flag registration:** every flag is added in
  `newCheckOverlapCommand()` near the existing flags
  (`--choose-tool`, `--tool`, `--include-disabled`,
  `--print-prompt`, `--min-overlap-type`, `--no-ask-to-apply`).
  The new `--allow-overlap` follows the same pattern.

### Integration Points
- `skill_overlap.go:165-172` — the section right after
  `printOverlapReport` is the natural place to insert the
  exit-code check (with `--allow-overlap` as a bypass). The flag
  declaration goes near the other flag registrations
  (line 218-223).
- The fixture directory at
  `packages/cli/internal/overlap/testdata/overlap/` is brand new.
  The test in `overlap_test.go` (a new file) reads it via
  `skills.ScanSource` on a `t.TempDir()` copy.

</code_context>

<deferred>
## Deferred Ideas

- **Local deterministic overlap rule** (rejected by the user for P2
  but worth keeping in mind). Could be a follow-up phase if the
  agent-driven approach proves too slow/costly/undeterministic.
- **Heuristic-weighted conflict scoring** beyond the agent's
  free-form `score` field. Out of REQ-3 scope.
- **Per-tool smoke test for check-overlap** across the full 5-tool
  matrix. Dropped for P2 the same way it was dropped for P1.
- **"Conflict severity" as a numeric cutoff.** The current spec
  uses the agent's `score` field, not an exit-code cutoff. If the
  user wants a `--min-score` flag later, that's a separate phase.

</deferred>

---

*Phase: 02-overlap-refactor*
*Context gathered: 2026-06-10*
