# Phase 1 — Skill security check (REQ-4) — CONTEXT

> Decisions captured during `discuss-phase 1` (standard mode) on
> 2026-06-10. These are the inputs the planner will use to write
> `phases/01-skill-security-check-PLAN.md`.

## Goal (from ROADMAP.md)

A user can run `skill-organizer check-security` and get a per-skill
risk score; high-risk skills prompt to be disabled; the score is
persisted and gates re-enable. Includes extracting a shared
agent-selection helper that P2 (overlap refactor) will re-use.

## Refactor boundary (a)

- **Extract into a shared `internal/agenttools` package:**
  - The chooser flow: `chooseOverlapTool` + `selectInstalledTool`.
  - Cost-acknowledgment (so security can prompt the same way).
  - Spinner + launch helpers (so security can run the agent
    in the foreground the same way overlap does).
- **Test injection:** two package-level function variables —
  `chooseAgentToolFunc` and `selectInstalledToolFunc` — to keep
  the existing test pattern (`t.Cleanup` swap).
- **Renames** as part of the move: `chooseOverlapTool` →
  `chooseAgentTool`; `selectInstalledTool` keeps its name but
  moves package.

## Config naming (b)

- Rename `OverlapConfig` → `AgentSelectionConfig`.
- Move it out of `AppConfig.Overlap` to `AppConfig.AgentSelection`
  in the `AppConfig` struct (`packages/cli/internal/config/config.go`).
- YAML migration: on read, if `agent-selection.*` is missing, fall
  back to `overlap.*` and use those values. Do **not** auto-rewrite
  the file — the next write goes to `agent-selection.*` and old keys
  get left behind (acceptable; users can clean up manually).
- All call sites of `LoadOverlapConfigOrDefault` / `SaveOverlapConfig`
  move to `LoadAgentSelectionConfigOrDefault` /
  `SaveAgentSelectionConfig`. The existing `overlap_test.go` is
  rewritten against the new helpers.

## Risk-score storage & schema (c)+(d)

- **Storage:** in `ManagedMetadata` (per-skill frontmatter under
  `metadata.skill-organizer.*`). Co-located with `Disabled`,
  `InstalledAt`, etc.
- **Schema (minimal):**
  - `risk-score` (int, 0–100)
  - `risk-evaluated-at` (RFC3339 string)
  - `risk-evaluator` (string, e.g. `claude-code:1.2.3`; empty when
    the skill is unevaluated)
  - `risk-reason` (one-line free text)
- **Default threshold:** **70**. Configurable later via flag; not
  a P1 requirement.
- **Parallel changes required:**
  - `ManagedMetadata` struct in
    `packages/cli/internal/skills/frontmatter.go`.
  - The `Skill` struct in
    `packages/cli/internal/skills/scanner.go:15` (so callers can
    read the score without parsing frontmatter).
  - `mergeManagedMetadata` / `updateManagedMetadata` helpers to
    round-trip the new fields.
  - `SetManagedFields` writer at `frontmatter.go:159`.

## UX & skill-add hook (e)+(f)+(g)

- **Invocation model (e):** **interactive, mirror overlap.** When
  a tool is available, run the agent in the foreground via the
  extracted spinner + launch helpers. Print-prompt is reserved
  for the no-tools-detected fallback.
- **No-tools-detected fallback (f):** print the analysis prompt
  to stdout and **exit 0**. The user pipes it into their tool
  manually. Differs from `skill-overlap`'s current hard-error
  behavior on the same condition.
- **Skill-add hook (g):** after a successful install, prompt
  *"Run check-security for `<name>`?"* with **default = yes**.
  - On yes: run `check-security` inline.
  - On no: write `risk-evaluator: ""` to mark the skill
    *unevaluated* so the next `check-security` run picks it up.
- **Re-enable gate:** if `risk-score >= 70` and
  `risk-evaluator != ""` on a skill the user is enabling, the
  `enable` command:
  1. Shows the `risk-reason`.
  2. Asks for confirmation, **default = no**.
  3. On *no*: **explicitly write `disabled: true`** to the
     metadata (don't just leave it as-is). The skill stays off.
  4. On *yes*: enable despite the risk; the score and reason
     remain in metadata.

## Out of scope for P1 (carried from spec, not decided here)

- **Analysis prompt wording** — user-flagged TBD. Will be a short
  discovery spike before plan-phase locks the prompt. Not part of
  this CONTEXT.
- Heuristic-weighted or remote-reputation scoring — follow-up
  phase; P1 ships static-only in the agent's analysis.
- Configurable threshold — fixed at 70 in P1.
- Per-tool smoke test for check-security across the full
  5-tool matrix — single-tool smoke only; the matrix coverage
  was dropped in planning.

## Carry-forward to P2 (Overlap refactor)

- The shared `internal/agenttools` package is the primary
  deliverable. P2 will swap `skill-overlap`'s chooser / cost-ack /
  spinner / launch to the shared helpers and delete the
  duplicates.
- The new `AppConfig.AgentSelection` is what overlap will read
  from in P2.

## Open questions after discussion

- None blocking. The prompt-wording discovery spike is the only
  remaining input, and it's flagged as a small pre-plan-phase step.
