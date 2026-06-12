# Phase 4 — Telemetry backend selection (REQ-9) — Discussion Log

> Audit trail of the `discuss-phase 4` conversation. Captures every
> option considered and the user's verbatim choice. NOT referenced
> by downstream agents — for human audit only.

## Phase goal (from ROADMAP.md)

> A user can run `skill-organizer telemetry status` and see a real,
> free-of-charge telemetry backend that accepts the events the CLI
> emits (7-field snake_case JSON, one POST per command).

## Carry-forward from prior phases

- Phase 3 ships the **emit side**:
  - `Recorder` interface + `NoopRecorder` + `HTTPRecorder` (passthrough)
  - `RecorderFactoryFunc` swappable package var
  - `Event` struct (7 fields, snake_case JSON, `Validate`)
  - On-disk `Buffer` (1 MB FIFO eviction)
  - TTY-gated `FirstRunPrompt`
  - `cmd/telemetry.go` (enable/disable/status/rotate-host-id)
  - `OBSERVABILITY.md` (7 sections, byte-for-byte example block)
  - Schema byte-for-byte test (`httptest`-based, was Phase 3
    plan 03) — pinned to the flat 7-field object

- The schema is **fixed** at 7 fields. Additions are a breaking
  change on the server side. Phase 4 picks the receive side
  without changing the emit-side schema.

## Scout findings

- `HTTPRecorder` is product-agnostic — POSTs JSON to any
  configured URL with no product-specific headers. The existing
  default `RecorderFactoryFunc` returns `NoopRecorder` when
  the endpoint is empty.
- `OBSERVABILITY.md` already documents the schema with a
  byte-for-byte example block that the recorder matches
  modulo the 4 volatile fields. The example does not specify
  a backend.
- `internal/telemetry/recorder.go` has the `Recorder`
  interface (`Record(ctx, event) error`), the `HTTPRecorder`
  struct, the `RecorderFactoryFunc` package var, and the
  `NewRecorder()` helper that picks `HTTPRecorder` when an
  endpoint is set.
- The existing `telemetry status` command (Phase 3) prints:
  enabled/disabled, URL, install_id, host_id. It does NOT
  print the recorder type or the account_id / key-presence
  flag. Phase 4 extends `status` to print these.

## Gray areas discussed

### Area 1: Hosting model
Options:
- **(A) Managed free tier — recommended.** New Relic 100 GB/mo,
  Grafana Cloud 10k metrics, Sentry 5k events/mo. No ops
  burden. Vendor controls retention.
- (B) Self-hosted open source. OpenObserve, SigNoz, HyperDX.
  Full retention control, more ops burden.
- (C) Both, tiered by user type. Default to managed; document
  self-hosted as a power-user option.
- (D) Project-controlled sink. A tiny Go receiver, a
  Cloudflare Worker, or a JSONL `tail -f`.

**User chose: (A) Managed free tier (Recommended).**

### Area 2: Specific product (within managed free-tier)
Options:
- **(A) New Relic — recommended.** 100 GB/mo free ingest,
  Insights Events API, single `X-Insert-Key` header.
- (B) Grafana Cloud. Well-suited to OTLP, but the JSON ingest
  path is more complex.
- (C) Highlight.io. 100% open source, custom event endpoint,
  simple POST + x-project-key header.
- (D) Sentry / BetterStack / Logtail / other.

**User chose: (A) New Relic (Recommended).**

### Area 3: Endpoint default & auth
Options:
- **(A) Default to New Relic ingest (Recommended).** Default
  `telemetry.endpoint` =
  `https://insights-collector.newrelic.com/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events`.
  `X-Insert-Key` from `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY`.
- (B) Default to no-op (no public URL). User sets endpoint
  themselves. More flexible, more setup friction.
- (C) Multi-backend pluggable. `telemetry.backend` enum:
  `newrelic` | `http`. More code, more flexibility.

**User chose: (A) Default to New Relic ingest (Recommended).**

### Area 4: Free-tier math & roll-over
Options:
- **(A) Drop on overflow — recommended.** Compute:
  ~200 bytes × 10/day × 1000 users = 60 MB/month (well under
  100 GB). On 413/429, log a warning and drop. No paid
  upgrade flow.
- (B) Buffer+log when disabled, drop when enabled + overflow.
  10 MB persistent overflow buffer; document a paid-upgrade
  path but don't implement it.
- (C) Custom quota tracker. Per-install daily ingest vs
  free-tier; emit "consider donating" log when threshold hit.

**User chose: (A) Drop on overflow (Recommended).**

### Area 5: Smoke test approach
Options:
- **(A) `httptest.NewServer` Go test (Recommended).** No real
  backend contact. Asserts POST URL, headers, body (array of
  length 1, `eventType`, byte-for-byte schema). Same
  discipline as the Phase 3 schema test.
- (B) Real backend contact. POSTs to real
  `insights-collector.newrelic.com`. Requires real account +
  insert key. Flaky.
- (C) Both. `httptest` always; opt-in real-backend test gated
  by env var. Belt-and-suspenders.

**User chose: (A) httptest.NewServer (no real backend) (Recommended).**

### Area 6: Schema envelope (New Relic expects an array)
Options:
- **(A) Backend-specific envelope — recommended.** Add
  `NewRelicRecorder` that wraps in
  `[{ "eventType": "skill_organizer_command", ... }]` and adds
  `X-Insert-Key`. The `RecorderFactoryFunc` picks
  `NewRelicRecorder` when `account_id` + `X-Insert-Key` (or
  env) are set; otherwise `HTTPRecorder` (passthrough) or
  `NoopRecorder` (default).
- (B) Generic `HTTPRecorder` POSTs the flat object. User's
  backend (e.g., a custom proxy) handles the envelope. Less
  code, less New-Relic-native, but the byte-for-byte schema
  test no longer matches what gets sent over the wire.
- (C) Two backends with a selector. `telemetry.backend` config
  key (`newrelic` | `http`).

**User chose: (A) Backend-specific envelope (Recommended).**

## Areas delegated to agent's discretion

- Whether to add a small retry/back-off inside `NewRelicRecorder`
  for transient 5xx (recommendation: yes, 1 retry with 250 ms
  back-off; do not queue to the buffer).
- Exact `eventType` value — "skill_organizer_command" is the
  proposed default; the planner can pick a different one if it
  fits the New Relic naming convention better.
- Whether to add a `User-Agent: skill-organizer/<version>` header
  to all recorder POSTs (recommendation: yes, for ops
  visibility).

## Deferred ideas

- Multi-tenant account routing (one user, multiple projects,
  each with its own account_id) — out of scope.
- Per-tool breakdown dashboards in New Relic — defined by the
  user after they start receiving data. Not Phase 4.
- Migration to a paid tier — future phase with its own
  decision.
- Other managed backends (Datadog, Sentry, BetterStack, Grafana
  Cloud) — the planner writes a single `NewRelicRecorder` for
  v0.x. Adding a `telemetry.backend` enum is a future phase.
- Custom proxy that ingests our flat schema and forwards to
  New Relic — out of scope; the `HTTPRecorder` already
  supports this.

---

*Phase: 04-observability-product-selection*
*Discussion captured: 2026-06-12*
