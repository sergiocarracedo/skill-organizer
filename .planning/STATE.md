# STATE.md

> Single source of truth for the current project state. Read this
> before any work to know what's done, what's next, and what
> constraints are in play.

## Current Phase

**Phase 2 — Overlap refactor (REQ-3)** — plans ready, 2026-06-10.

2 plans committed (`6ce4549`):
- 02-01: `--allow-overlap` flag + non-zero exit code
- 02-02: curated fixtures + overlap-package tests (single_layer_justified: true)

Phase 1 (Skill security check, REQ-4) complete on 2026-06-10 with
all 4 plans executed, ~30 new tests, ~12 files changed.

Phase 2 discuss-phase completed 2026-06-10:
- Detection source: keep agent-driven (no new local rule)
- Trigger semantics: name + path + description in prompt
- Output schema: keep existing `Report.Groups` (no schema change)
- Exit code: non-zero on any group, `--allow-overlap` forces 0
- Filters: keep `--min-overlap-type=partial` default
- Test fixtures: `packages/cli/internal/overlap/testdata/overlap/`
- Test scope: parse + filter + exit + flag + a single agent smoke test
- "Refactor" deliverable from original P2 scope is moot — P1 plan 02 already shipped it

## Last completed

- **Phase 1 — Skill security check (REQ-4) ✓** (2026-06-10)
  - Plan 02: REFACTOR — extract agent-selection helper into
    `internal/agenttools`; rename `OverlapConfig` → `AgentSelectionConfig`
    with YAML migration
  - Plan 03: METADATA — add risk-score fields to `ManagedMetadata`
  - Plan 04: COMMAND — implement `skill check-security` command
  - Plan 05: HOOKS — re-enable gate + post-install hook
  - All `go test ./...` pass; e2e tests pass; lefthook pre-commit
    passes

## Recent decisions

- **ToolSelector** signature is `func(prompt string, labels []string, defaultOption string) (string, error)` (3 args, not 2 as plan 02 specified). Required so `selectOption` from `prompt.go` can be passed directly without an adapter.
- **`mergeManagedMetadata` heuristic for `RiskScore`**: only overwrites when `updates.RiskScore > 0` OR `updates.RiskEvaluator != ""`. Discovered regression: empty-update calls were clobbering existing risk scores. Plan 03's "always overwrite" was wrong; the heuristic is the practical fix.
- **`skills.SetDisabled` is a new helper** to update only the disabled flag without touching risk fields. Required for the re-enable gate.
- **`RunCheckSecurityForSkill` skips cost-ack prompt in hook mode** (per plan: "no prompt in hook mode"). The full `check-security` command still has the cost-ack prompt.
- **Risk evaluator field = tool.Tool.ID** (e.g. `claude-code`, `codex`); empty string = unevaluated.

## Open questions

- **Security prompt wording** — RESEARCH.md has 5 variants; Variant E (checklist-based structured scoring with `risk_factors` array) is recommended but not yet integrated. Defer to a future phase.
- **Configurable risk threshold** — currently hardcoded at 70.
- **What "evaluated" means for a skill that was disabled and re-enabled** — currently `risk-evaluator` is preserved across enable, so the gate triggers again on every re-enable.
- **What happens to risk metadata when a skill is uninstalled** — no current code path handles this; out of scope for P1.

## Constraints in play

- **AI-tool independence** — the CLI must work with any AI tool that consumes the Agent Skills spec.
- **Single-binary distribution** — no Python, no Node, no system packages.
- **Offline-first** — every command works without network, except `install` and any remote-backed security lookup.
- **No skill runner / registry / multi-user / web app for skill content** — anti-vision guards from PROJECT.md.

## Tech stack

- Go 1.24.0, Cobra v1.9.1, pterm v0.12.83, atomicgo/keyboard, kardianos/service
- pnpm monorepo (web + CLI + npm wrapper)
- lefthook pre-commit + commitlint
- release-please on `alpha`/`beta`/`main` branches

## Commands

- `pnpm test` (web, Vitest + Playwright)
- `go test ./...` (CLI)
- `git status` — should be clean after phase complete
- `git log --oneline -20` — see recent commits
