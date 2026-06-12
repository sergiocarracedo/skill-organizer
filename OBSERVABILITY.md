# Observability (REQ-8)

> Anonymous, opt-in telemetry for `skill-organizer`. Disabled by default.
> No args, no paths, no PII. Schema and endpoint are documented below.

## What is collected

For each command invocation, we record exactly 7 fields, sent as a single
JSON object:

- `command` — the cobra subcommand name (e.g. `check-security`, `enable`).
- `exit_status` — `0` on success, `1` on error.
- `install_id` — 32 hex chars. Stable across re-installs.
- `host_id` — 32 hex chars. Rotatable via `skill-organizer telemetry rotate-host-id`.
- `timestamp` — RFC3339 UTC, e.g. `2026-06-11T12:34:56Z`.
- `version` — CLI semver, e.g. `0.4.0`.
- `event_id` — 26-char ULID for server-side de-duplication.

We do NOT collect: command arguments, file paths, environment variables,
machine fingerprints, hostnames, IP addresses, skill content, or any
other data.

The schema is fixed at 7 fields; we will not add a field without bumping
the schema version. Bumping the version is a breaking change on the
server side, so additions are intentionally rare.

## Schema

A single POST to the endpoint. Content-Type: `application/json`.

Example payload (the four volatile fields are placeholders in this doc;
the recorder's real output matches byte-for-byte modulo those four fields):

```json
{
  "command": "check-security",
  "exit_status": 0,
  "install_id": "0123456789abcdef0123456789abcdef",
  "host_id": "fedcba9876543210fedcba9876543210",
  "timestamp": "2026-06-11T12:34:56Z",
  "version": "0.4.0",
  "event_id": "01HXYZABCDEFGHJKMNPQRSTVWX"
}
```

The 4 volatile fields (install_id, host_id, event_id, timestamp) match:
- `install_id` / `host_id`: `^[0-9a-f]{32}$`
- `event_id`: `^[0-9A-HJKMNP-TV-Z]{26}$` (Crockford base32 ULID)
- `timestamp`: `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`

The other 3 fields (`command`, `exit_status`, `version`) are deterministic
and asserted byte-for-byte in the integration test.

Field order matters: the JSON encoder emits the struct in declaration
order. A future refactor that reorders the fields will break the
byte-for-byte schema test, which is by design — schema drift is
detected at CI time, not in production.

## How to enable / disable

Three layers, in order of precedence (highest wins):

1. **CLI flag** — `--telemetry-endpoint=https://example.com/in`
2. **Env var** — `SKILL_ORGANIZER_TELEMETRY_ENDPOINT=https://example.com/in`
3. **YAML** — `telemetry: { enabled: true, endpoint: "https://example.com/in" }`
   in `~/.config/skill-organizer/skill-organizer.yml`.

The default backend is the New Relic Insights Events API — see
the [Backend: New Relic](#backend-new-relic) sub-section below
for setup. For a custom proxy, point `telemetry.endpoint` at
the proxy URL and the CLI will POST the flat 7-field object
(the HTTPRecorder passthrough mode).

The first run of the binary asks for consent interactively (TTY only).
The default is **no**. The answer is sticky: the prompt does not fire on
subsequent runs. To re-prompt, delete `<appDir>/telemetry-prompted`.

Subcommands:
- `skill-organizer telemetry enable` — writes `telemetry.enabled: true`
- `skill-organizer telemetry disable` — writes `telemetry.enabled: false` and clears the buffer
- `skill-organizer telemetry status` — prints the current state
- `skill-organizer telemetry rotate-host-id` — re-rolls the host_id

The subcommands are escape hatches: they bypass the first-run prompt so
the user can disable telemetry even if they missed the prompt on the
first run, and they can rotate `host_id` at any time without restarting
the binary.

## Endpoint configuration

See "How to enable / disable" for the three-layer precedence. The endpoint
MUST be set for events to be sent; if no endpoint is configured, the
factory returns a `NoopRecorder` that drops events with zero network
egress, regardless of the `enabled` flag.

The default endpoint is empty. The first run with a configured endpoint
prompts for consent; the prompt is skipped on non-TTY (CI, piped input)
and the default ("no") is NOT persisted, so the next TTY run re-prompts.

The endpoint is expected to accept `POST` requests with a `Content-Type:
application/json` header and a 2xx response on success. The CLI follows
3xx redirects via the standard `http.Client`; non-2xx (4xx, 5xx) is
treated as a failure and the event is appended to the on-disk buffer
for a later drain.

### Backend: New Relic

The default backend is the New Relic Insights Events API, served
on the free tier (100 GB / month of ingest, 1 full user, 8-day
retention). The CLI wraps the 7-field schema in a backend-specific
envelope (a JSON array of length 1 with an `eventType` prefix)
and sends it to the collector with an `X-Insert-Key` auth header.
The schema is unchanged — the envelope is a transform applied at
the recorder layer, not a wire-format bump.

Setup (4 steps):

1. Sign up for New Relic (free tier) at https://newrelic.com/signup.
2. Create an Insights insert key in the New Relic UI
   (Account settings → API keys → Insights insert key).
3. Export the two env vars:
   ```
   export SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID=...   # your account number
   export SKILL_ORGANIZER_NEWRELIC_INSERT_KEY=...    # the insert key
   ```
4. Enable telemetry: `skill-organizer telemetry enable`.

The CLI resolves the endpoint URL by substituting
`SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` into the default template:

```
https://insights-collector.newrelic.com/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events
```

Envelope (8 keys per event):

- `eventType` — always `"skill_organizer_command"`. New Relic uses
  this to group events in the NRDB UI.
- `command`, `exit_status`, `install_id`, `host_id`, `version`,
  `event_id` — the 6 schema fields, sent with their snake_case
  names verbatim (NRQL is case-sensitive).
- `clientTime` — the RFC3339 UTC string from the `timestamp` field.
  **Renamed** from `timestamp` to dodge the New Relic reserved-
  attribute rule: the server reserves `timestamp` for Unix-epoch
  integers and silently drops an RFC3339 string sent in that
  field. The rename is an envelope-only transform; the flat
  7-field schema above is unchanged. The HTTPRecorder
  (passthrough) still emits the field as `timestamp`.

Hard-drop on 413 / 429 (quota or rate limit): the recorder logs a
one-line warning and **drops the event**. The local on-disk buffer
is for network-down, not server-quota. If the recorder buffered
the event on 413/429, the next drain would re-POST it, the server
would return 413/429 again, and the buffer would thrash until
FIFO eviction kicks in.

503 retry: one retry with a 250ms context-aware backoff. A
cancelled context (Ctrl-C during the backoff) is honored
immediately. The 2nd 503 (after the retry) returns the error and
the event is buffered for the next drain.

**EU data center users**: replace the default URL with
`https://insights-collector.eu01.nr-data.net/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events`
by setting `telemetry.endpoint` in the YAML (or
`SKILL_ORGANIZER_TELEMETRY_ENDPOINT` in the env). The recorder
uses whichever URL it is given; the New Relic region is the
user's choice.

**Roll-over behavior**: when the free tier is exceeded, the
recorder hard-drops events. There is no paid-upgrade flow in
v0.x. The on-disk buffer covers offline / restart cases, not
server-quota cases. See `.planning/PHASE-4-DECISION.md` for the
ingestion math and the rationale.

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

- No args, no paths, no PII. Only the 7 fields above.
- `install_id` and `host_id` are 16 random bytes each, generated via
  `crypto/rand`. They are not derived from the machine, hostname,
  username, or IP address.
- `host_id` is rotatable via `telemetry rotate-host-id`. Deleting
  `<AppDir>/host_id` also regenerates the ID on the next run.
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
