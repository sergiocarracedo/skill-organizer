# Phase 2 — Overlap refactor (REQ-3) — Discussion Log

> Audit trail of the `discuss-phase 2` conversation. Captures every
> option considered and the user's verbatim choice. NOT referenced
> by downstream agents — for human audit only.

## Phase goal (from ROADMAP.md)

> `skill-organizer check-overlap` uses the same shared
> agent-selection helper that P1 extracted, and surfaces conflict
> pairs with the offending trigger conditions.

## Carry-forward from P1 CONTEXT

- The shared `internal/agenttools` package was the P1 primary
  deliverable. P2 was supposed to swap `skill-overlap`'s chooser /
  cost-ack / spinner / launch to the shared helpers and delete the
  duplicates.
- **In practice, P1 plan 02 already did all of that.** The
  refactor section of P2 is moot.

## Scout findings

- `packages/cli/internal/overlap/overlap.go` (374 lines) —
  `BuildPrompt` is agent-driven, `Run` parses JSON report.
- `packages/cli/cmd/skill_overlap.go` (453 lines) — already uses
  `agenttools.ChooseAgentTool`, `agenttools.StartSpinner`,
  `agenttools.LaunchSession` after P1 plan 02.
- The current `Report.Groups` is list-of-groups, not list-of-pairs.
- The current command has no `--allow-overlap` flag and no
  enforced exit-code behavior. REQ-3 acceptance demands both.

## Gray areas discussed

### Area 1: Detection source
Options:
- **(A) Local rule only (drop agent) — recommended.** Trade-off:
  loses description-level overlap.
- **(B) Agent only (keep current).** Trade-off: cost, latency,
  non-determinism.
- **(C) Both, layered (local first, agent fallback).** Most
  coverage, most complexity.
- **(D) Both, opt-in via `--deep`.** Pragmatic, adds a flag.

**User chose: (B) Agent only (keep current).** No new detection
logic; the agent keeps doing the work.

### Area 2: Trigger-condition semantics
Options:
- **(A) Both triggers + description — recommended.** `BuildPrompt`
  continues to pass name/path/flattened-name/description. Agent
  interprets.
- (B) auto_trigger only, skip if absent.
- (C) auto_trigger, fallback to description.

**User chose: (A) Both triggers + description (Recommended).**
Unchanged from current behavior.

### Area 3: Output structure
Options:
- **(A) Keep groups (2+ skills) — recommended.** REQ-3's "pairs" is
  satisfied because a group of size 2 is a pair, and groups of size
  3+ contain multiple pairs.
- (B) Flatten to explicit pairs.
- (C) Rename groups to pairs (keep shape).

**User feedback (verbatim):** "I dont undestand this part and area 2:
I want to keep exactly the same as before to check the overlap but
using the common code we extracted."

**Resolution: keep the existing `Report.Groups` shape.** No schema
change. The "common code" the user refers to is the
`agenttools.ChooseAgentTool` / `StartSpinner` / `LaunchSession`
helpers from P1 plan 02, which are already wired up.

### Area 4: Exit code & `--allow-overlap` flag
Options:
- **(A) Always non-zero on any group, --allow-overlap→0 — recommended.**
  Matches the spec literally.
- (B) Non-zero only on duplicate/partial (not adjacent).
- (C) Score-based cutoff.

**User chose: (A) Always non-zero, --allow-overlap→0 (Recommended).**

### Area 5: Filtering & severity thresholds
Options:
- **(A) Keep current --min-overlap-type default=partial — recommended.**
- (B) Add score cutoff flag.
- (C) Confirm disabled-skills-default.
- (D) All-clear: no changes.

**User chose: (A) Keep current --min-overlap-type default=partial
(Recommended).** No filtering changes.

### Area 6: Test fixture shape
Options:
- **(A) testdata/ YAML files — recommended.** Mirrors the
  existing e2e fixture style.
- (B) Inline SkillInfo slices.
- (C) Real-world skills from a sample repo.
- (D) All-clear: just unit tests on ParseReport.

**User chose: (A) testdata/ YAML files (Recommended).**

### Follow-up probe: Fixture scope
With agent-driven detection, fixtures can only cover the
parser/filter/exit/flag and a smoke test on the agent.

Options:
- **(A) Parse + exit + flag — recommended.**
- (B) End-to-end with fake agent runner.
- (C) Unit only (no fixtures).

**User chose: (A) Parse + exit + flag (Recommended).**

## Areas delegated to agent's discretion

- Exact wording of the curated JSON fixtures (skill names, paths,
  descriptions) — as long as they exercise overlapping and
  non-overlapping cases.
- The shape of the per-test data structures (slice, map, table-driven).
- How to wire `--allow-overlap` into the cobra command (flag
  variable + a single early-return before the exit-code check).

## Deferred ideas

- **Local deterministic overlap rule.** Rejected for P2. Could be a
  follow-up phase if the agent-driven approach proves too slow /
  costly / undeterministic.
- **Heuristic-weighted conflict scoring** beyond the agent's
  free-form `score` field. Out of REQ-3 scope.
- **Per-tool smoke test** for check-overlap across the full 5-tool
  matrix. Dropped the same way it was dropped for P1.
- **"Conflict severity" as a numeric cutoff** (`--min-score` flag).
  Not in REQ-3.

---

*Phase: 02-overlap-refactor*
*Discussion captured: 2026-06-10*
