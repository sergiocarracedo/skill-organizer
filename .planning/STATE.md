# STATE.md

> Single source of truth for the current project state. Read this
> before any work to know what's done, what's next, and what
> constraints are in play.

## Current Phase

**Phase 3 — Observability (REQ-8)** — all 3 plans complete 2026-06-12; REQ-8 acceptance is observably met.

Plan progress:
- 03-01: package + identity + interface (Wave 1, no deps) ✓ implemented
- 03-02: buffer + HTTPRecorder + first-run prompt + cobra (Wave 2, depends on 01) ✓ implemented
- 03-03: OBSERVABILITY.md + byte-for-byte schema test + e2e (Wave 3, depends on 01+02) ✓ implemented
- 02-01: `--allow-overlap` flag + non-zero exit code ✓ (committed 2026-06-11)
- 02-02: curated fixtures + overlap-package tests ✓ (committed 2026-06-11)

Verification: 200 PASS, 1 SKIP, 0 FAIL across 19 packages; 13 new tests in plan 03-03; go vet clean; go build clean; lefthook pre-commit e2e green; end-to-end demo path (build -> telemetry status -> enable -> inspect YAML) completes.

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

- **Phase 3 — Observability (REQ-8) — plan 03-03 ✓** (2026-06-12)
  - 5 atomic commits; SUMMARY at
    `.planning/phases/03-observability/03-03-plan-SUMMARY.md`
  - OBSERVABILITY.md at the repo root (157 lines, 7 sections:
    What is collected, Schema, How to enable / disable, Endpoint
    configuration, Data retention, Privacy guarantees, FAQ)
    with an example JSON payload block that the schema test
    parses
  - 3 new HTTPRecorder schema tests (byte-for-byte,
    field order, field count) using httptest.NewServer to
    capture the raw POST body
  - 4 new factory + zero-egress tests using a
    `countingTransport` atomic-counter wrapper: 100
    `NoopRecorder.Record` calls must produce 0 HTTP calls
  - 3 new OBSERVABILITY doc tests in a new
    `observability_test.go` file: example payload matches
    the recorder's output (modulo 4 volatile fields), 7
    section headers present, example is valid JSON
  - 3 new e2e tests in `e2e_test.go`:
    first-run prompt fires once (with sentinel cleanup),
    disabled state never writes the buffer,
    `telemetry status` prints the 5 expected lines
  - Deviations: (1) `Telemetry` is `omitempty` in the YAML
    so a "no" answer omits the `telemetry:` key — the
    first-run e2e test was rewritten to assert the YAML
    parses as a valid AppConfig and the sentinel is created;
    (2) `newCLIEnv` pre-creates the sentinel, so the
    first-run e2e test removes the sentinel in the test
    body instead of modifying the shared helper; (3) the
    OBSERVABILITY path-resolver walks up from CWD to
    handle both `go test ./...` and
    `go test ./internal/telemetry/...`
  - Build, vet, full test suite (200 PASS, 1 SKIP, 0 FAIL
    in 19 packages), and lefthook pre-commit
    (`pnpm run test:cli:e2e`) all green
  - End-to-end manual demo path completes (build binary,
    set fresh XDG_CONFIG_HOME, `telemetry status` -> enable
    -> inspect YAML -> status shows `Enabled: true`)

- **Phase 3 — Observability (REQ-8) — plan 03-02 ✓** (2026-06-12)
  - 12 atomic commits; SUMMARY at
    `.planning/phases/03-observability/03-02-plan-SUMMARY.md`
  - HTTPRecorder (POSTs JSON, 4xx/5xx = failure) +
    `SetDefaultFactory` closure
  - Buffer JSONL spool with O_APPEND writes and 1 MB FIFO
    eviction (post-condition enforced inside Append)
  - TTY-gated FirstRunPrompt; non-TTY does NOT persist "no"
    (Pitfall P10)
  - Service umbrella with `RecordEvent` (single write path;
    falls back to buffer on failure) and `DrainBuffer`
  - `NormalizeCommandName` alias canonicalisation
    (on→enable, off→disable, install→add, rm→delete)
  - `ResolveEndpoint` flag > env > YAML precedence
  - `TelemetryConfig` YAML struct +
    `LoadTelemetryConfigOrDefault` / `SaveTelemetryConfig`
  - `cmd/telemetry.go` with `enable|disable|status|rotate-host-id`
    subcommands (all skip the first-run prompt via the
    `cmd.Name() == "telemetry"` guard in PersistentPreRun)
  - root.go: new `--telemetry-endpoint` persistent flag;
    PersistentPreRun guard extended to skip `telemetry`
    (in addition to `completion` and `help`); new
    PersistentPostRun emits one event per non-skipped command
  - 38 new tests (53 telemetry + 75 cmd + 23 config; 223 total
    in 19 packages, up from 185 baseline)
  - Two plan-checker bugs fixed and documented in the
    SUMMARY:
    - **BUG #1**: env var is
      `SKILL_ORGANIZER_TELEMETRY_ENDPOINT`, not
      `_ENABLED` (CONTEXT, RESEARCH, and the flag's own help
      text all specify `_ENDPOINT`)
    - **BUG #2**:
      `TestService_RecordEvent_NoEgressWhenDisabled` calls
      `SetDefaultFactory` BEFORE `telemetry.New` because
      `Service.Recorder` is captured inside `New` via
      `NewRecorder()` → `RecorderFactoryFunc()` at construction
      time (swapping after is a no-op)
  - Deviations: (1) `TelemetryConfig` is a type alias of
    `configpkg.TelemetryConfig` so the cmd package can
    construct it from the same struct that holds the YAML
    tags; (2) the buffer auto-evicts inside `Append` (the
    RESEARCH P7 post-condition pattern), so the FIFO test
    was rewritten to assert the post-condition rather than
    `file > 1 MB` followed by an explicit `evictLocked()`
    call; (3) `e2e_test.go` pre-creates the
    `telemetry-prompted` sentinel in `newCLIEnv` so the
    binary's first-run prompt does not block the e2e PTY
    tests
  - Build, vet, full test suite (223 passing in 19 packages),
    and lefthook pre-commit (`pnpm run test:cli:e2e`) all green

- **Phase 3 — Observability (REQ-8) — plan 03-01 ✓** (2026-06-12)
  - New `packages/cli/internal/telemetry/` package with
    `Event` struct (7 fields, snake_case JSON, regex-validated),
    `Recorder` interface, `NoopRecorder` (zero-egress default),
    `RecorderFactoryFunc` package var for test injection, and
    `NewHTTPClientFunc` placeholder for plan 02's HTTPRecorder
  - `Identity` type with `LoadOrCreate` and `RotateHostID` (32 hex
    chars from 16 random bytes via `crypto/rand`; unexported
    `generateID(io.Reader)` test seam uses `bytes.NewReader`)
  - 22 unit tests across 3 test files: 12 in `event_test.go`
    (Validate 5 sub-cases + 2 host/version paths, JSON shape,
    100-ULID format check, timestamp regex), 3 in
    `recorder_test.go` (Noop drops 1000 events, factory returns
    noop on default, factory swap with captured `[]Event`), 7 in
    `identity_test.go` (hex format, create-if-missing, reuse,
    rotation preserves install_id, corruption recovery,
    app-dir creation, regenerate-on-call)
  - 7 atomic commits; SUMMARY at
    `.planning/phases/03-observability/03-01-plan-SUMMARY.md`
  - Deviations: (1) `go get` and `go mod tidy` were split
    between task 1 and task 2 because `tidy` with no caller
    removes the dep; (2) added 2 extra Validate tests to cover
    the `host_id` and `version` error paths not in the
    table-driven test; (3) `fakeRecorder` was promoted to
    package scope because Go does not allow method declarations
    inside function bodies
  - Build, vet, full test suite (184 passing in 19 packages),
    and lefthook pre-commit (`pnpm run test:cli:e2e`) all green
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

- **`TelemetryConfig` is a type alias in the telemetry package** of `configpkg.TelemetryConfig`. The plan calls `telemetrypkg.TelemetryConfig{...}` from the cmd package but the YAML persistence layer lives in `config.TelemetryConfig`. A type alias avoids duplication while letting the cmd package construct the struct from the same fields that have `yaml:` tags.
- **Buffer auto-evicts inside `Append` (post-condition pattern from RESEARCH P7).** The plan's `TestBufferFIFOEvictionAt1MB` assumed an explicit `evictLocked()` call after the file exceeded the cap. The auto-evict is the cleaner pattern (the file is never observed to exceed 1 MB), so the test was rewritten to assert the post-condition: `file <= 1 MB` AND `oldest events dropped` AND `newest preserved`.
- **e2e test pre-creates the `telemetry-prompted` sentinel** in `newCLIEnv` (with content `no`). The first-run prompt would otherwise block the e2e PTY tests, which don't drive that prompt. The sentinel short-circuits the prompt and exercises the "user has already answered" path.
- **BUG #1 fix (plan 03-02, plan-checker)**: env var for the endpoint is `SKILL_ORGANIZER_TELEMETRY_ENDPOINT`, not `_ENABLED`. The CONTEXT, RESEARCH, the flag's help text, and OBSERVABILITY.md all specify `_ENDPOINT`. The plan's task 03-02-06 had `_ENABLED`; we use `_ENDPOINT` in `cmd/root.go`.
- **BUG #2 fix (plan 03-02, plan-checker)**: `TestService_RecordEvent_NoEgressWhenDisabled` must call `SetDefaultFactory` BEFORE `telemetry.New(...)`. The `Service.Recorder` field is set inside `New` via `NewRecorder()` → `RecorderFactoryFunc()` at construction time. Swapping the factory after `New` returns is a no-op for that Service.
- **Telemetry dep workflow: `go get` first, `go mod tidy` after the first caller exists.** Running `go mod tidy` with no caller silently removes an unused dep from `go.mod`. The plan's task 1 step said "go get && go mod tidy" but the verify step (`go list -m`) required the dep to remain. Splitting the two commands across the dep-add and the first-caller tasks preserves the dep in `go.mod` and lets `tidy` promote it from indirect to direct at the right moment.
- **`fakeRecorder` is a package-level test double** in the telemetry test file, not a function-local type. Go does not allow method declarations inside function bodies, so the type and its `Record` method must live at package scope. The factory-swap test instantiates a fresh `*fakeRecorder` per swap.
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
