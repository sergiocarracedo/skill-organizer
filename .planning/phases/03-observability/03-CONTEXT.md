# Phase 3 — Observability (REQ-8) — Context

**Gathered:** 2026-06-11
**Mode:** standard
**Status:** Ready for planning

<domain>
## Phase Boundary

`skill-organizer` ships an opt-in, anonymous telemetry layer so the
team can measure adoption (the project's top risk per PROJECT.md
is "low ROI / no users"). Disabled by default. When disabled,
**zero** network egress is emitted by the telemetry path. When
enabled, the layer records command-invocation events (no args,
no paths, no PII), buffers them on disk, and POSTs to a
configurable endpoint. The schema, opt-in/opt-out procedure, and
data-retention policy are documented in a new `OBSERVABILITY.md`
at the repo root.

The success metric in PROJECT.md stays "to be defined" until this
phase has shipped and produced data.

</domain>

<decisions>
## Implementation Decisions

### First-run prompt placement
- **Fire on the first run of any command.** If the user has
  never answered the prompt, the very first invocation of the
  binary (any subcommand, or the bare `skill-organizer`) shows
  the opt-in prompt before the command runs.
- **Default = off.** The prompt asks "Enable anonymous
  telemetry?" with the default answer set to **no**. The user
  can press Enter to decline, or arrow-key to yes.
- **Once answered, never ask again.** Whether the user picks yes
  or no, the answer is sticky in `AppConfig.Telemetry.Enabled`
  (YAML key `telemetry.enabled`) and the prompt is suppressed
  on every subsequent run. The user can re-enable or disable at
  any time by editing the YAML, by setting the
  `SKILL_ORGANIZER_TELEMETRY_ENABLED` env var, or by running
  the `telemetry` subcommand.
- **Skippability:** declining counts as a sticky "no". The
  prompt cannot be "skipped without answering"; it must be
  either yes or no.

### Event schema
- **JSON body**, snake_case keys (matches the existing repo
  style: `auto_trigger`, `agent_selection`, `risk_score`).
- **Fields** (7):
  - `command` (string): the cobra subcommand name, e.g.
    `check-security`, `enable`, `add`, `check-overlap`.
  - `exit_status` (int): 0 = success, 1 = error. Mirrors the
    cobra command's exit semantics.
  - `install_id` (string): 32 hex chars from 16 random bytes
    (see Identity model).
  - `host_id` (string): 32 hex chars from 16 random bytes.
  - `timestamp` (string): RFC3339 UTC, e.g.
    `2026-06-11T12:34:56Z`.
  - `version` (string): CLI semver, e.g. `0.4.0`.
  - `event_id` (string): ULID for de-dup on the server side.

### Identity model
- **Two distinct IDs**: `install_id` and `host_id`. Each is
  32 hex chars from 16 bytes read via `crypto/rand`. Both are
  generated **on first run** and stored in
  `<AppDir>/install_id` and `<AppDir>/host_id` respectively.
- **install_id never rotates.** It is the binary's stable
  identity across runs on the same machine.
- **host_id is rotatable** via a new `telemetry rotate-host-id`
  subcommand. The user can re-roll the dice if they want a
  fresh identity.
- **No PII.** Both IDs are random; no machine fingerprint, no
  hostname, no username, no IP.

### Endpoint model
- **No-op default.** The default `Recorder` returned by the
  factory is a `NoopRecorder` that drops events. Zero network
  calls are made by the no-op path.
- **HTTP `Recorder`** is wired when the user enables telemetry
  AND configures an endpoint URL. It POSTs the JSON event
  body to the configured URL with `Content-Type:
  application/json`.
- **Configurable via three layers** (in order of precedence,
  highest wins): `--telemetry-endpoint` flag,
  `SKILL_ORGANIZER_TELEMETRY_ENDPOINT` env var,
  `telemetry.endpoint` in YAML. If none is set, the factory
  returns `NoopRecorder` regardless of the `enabled` flag.
- **Factory pattern.** A `Recorder` interface with two
  implementations: `NoopRecorder` (drops) and `HTTPRecorder`
  (POSTs). A package-level factory function returns the right
  one based on config. Test code can override the factory via
  a swappable func var (same pattern as
  `agenttools.ChooseAgentToolFunc`).

### Network gating & offline buffering
- **Zero network egress when disabled.** The no-op path is
  reachable without going through any HTTP client. The test
  suite asserts this by wrapping `http.DefaultTransport` with
  a counting transport and failing the test if any request is
  made while telemetry is disabled.
- **Buffer on disk for later retry** when telemetry is enabled
  but the network call fails. Buffer is a JSONL file at
  `<AppDir>/telemetry-buffer.jsonl`. On each enabled run, the
  recorder:
  1. Drains the buffer (best-effort, drops on failure).
  2. Sends the new event.
  3. On HTTP failure (offline, timeout, DNS error), appends
     the new event to the buffer.
- **Buffer cap: 1 MB.** When the buffer would exceed 1 MB, the
  oldest events are dropped (FIFO). Prevents unbounded growth
  on long-offline machines.
- **No retry timers, no daemon.** The drain is opportunistic —
  it only runs when the user invokes the binary.

### `OBSERVABILITY.md` doc shape
- **Full 7-section doc** at the repo root:
  1. **What is collected** — the 7 fields, in plain English.
  2. **Schema** — the JSON shape with field types and an
     example payload.
  3. **How to enable / disable** — the three-layer config
     precedence, the first-run prompt, and the
     `telemetry enable|disable` subcommand.
  4. **Endpoint configuration** — flag, env var, YAML.
  5. **Data retention** — "TBD by user" placeholder; the
     current phase ships the buffer cap (1 MB) but the
     server-side retention is a placeholder for the
     server's owner.
  6. **Privacy guarantees** — no args, no paths, no PII; only
     the 7 fields above; identity is random; the user can
     rotate `host_id` or disable at any time.
  7. **FAQ** — what happens offline, how to inspect the
     buffer, how to verify zero network egress (the
     `telemetry status` subcommand prints the current state).

### Test strategy
- **Two-part** (per the recommended option):
  1. **Zero-network assertion.** A test wraps
     `http.DefaultTransport` with a counting transport and
     fails the test if any request is made. Combined with
     a `FakeRecorder` that records events to a slice; the
     slice is asserted empty when telemetry is disabled.
  2. **Schema byte-for-byte.** A `FakeRecorder` captures the
     JSON-serialized event; the test asserts field-by-field
     equality. An integration test uses
     `net/http/httptest.NewServer` as the endpoint and
     asserts the request body matches the schema byte-for-byte.

### Agent's Discretion
- The exact wording of the first-run prompt and the
  `OBSERVABILITY.md` copy.
- The choice of ULID library (the `oklog/ulid` package is the
  standard; alternatives exist).
- Buffer file naming and location (within `<AppDir>`).

</decisions>

<specifics>
## Specific Ideas

- The user's pattern across P1 and P2: prefer minimal changes
  that exactly match spec. Reuse extracted code (the
  `agenttools` factory pattern for the swappable func var).
- The "Refactor" deliverable in P2 was a no-op because the work
  was already done in P1 plan 02. The user accepted the
  re-classification gracefully. Expect the same disposition for
  P3: do exactly what REQ-8 says, no more, no less.
- The user picked **buffer + retry** for offline behavior, not
  the simpler "drop on failure" default. This is a non-trivial
  addition: it requires an on-disk JSONL file, a drain step at
  the start of each run, and a cap to prevent unbounded growth.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `.planning/PROJECT.md` — the "Top risk" and "Success metric"
  sections. The success metric stays "to be defined" until
  this phase ships.
- `.planning/REQUIREMENTS.md` REQ-8 — the acceptance criteria
  (first-run prompt skippable, opt-in, default off, schema
  documented, zero network egress when disabled).
- `.planning/phases/02-overlap-refactor/02-CONTEXT.md` — the
  `single_layer_justified: true` precedent for plans that are
  test-infrastructure-only. Likely applicable to a P3 plan
  whose deliverable is OBSERVABILITY.md + tests.
- `packages/cli/internal/config/config.go` — where the new
  `TelemetryConfig` struct will live (alongside
  `AgentSelectionConfig`, `BackupConfig`, etc.).
- `packages/cli/internal/config/registry.go` — the on-disk
  config dir is `<AppDir>` (via `configpkg.AppDir()`). New
  files: `<AppDir>/install_id`, `<AppDir>/host_id`,
  `<AppDir>/telemetry-buffer.jsonl`.
- `packages/cli/internal/agenttools/agenttools.go` — the
  func-var test-injection pattern. Reuse for
  `RecorderFactoryFunc`.
- `packages/cli/cmd/root.go` — the entry point where the
  first-run prompt and the per-command event emit live.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `configpkg.AppConfig` and the `AgentSelectionConfig` /
  `BackupConfig` precedent — a new `TelemetryConfig` struct
  lives next to them in
  `packages/cli/internal/config/config.go`.
- `configpkg.AppDir()` returns the on-disk config directory;
  use it for `<AppDir>/install_id`, `<AppDir>/host_id`,
  `<AppDir>/telemetry-buffer.jsonl`.
- `agenttools.ChooseAgentToolFunc` (and the other
  `*Func` vars in `internal/agenttools/agenttools.go`) — the
  swappable func-var pattern for test injection. Reuse for
  `RecorderFactoryFunc` in the new `internal/telemetry`
  package.
- `configpkg.AgentSelectionConfig` YAML migration pattern
  (read `overlap.*` keys if `agent-selection.*` is missing) —
  analogous if we ever change the telemetry config key name.

### Established Patterns
- **Func-var test injection:** every external dependency in
  the cmd package is a package-level func var. The new
  `RecorderFactoryFunc` follows the same pattern.
- **YAML key migration:** when renaming a config key, the
  reader falls back to the old key for backward
  compatibility. Apply this to any future telemetry-config
  rename.
- **First-class `AppConfig` sub-structs:** every config
  concern lives as a sub-struct of `AppConfig` (e.g.
  `AgentSelection`, `Backup`). `TelemetryConfig` is the
  next one.

### Integration Points
- `cmd/root.go` — the cobra root command. Persistent
  pre-run hook is the natural place for the first-run
  prompt and the event recorder. The recorder emits
  per-command events at the end of `RunE`.
- `configpkg.RegistryPath()` / `configpkg.AppDir()` — the
  on-disk user config dir. The new `install_id` and
  `host_id` files live here.
- `main.go` (or the `kardianos/service` daemon code, if
  relevant) — the entry point that initializes the
  recorder at startup. Untouched for P3.

</code_context>

<deferred>
## Deferred Ideas

- **Server-side retention policy.** The server that receives
  events isn't built yet; the OBSERVABILITY.md section on
  data retention is a placeholder. A future phase (or a
  separate "telemetry server" repo) will fill this in.
- **Heavier analytics** (per-flag, per-subcommand, per-error
  category) — not in REQ-8. The 7-field schema is the
  minimum viable surface.
- **Opt-in retry backoff / jitter** — the buffer is drained
  opportunistically on each run. No exponential backoff
  because there's no daemon to schedule the retries. If a
  daemon mode ships in a later phase, the telemetry layer
  can hook into it.
- **A "dry-run" / "preview the next event" subcommand** —
  useful for power users and security review, but not in
  REQ-8 scope.

</deferred>

---

*Phase: 03-observability*
*Context gathered: 2026-06-11*


