# STATE.md

> Single source of truth for the current project state. Read this
> before any work to know what's done, what's next, and what
> constraints are in play.

## Current Phase

**Phase 3 — Observability (REQ-8)** — context captured, ready for planning.

Plan progress: none yet (Phase 3 has 0/0 plans).
- 02-01: `--allow-overlap` flag + non-zero exit code ✓ (committed 2026-06-11)
- 02-02: curated fixtures + overlap-package tests ✓ (committed 2026-06-11)

Verification: 14/14 must-haves passed (`9ab7ef1`).

Phase 3 discuss-phase completed 2026-06-11:
- First-run prompt: fires on first run of any command, default=no, sticky
- Event schema: JSON, 7 fields (command, exit_status, install_id, host_id, timestamp RFC3339 UTC, version, event_id ULID), snake_case
- Identity: two distinct IDs (install_id never rotates, host_id rotatable via `telemetry rotate-host-id`); 32 hex chars from 16 random bytes via crypto/rand
- Endpoint: no-op default + YAML/env/flag (precedence: flag > env > YAML); `Recorder` interface with `NoopRecorder` (drops) and `HTTPRecorder` (POSTs JSON); factory func var for test injection
- Network gating: zero egress when disabled; **buffer on disk** (`<AppDir>/telemetry-buffer.jsonl`, 1 MB cap FIFO eviction) for retry on offline
- OBSERVABILITY.md: full 7-section doc at repo root
- Test strategy: Recorder interface + httptest server (FakeRecorder + counting transport for zero-network; httptest.NewServer for schema byte-for-byte)

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

- **Phase 2 — Overlap refactor (REQ-3) — plan 02-02 ✓** (2026-06-11)
  - Added 7 curated `SKILL.md` fixtures under
    `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/`
    (2 + 2 + 3)
  - Added `loadFixtureRoot` + `copyDir` helpers in
    `packages/cli/internal/overlap/overlap_test.go`
  - Added 4 new tests in the `overlap` package:
    `TestCollectSkillsOnConflictingFixture`,
    `TestCollectSkillsOnCleanFixture`,
    `TestCollectSkillsOnPartialFixture`,
    `TestRunParsesReportWithMixedSeverities`
  - All `go test ./...` and `go build ./...` pass; lefthook
    pre-commit hook passes
  - Deviation noted in SUMMARY.md: the new tests use leaf-name
    `RelativePath` keys (`"alpha"`, `"beta"`, `"gamma"`) instead
    of the scenario-prefixed paths the plan action text
    referenced, because `loadFixtureRoot` only copies the inner
    entries of `testdata/overlap/<scenario>/` into `t.TempDir()`.
- **Phase 2 — Overlap refactor (REQ-3) — plan 02-01 ✓** (2026-06-11)
  - Added `--allow-overlap` cobra flag (default false) and
    `overlapAllowOverlap` package var in `packages/cli/cmd/skill_overlap.go`
  - Inserted exit-code check after `printOverlapReport` and before
    the `--no-ask-to-apply` early-return; returns
    `fmt.Errorf("overlap detected: %d group(s) (use --allow-overlap to ignore)", ...)`
  - Added 3 new tests; 65 cmd tests pass; e2e tests pass
  - Build, vet, help text, all green
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
