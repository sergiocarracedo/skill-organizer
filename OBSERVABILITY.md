# Observability (REQ-8)

> Anonymous, opt-in telemetry for `skill-organizer`. Disabled by default.
> No args, no paths, no PII. Schema and endpoint are documented below.

For the privacy and data-protection posture, see PRIVACY.md.

## What is collected

For each command invocation, we record exactly 5 fields, sent as a single
JSON object:

- `command` — the cobra subcommand name (e.g. `check-security`, `enable`).
- `exit_status` — `0` on success, `1` on error.
- `timestamp` — RFC3339 UTC, e.g. `2026-06-11T12:34:56Z`.
- `version` — CLI semver, e.g. `0.4.0`.
- `event_id` — 32 hex chars from 16 random bytes per event, for local
  buffer dedup (not linkable across events).

We do NOT collect: command arguments, file paths, environment variables,
machine fingerprints, hostnames, IP addresses, skill content, or any
other data.

The schema is fixed at 5 fields; we will not add a field without bumping
the schema version. Bumping the version is a breaking change on the
server side, so additions are intentionally rare.

## Schema

A single POST to the endpoint. Content-Type: `application/json`.

Example payload (the two volatile fields are placeholders in this doc;
the recorder's real output matches byte-for-byte modulo those two fields):

```json
{
  "command": "check-security",
  "exit_status": 0,
  "timestamp": "2026-06-11T12:34:56Z",
  "version": "0.4.0",
  "event_id": "01HXYZABCDEFGHJKMNPQRSTVWX"
}
```

The 2 volatile fields (event_id, timestamp) match:
- `event_id`: `^[0-9A-HJKMNP-TV-Z]{26}$` (Crockford base32 ULID)
- `timestamp`: `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`

The other 3 fields (`command`, `exit_status`, `version`) are deterministic
and asserted byte-for-byte in the integration test.

Field order matters: the JSON encoder emits the struct in declaration
order. A future refactor that reorders the fields will break the
byte-for-byte schema test, which is by design — schema drift is
detected at CI time, not in production.

## How to enable / disable

Telemetry is controlled by a single key: `telemetry.enabled` (bool,
default `false`). The value can be set in the YAML config file at
`~/.config/skill-organizer/skill-organizer.yml`. There is no user-
configurable endpoint — the backend URL is baked into the binary at
build time (see [Build-time backend](#build-time-backend) below).

The first run of the binary asks for consent interactively (TTY only).
The default is **no**. The answer is sticky: the prompt does not fire on
subsequent runs. To re-prompt, delete `<appDir>/telemetry-prompted`.

Subcommands:
- `skill-organizer telemetry enable` — writes `telemetry.enabled: true`
- `skill-organizer telemetry disable` — writes `telemetry.enabled: false`
- `skill-organizer telemetry status` — prints `Enabled: yes|no` and `Recorder: newrelic|noop`
- `skill-organizer telemetry wipe` — deletes the on-device buffer

The subcommands are escape hatches: they bypass the first-run prompt so
the user can disable telemetry even if they missed the prompt on the
first run.

### Build-time backend

The New Relic endpoint URL and API key are baked into the binary at
build time via `-ldflags`:

```
go build -ldflags "-X .../telemetry.NewRelicEndpoint=https://insights-collector.newrelic.com/v1/accounts/.../events -X .../telemetry.NewRelicAPIKey=..."
```

The user never configures these values. A dev build (no `-ldflags`
injection) routes all events to the `NoopRecorder` even when
`telemetry.enabled` is `true`.

The backend is the New Relic Insights Events API, served on the free
tier (100 GB / month of ingest, 1 full user, 8-day retention). The CLI
wraps the 5-field schema in a backend-specific envelope (a JSON array
of length 1 with an `eventType` prefix) and sends it to the collector
with an `X-Insert-Key` auth header. The schema is unchanged — the
envelope is a transform applied at the recorder layer, not a wire-
format bump.

Envelope (6 keys per event):

- `eventType` — always `"skill_organizer_command"`.
- `command`, `exit_status`, `version`, `event_id` — the 4 schema fields
  that survive the New Relic `timestamp` reserved-attribute rule.
- `clientTime` — the RFC3339 UTC string from the `timestamp` field,
  renamed to dodge the New Relic reserved-attribute `timestamp`.

Hard-drop on 413 / 429 (quota or rate limit): the recorder logs a
one-line warning and **drops the event**. 503 retry: one retry with a
250ms context-aware backoff. See `.planning/PHASE-4-DECISION.md` for
the ingestion math and the rationale.

## Data retention

The on-disk buffer lives at `<AppDir>/telemetry-buffer.jsonl` and is
capped at 1 MB. When the buffer exceeds 1 MB, the oldest events are
dropped (FIFO eviction). The buffer is drained opportunistically on
each run: events are sent, and on success the file is truncated; on
network failure, the unsent events are preserved for the next run.

The buffer is a JSONL file: one event per line. Each line is a complete
JSON object matching the Schema section above. The file uses `O_APPEND`
writes for atomicity against concurrent invocations, and the
opportunistic drain reads-then-truncates to avoid losing events on
partial reads.

Server-side retention is **TBD by the server owner**. The CLI does not
ship a server; the team will publish a retention policy when the
server is stood up. Until then, treat the endpoint as "your data, your
responsibility".

## Privacy guarantees

- No args, no paths, no PII. Only the 5 fields listed above.
- The 5-field schema has no linkable identifiers. The identity fields
  that were present in earlier versions (`install id`, `host id`) are
  removed. Each event carries a per-event random `event_id` that cannot
  link two events from the same machine.
- Run `skill-organizer telemetry wipe` to delete the on-device buffer.
  There are no persistent identifiers to rotate or remove.
- Disabling telemetry (`telemetry.enabled: false` or
  `telemetry disable`) stops all network egress on the telemetry path.
  The disabled state is asserted at runtime by the
  `TestNoopRecorderNoNetworkCalls` test (counting transport).

## FAQ

**Q: What happens when I'm offline?**
Events are appended to `<AppDir>/telemetry-buffer.jsonl`. The next run
with network connectivity drains the buffer.

**Q: How do I inspect the buffer?**
`ls -la ~/.config/skill-organizer/telemetry-buffer.jsonl`. The file is
JSONL, one event per line.

**Q: How do I verify zero network egress?**
Run `telemetry status` — the buffer size in bytes is reported. When
disabled, the size stays at 0 across runs.

**Q: How do I opt out?**
`skill-organizer telemetry disable` — also clears the buffer.

**Q: Where is the consent sentinel?**
`<AppDir>/telemetry-prompted`. Delete it to re-prompt on the next TTY run.

**Q: What if my config file is read-only?**
The CLI will fail to write `telemetry.enabled: true|false` and fall
back to the default (`false`). The on-disk buffer is never created in
that case — telemetry is fully opt-in, and a write failure on opt-in
is treated as opt-out. The binary still functions normally for the
non-telemetry commands.

**Q: Can I redirect events to my own backend?**
No, in v0.x the only backend is the one the binary was built with.
A future phase may add a pluggable backend.

---

*Last updated: 2026-06-12 (Phase 5).*
