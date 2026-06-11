# Phase 3 — Observability (REQ-8) — Discussion Log

> Audit trail of the `discuss-phase 3` conversation. Captures every
> option considered and the user's verbatim choice. NOT referenced
> by downstream agents — for human audit only.

## Phase goal (from ROADMAP.md)

> Opt-in, anonymous telemetry that records command invocations
> without args / paths / PII. Disabled by default. Documented
> schema and endpoint.

## Carry-forward from prior CONTEXT files

- P1: the `agenttools` package's swappable func-var pattern is
  the established test-injection idiom in this codebase.
- P2: the user prefers minimal changes that exactly match the
  spec; "refactor" deliverables get re-classified when the work
  is already done in a prior phase. The same disposition is
  expected for P3.

## Scout findings

- No telemetry/observability code exists in the codebase.
  Grep for `telemetry|metric|observ` matches only the unrelated
  `selfupdate` package.
- The natural home for a new `TelemetryConfig` sub-struct is
  `packages/cli/internal/config/config.go`, alongside
  `AgentSelectionConfig` and `BackupConfig`.
- `configpkg.AppDir()` provides the user config dir for the
  install-id and host-id files.
- The `agenttools.ChooseAgentToolFunc` pattern in
  `packages/cli/internal/agenttools/agenttools.go` is the model
  for the swappable `RecorderFactoryFunc`.

## Gray areas discussed

### Area 1: First-run prompt placement
Options:
- **(A) On first run of any command — recommended.** Friendly,
  catches every user.
- (B) On root command only.
- (C) Explicit `telemetry` subcommand only — user must opt in
  themselves.
- (D) First run of `status` command.

**User chose: (A) On first run of any command (Recommended).**

### Area 2: Event schema
Options:
- **(A) JSON, 7 fields, snake_case keys — recommended.**
  Fields: `command`, `exit_status`, `install_id`, `host_id`,
  `timestamp` (RFC3339 UTC), `version`, `event_id` (ULID for
  de-dup).
- (B) Minimal 4 fields (no host_id, no version, no event_id).
- (C) JSON, 5 fields (no event_id).

**User chose: (A) JSON, 7 fields, snake_case keys (Recommended).**

### Area 3: Identity model
Options:
- **(A) Two distinct IDs (install_id, host_id) — recommended.**
  Both 32 hex chars from 16 random bytes via `crypto/rand`.
  install_id never rotates; host_id rotatable via
  `telemetry rotate-host-id` subcommand.
- (B) One ID (host_id only).
- (C) UUID v4 instead of random hex.

**User chose: (A) Two distinct IDs (Recommended).**

### Area 4: Endpoint model
Options:
- **(A) No-op default + YAML/env/flag — recommended.**
  `Recorder` interface with `NoopRecorder` (drops) and
  `HTTPRecorder` (POSTs JSON). Factory returns no-op by
  default; HTTP when telemetry is enabled AND a URL is
  configured. Precedence: flag > env > YAML > no-op.
- (B) Hardcoded local sink.
- (C) Default to a real endpoint (server we don't have).

**User chose: (A) No-op default + YAML/env/flag (Recommended).**

### Area 5: Network gating & offline buffering
Options:
- (A) Drop on failure (no buffering) — recommended.
- **(B) Buffer on disk for later retry.**
- (C) Best-effort with timeout (block up to 5s).

**User chose: (B) Buffer on disk for later retry.** (Override of
the recommended option.) Buffer is a JSONL file at
`<AppDir>/telemetry-buffer.jsonl` with a 1 MB cap (FIFO eviction).
Drain is opportunistic — only runs when the user invokes the
binary.

### Area 6: OBSERVABILITY.md doc shape
Options:
- **(A) Full 7-section doc — recommended.**
  1. What is collected
  2. Schema (with example JSON)
  3. How to enable / disable
  4. Endpoint configuration
  5. Data retention
  6. Privacy guarantees
  7. FAQ
- (B) Minimal 4-section doc.
- (C) Schema only.

**User chose: (A) Full 7-section doc (Recommended).**

### Area 7: Test strategy
Options:
- **(A) Recorder interface + httptest server — recommended.**
  Two-part: zero-network via counting transport +
  FakeRecorder-empty-slice; schema via FakeRecorder field
  check + httptest.NewServer request-body assertion.
- (B) Only FakeRecorder (no httptest).
- (C) Only httptest server.

**User chose: (A) Recorder interface + httptest server
(Recommended).**

## Areas delegated to agent's discretion

- Exact wording of the first-run prompt and the
  `OBSERVABILITY.md` copy.
- Choice of ULID library (the `oklog/ulid` package is the
  standard; alternatives exist).
- Buffer file naming and location (within `<AppDir>`).

## Deferred ideas

- **Server-side retention policy.** The server isn't built
  yet; the OBSERVABILITY.md retention section is a
  placeholder.
- **Heavier analytics** (per-flag, per-subcommand, per-error
  category) — not in REQ-8.
- **Opt-in retry backoff / jitter.** The buffer is drained
  opportunistically. If a daemon mode ships later, the
  telemetry layer can hook into it.
- **A "dry-run" / "preview the next event" subcommand** —
  useful for power users and security review, but not in
  REQ-8 scope.

---

*Phase: 03-observability*
*Discussion captured: 2026-06-11*
