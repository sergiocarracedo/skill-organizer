# Phase 4 — Telemetry backend selection (REQ-9) — Context

**Gathered:** 2026-06-12
**Mode:** standard
**Status:** Ready for planning

<domain>
## Phase Boundary

`skill-organizer telemetry status` shows a real, free-of-charge
telemetry backend that accepts the 7-field events the CLI emits.
Phase 3 shipped the emit side (recorder, buffer, opt-in prompt, no-op
when disabled) and the schema doc. Phase 4 picks the receive side:
a managed-free-tier product, the auth path, and a smoke test that
asserts the recorder's byte format against the product's ingest
contract. No new user-facing CLI commands. No changes to the
opt-in flow or the buffer.

</domain>

<decisions>
## Implementation Decisions

### Hosting model
- **Managed free tier.** The project does not stand up or operate
  a backend. The user pastes their own account credentials; we
  point at the product's public ingest URL.

### Specific product
- **New Relic (Insights Events API).** Selected for: 100 GB/month
  permanent free ingest, simple JSON event ingest, single
  `X-Insert-Key` header for auth, stable endpoint
  (`https://insights-collector.newrelic.com/v1/accounts/{account_id}/events`).
  Vendor risk accepted as a known cost of "managed free tier."

### Endpoint default & auth
- **Default `telemetry.endpoint`** is the New Relic Insights
  Events API URL with the account_id placeholder:
  `https://insights-collector.newrelic.com/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events`.
- **Auth: `X-Insert-Key` header** sourced from
  `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY` env var. (Picked over
  URL-embedded query params because New Relic explicitly
  documents the header.)
- **User setup flow** (documented in `OBSERVABILITY.md`):
  1. Sign up for New Relic (free tier).
  2. Create an Insights insert key.
  3. `export SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID=...`
  4. `export SKILL_ORGANIZER_NEWRELIC_INSERT_KEY=...`
  5. `skill-organizer telemetry enable`

### Free-tier math & roll-over
- **Volume estimate:** ~200 bytes/event × ~10 invocations/user/day
  × ~1000 active users = ~2 MB/day ≈ 60 MB/month. Comfortably
  under 100 GB/month free tier.
- **Roll-over behavior:** when the recorder gets a 413 or 429
  from New Relic, log a one-line warning and drop the event.
  No paid-upgrade flow. The local on-disk buffer (already in
  Phase 3) covers offline / restart cases, not server-quota
  cases — server-quota is a hard drop.

### Smoke test
- **`httptest.NewServer` Go test in `internal/telemetry/`.** No
  real backend contact. The test stands up a test server,
  configures a `NewRelicRecorder` to point at it, fires one
  `Record(...)`, and asserts:
  1. The POST URL is `/v1/accounts/{account_id}/events`.
  2. The `X-Insert-Key` header matches.
  3. The body is a JSON array of length 1.
  4. The first element has `eventType: "skill_organizer_command"`.
  5. The other 7 schema fields match the recorder's output
     byte-for-byte (modulo the 4 volatile fields).
- The existing schema byte-for-byte test (Phase 3 plan 03) is
  extended (not replaced) — it stays the canonical source of
  truth for the schema. The new test is the New-Relic-specific
  contract on top.

### Schema envelope
- **Backend-specific envelope: `NewRelicRecorder`.** The
  New Relic Insights Events API expects a JSON **array** of
  events, each prefixed with a string `eventType` field. Our
  flat 7-field object is wrapped:
  ```json
  [
    {
      "eventType": "skill_organizer_command",
      "command": "check-security",
      "exit_status": 0,
      "install_id": "0123456789abcdef0123456789abcdef",
      "host_id": "fedcba9876543210fedcba9876543210",
      "timestamp": "2026-06-11T12:34:56Z",
      "version": "0.4.0",
      "event_id": "01HXYZABCDEFGHJKMNPQRSTVWX"
    }
  ]
  ```
- The `RecorderFactoryFunc` picks `NewRelicRecorder` when both
  `account_id` and `X-Insert-Key` (or env equivalents) are set;
  otherwise it falls back to `HTTPRecorder` (the existing
  passthrough) or `NoopRecorder` (default).
- The `HTTPRecorder` (Phase 3) is retained as the "passthrough"
  mode for users who want to point at a custom proxy, with the
  flat object untouched.

### Agent's Discretion
- Whether to add a small retry/back-off inside `NewRelicRecorder`
  for transient 5xx (recommendation: yes, 1 retry with 250 ms
  back-off; do not queue to the buffer — the buffer is for
  network-down, not server-overloaded).
- Exact `eventType` value — "skill_organizer_command" is the
  proposed default; the planner can pick a different one if it
  fits the New Relic naming convention better.
- Whether to add a `User-Agent: skill-organizer/<version>` header
  to all recorder POSTs (recommendation: yes, for ops
  visibility).

</decisions>

<specifics>
## Specific Ideas

- The user wants the CLI to ship with New Relic as the
  out-of-the-box backend, with the existing `HTTPRecorder`
  available as a fallback for power users.
- The OBSERVABILITY.md example payload block (7-field flat
  object) stays as the canonical schema. The New Relic
  envelope is documented in a new "Backend: New Relic" section
  that wraps the schema inside an array and adds `eventType`.
- The `HTTPRecorder`'s existing byte-for-byte test continues
  to pass — its schema is the flat object. The new
  `httptest`-based test for `NewRelicRecorder` is a
  strict-superset test (array + header + `eventType` + flat
  object inside).
- No `OBSERVABILITY.md` schema bump — the project's schema
  contract is unchanged. The envelope is a backend-specific
  wrapping, not a schema change.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `OBSERVABILITY.md` — the schema contract. 7 fields, snake_case
  JSON, byte-for-byte example. Do not change.
- `packages/cli/internal/telemetry/recorder.go` — `Recorder`
  interface, `NoopRecorder`, `HTTPRecorder`, `RecorderFactoryFunc`.
  Phase 4 adds `NewRelicRecorder` here.
- `packages/cli/internal/telemetry/recorder_test.go` — existing
  byte-for-byte schema test, factory-swap test, counting-transport
  zero-egress test. Phase 4 adds a `httptest.NewServer` test for
  `NewRelicRecorder`.
- `packages/cli/internal/telemetry/event.go` — the `Event` struct
  (7 fields, `Validate` method). Phase 4 does not modify this.
- `packages/cli/internal/config/registry.go` — `ResolveEndpoint`
  flag > env > YAML precedence (Phase 3). Phase 4 extends this
  to include `SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` and
  `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY` env vars.
- `packages/cli/cmd/telemetry.go` — `enable`/`disable`/`status`/
  `rotate-host-id` subcommands (Phase 3). `status` may need to
  print the New Relic endpoint / account / key-present flag.
- `.planning/phases/03-observability/03-CONTEXT.md` and
  `03-RESEARCH.md` — Phase 3 decisions that Phase 4 builds on
  (event schema, opt-in flow, buffer semantics).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Recorder` interface (`recorder.go:25-29`) — `Record(ctx, event) error`.
  `NewRelicRecorder` implements this directly; no interface
  change needed.
- `RecorderFactoryFunc` package var (`recorder.go:38`) — swappable
  func var for test injection. Phase 4 extends the default
  closure to pick `NewRelicRecorder` when the New Relic env vars
  are set.
- `HTTPRecorder` (`recorder.go:56-90`) — passthrough backend.
  Phase 4 keeps this as the fallback when New Relic env vars are
  not set.
- `configpkg.ResolveEndpoint` (Phase 3, `registry.go`) — flag > env >
  YAML precedence. Phase 4 reuses the same pattern for
  `SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` and
  `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY`.
- `httptest.NewServer` pattern — used by
  `TestHTTPRecorderSchemaByteForByte` in Phase 3. Phase 4
  reuses the exact same pattern for the new test.

### Established Patterns
- **Func-var test injection** — every external dependency is a
  package-level func var. `NewRelicRecorder`'s test uses the
  same `t.Cleanup` swap pattern.
- **Atomic commits per task** — one task = one commit. The
  planner writes tasks in this style and the executor follows.
- **Byte-for-byte assertions** — the Phase 3 byte-for-byte
  schema test is the strongest assertion in the project. The
  Phase 4 `NewRelicRecorder` test follows the same discipline.

### Integration Points
- `recorder.go:38` (`RecorderFactoryFunc`) — extend the default
  closure to inspect the New Relic env vars and return a
  `NewRelicRecorder` if both are set, else the existing
  `HTTPRecorder` (if `Endpoint` is set), else `NoopRecorder`.
- `recorder.go` (new file section or appended) — the
  `NewRelicRecorder` struct, `NewNewRelicRecorder` constructor,
  and the `Record` method.
- `recorder_test.go` (append) — the new `httptest`-based test.
- `config.go` (`internal/config`) — add two new fields to
  `TelemetryConfig` (or a sibling struct):
  `NewRelicAccountID string` and `NewRelicInsertKey string`. With
  YAML and env-var precedence. (Or: keep them out of
  `TelemetryConfig` and read them directly from env via
  `os.Getenv` — simpler, no new YAML keys.)
- `telemetry.go` (`cmd/telemetry.go`) — extend the `status`
  subcommand to print: enabled/disabled, endpoint, recorder
  type (NoopRecorder / HTTPRecorder / NewRelicRecorder),
  account_id (truncated to first 4 chars), key-present
  (boolean), install_id (truncated), host_id (truncated),
  version.

</code_context>

<deferred>
## Deferred Ideas

- **Multi-tenant account routing** (one user, multiple
  projects, each with its own account_id) — out of scope. One
  account per user.
- **Per-tool breakdown dashboards** in New Relic — defined by
  the user after they start receiving data. Not Phase 4.
- **Migration to a paid tier** — if the free tier proves too
  small, that's a future phase with its own decision. The
  recorder's hard-drop-on-overflow (no paid-upgrade flow) is
  the v0.x contract.
- **Other managed backends** (Datadog, Sentry, BetterStack,
  Grafana Cloud) — the planner writes a single `NewRelicRecorder`
  for v0.x. Adding a `telemetry.backend` enum with pluggable
  backends is a future phase.
- **Custom proxy that ingests our flat schema and forwards to
  New Relic** — out of scope. The `HTTPRecorder` already supports
  this; the planner just documents it.

</deferred>

---

*Phase: 04-observability-product-selection*
*Context gathered: 2026-06-12*
