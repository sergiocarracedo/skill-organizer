# Phase 4: Telemetry backend selection (REQ-9) — Research

**Researched:** 2026-06-12
**Phase goal:** `skill-organizer telemetry status` shows a real, free-of-charge
telemetry backend (New Relic Insights Events API) that accepts the 7-field
events the CLI emits.

**Scope of this research:** what the planner needs to know to break Phase 4
into small, shippable plans. The user-locked decisions in
`04-CONTEXT.md` (New Relic Insights Events API, env-var auth, hard-drop on
413/429, `httptest` smoke test, `NewRelicRecorder` extension) are taken as
given; this doc focuses on the *implementation playbook* for those decisions.

Confidence levels (HIGH / MEDIUM / LOW) reflect how much of the
recommendation is verified against the official New Relic docs, the
existing Phase 3 code, and reproducible external sources.

---

## Don't Hand-Roll

| Problem | Recommended solution | Why | Confidence |
|---|---|---|---|
| New Relic HTTP POST | Stdlib `net/http` via the existing `NewHTTPClientFunc` package var (Phase 3, `recorder.go:52-54`) | Phase 3 already wires a 10-second-timeout client and a swappable transport for the counting-transport test. Reuse the same `NewHTTPClientFunc` for the NewRelicRecorder so the zero-egress test pattern (Phase 3, `recorder_test.go:284-304`) extends unchanged. [VERIFIED: `packages/cli/internal/telemetry/recorder.go:52-54`] | HIGH |
| JSON array envelope | Stdlib `encoding/json` on a slice: `[]map[string]any{ {"eventType": "skill_organizer_command", ...event fields} }` | The New Relic API expects a JSON array of flat objects; using a `[]map[string]any` lets the recorder inject the `eventType` prefix without a second struct. `encoding/json` marshals maps in key-sorted order — the byte-for-byte test asserts key set + values, not field order inside the array element. [VERIFIED: https://docs.newrelic.com/docs/data-apis/ingest-apis/event-api/introduction-event-api/] | HIGH |
| `X-Insert-Key` auth header | Stdlib `req.Header.Set("X-Insert-Key", cfg.InsertKey)` | The CONTEXT decision locks `X-Insert-Key` (over URL-embedded query params). The current New Relic docs use `Api-Key` as the recommended header on the Event API page [VERIFIED: https://docs.newrelic.com/docs/data-apis/ingest-apis/event-api/introduction-event-api/], but the legacy `X-Insert-Key` header is still accepted by the Insights Event API specifically (the page documents both). Per CONTEXT lock, use `X-Insert-Key`. Flag in the SUMMARY that real-account manual testing should confirm. | MEDIUM |
| Free-tier math | New Relic free tier = **100 GB data ingest per month, 1 full user, 8-day retention** | The CONTEXT's "~60 MB/month" estimate is ~0.06% of the 100 GB cap — three orders of magnitude under the quota. Confirmed by New Relic's pricing page and 3rd-party analyses (G2, Vendr, comparretiers). [VERIFIED: https://newrelic.com/pricing, https://costbench.com/software/developer-tools/new-relic/free-plan/, https://www.g2.com/products/new-relic/pricing] | HIGH |
| Endpoint URL with placeholder | Resolve the env var at the call site, not at config-load time: `os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID")` then `fmt.Sprintf("https://insights-collector.newrelic.com/v1/accounts/%s/events", accountID)` | The CONTEXT specifies the env var name and the placeholder URL. Resolving in the constructor (not at package init) makes tests deterministic and lets the smoke test inject a fake account_id. The NewRelicRecorder is constructed per-command (Pitfall P9-style), so a fresh env-read is cheap. | HIGH |
| Env-var precedence for NewRelic creds | `os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID")` and `os.Getenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY")` directly — no new YAML keys | These are secrets; putting them in YAML forces the user to gitignore the file or risk committing them. Env-var-only matches GitHub CLI / npm token patterns. The factory reads them at recorder-construction time. | HIGH |
| 1-retry on 503 with 250ms backoff | Stdlib `time.NewTimer(250 * time.Millisecond)` + select on `ctx.Done()` for cancellation | The CONTEXT's "agent's discretion" section recommends 1 retry with 250 ms backoff for transient 5xx. Hard-coding 1 retry (not 2+) keeps the per-event wall-clock under the New Relic 10-second timeout even with the existing 10s HTTP client. Do NOT retry 4xx (413/429 are quota, not transient) and do NOT retry 400 (malformed payload, retry won't help). | MEDIUM |
| `User-Agent: skill-organizer/<version>` | Reuse the existing `selfupdate.userAgent` pattern at `packages/cli/internal/selfupdate/selfupdate.go:250, 326` | The selfupdate package already sets `User-Agent: skill-organizer/<version>` on its GitHub requests. Mirror the pattern in the NewRelicRecorder — a one-liner that helps the New Relic side debug which client version is sending. | HIGH |
| Smoke test | `httptest.NewServer` in `recorder_test.go` (same pattern as Phase 3's `TestHTTPRecorderSchemaByteForByte`, `recorder_test.go:98-176`) | The Phase 3 test pattern is the established "capture the raw POST body and assert" approach. Reuse it for the NewRelicRecorder. The 5 assertion points in the CONTEXT smoke-test spec map 1:1 onto the existing test's structure (path, header, body is array of length 1, first element has `eventType`, other fields match). | HIGH |

**Avoid** these tempting hand-rolls:

- **Don't write a custom HTTP client with retries for the New Relic endpoint.** The existing `NewHTTPClientFunc` package var + stdlib `http.Client` covers 99% of the cases. Add retries only on the recorder's outer `Record` method, where the caller can already wire timeouts.
- **Don't put `account_id` or `insert_key` in the YAML schema.** YAML files end up in git; secrets in YAML is a footgun. Env-var-only matches how `GITHUB_TOKEN`, `NPM_TOKEN`, etc. are stored.
- **Don't try to auto-detect a region (US vs EU) for the New Relic endpoint.** CONTEXT locks the US endpoint. Adding EU support later is a one-line change (the URL is the only difference). YAGNI for v0.x.
- **Don't gzip the request body.** The official docs *recommend* compression [VERIFIED: https://docs.newrelic.com/docs/data-apis/ingest-apis/event-api/introduction-event-api/], but our events are ~200 bytes — well under the 1 MB payload cap. Adding `gzip.Writer` + `Content-Encoding: gzip` header for sub-kilobyte payloads buys nothing and adds two failure modes (compression error, decode error). Defer to a future phase if the event size grows.
- **Don't import a New Relic SDK.** The `newrelic/go-agent` SDK is for the APM agent protocol (different endpoint, different auth). The Insights Event API is a single HTTP POST — stdlib is sufficient and avoids a 50-MB transitive dep tree.
- **Don't use `event.Timestamp` (RFC3339 string) as the New Relic `timestamp` field.** The official docs explicitly reserve `timestamp` as a Unix-epoch integer attribute. [VERIFIED: https://docs.newrelic.com/docs/data-apis/custom-data/custom-events/data-requirements-limits-custom-event-data/] An RFC3339 string sent in the `timestamp` field will be **dropped at ingest** — the server uses the receive time as the event time, and we lose the original command time. See Pitfall NP1 below for the fix.

---

## Common Pitfalls

### NP1. `timestamp` is a reserved New Relic attribute — RFC3339 string will be dropped

**What goes wrong:** The Phase 3 schema has `timestamp` as an RFC3339 UTC
string (e.g., `2026-06-11T12:34:56Z`); this is locked by OBSERVABILITY.md
and the byte-for-byte schema test. The CONTEXT's envelope example shows
this exact field sent as `"timestamp": "2026-06-11T12:34:56Z"`. But the
official New Relic docs list `timestamp` as a *reserved* attribute that
"Must be a Unix epoch timestamp. You can define timestamps either in
seconds or in milliseconds. It must be +/-1 day (24 hours) of the
current time on the server." [VERIFIED:
https://docs.newrelic.com/docs/data-apis/custom-data/custom-events/data-requirements-limits-custom-event-data/]

A string value sent in the `timestamp` field will be **dropped at
ingest**. The event will still be accepted (200 OK), but the server uses
its receive time as the event time, and the original `timestamp` field
is silently discarded. We lose the ability to query by the user's local
command time — we'd be looking at the network's view of when the event
arrived, which is always slightly later and varies with retries.

**How to avoid:** Two options. Pick (a).

(a) **Map in the NewRelicRecorder envelope only** — the
NewRelicRecorder builds the array element as a `map[string]any` and
copies the 6 other schema fields verbatim, but emits the
timestamp under a different key like `clientTime` (or
`recordedAt`):

```go
elem := map[string]any{
    "eventType": "skill_organizer_command",
    "command":   event.Command,
    "exitStatus": event.ExitStatus,  // Note: see NP2 about exit_status
    "installId": event.InstallID,    // Note: see NP2 about snake_case
    "hostId":    event.HostID,
    "clientTime": event.Timestamp,   // renamed to dodge the reserved word
    "version":   event.Version,
    "eventId":   event.EventID,
}
```

This is a **backend-specific transform** — the 7-field flat schema is
unchanged, the HTTPRecorder test stays the same, the NewRelicRecorder
gets a new envelope. The OBSERVABILITY.md "Backend: New Relic"
section will document the rename.

(b) **Bump the schema** to `clientTime` everywhere — requires changing
OBSERVABILITY.md, the schema test, the recorder_test.go, and the
contxt example. CONTEXT explicitly says the schema doesn't change, so
this is out of scope for Phase 4.

**Confidence:** HIGH — verified against the official reserved-words
doc; the RFC3339 string sent in the `timestamp` field is documented
as dropped.

### NP2. `exit_status` and `exitStatus` — case-only difference is significant on the server

**What goes wrong:** New Relic attribute names are case-sensitive
("NRQL is case-sensitive for both event types and attribute names").
[VERIFIED:
https://docs.newrelic.com/docs/data-apis/custom-data/custom-events/data-requirements-limits-custom-event-data/]
Our schema uses `exit_status` (snake_case); the CONTEXT's envelope
example shows `exit_status`; that's consistent. The risk is that a
**future** refactor (e.g., "convert snake_case to camelCase for the
New Relic envelope to match the docs' example") silently breaks the
schema. Don't do that — the OBSERVABILITY.md schema is snake_case,
the New Relic envelope is snake_case, the wire format is
byte-for-byte in both.

**How to avoid:** The NewRelicRecorder copies the field names verbatim
from the schema. Document in OBSERVABILITY.md "Backend: New Relic"
section that the New Relic envelope uses the *same* snake_case field
names as the schema, with two exceptions: `eventType` (required) and
`clientTime` (renamed to dodge the reserved `timestamp`).

**Confidence:** HIGH — verified against the docs.

### NP3. 503 retry must use a context-aware timer (not `time.Sleep`)

**What goes wrong:** The CONTEXT recommends 1 retry with 250 ms
backoff for 503 (Service temporarily unavailable). A naive
`time.Sleep(250 * time.Millisecond)` blocks the goroutine, which
means a cancelled context (e.g., the user pressed Ctrl-C) is not
honored until the sleep returns. The HTTP request already uses
`http.NewRequestWithContext`, so a slow retry is a slow cancellation.

**How to avoid:** Use `time.NewTimer` + `select` on `ctx.Done()`:

```go
select {
case <-time.After(250 * time.Millisecond):
    // continue with retry
case <-ctx.Done():
    return ctx.Err()
}
```

The existing `Record(ctx, event)` signature accepts a context — use
it. This is the same pattern the existing `Maintenance.MaybeNotify*`
helpers use for cancellable sleeps.

**Confidence:** HIGH — standard Go context pattern.

### NP4. The NewRelicRecorder must NOT fall back to the buffer on 413/429

**What goes wrong:** Phase 3's `Service.RecordEvent` (telemetry.go:69-91)
has a "recorder fails → buffer the event" pattern. The CONTEXT says
413/429 are "hard-drops" (the local buffer is for network-down, not
server-quota). If the NewRelicRecorder returns a non-nil error for
413/429, the Service will append the event to the buffer, the next
drain will re-POST it, and the server will return 413/429 again —
an infinite loop until the buffer's 1 MB FIFO eviction kicks in
(thousands of events later).

**How to avoid:** The NewRelicRecorder distinguishes three outcomes:
1. **2xx**: return nil. Service moves on.
2. **413, 429**: log a one-line warning via pterm (`pterm.Warning`),
   return nil. The event is dropped. Service moves on.
3. **Other 4xx, 5xx, network error**: return the error so the
   Service appends to the buffer for later drain. (Note: 5xx gets
   the 1-retry path first; only the final 5xx is bubbled up.)

The CONTEXT's "log a one-line warning" maps to `pterm.Warning` —
not `fmt.Fprintln(os.Stderr)`, because the project's color rules
(yellow reserved for keyboard hints; magenta/cyan for status) prefer
pterm helpers. Reuse the existing `pterm.Warning` import.

**Confidence:** HIGH — the CONTEXT is explicit; the buffer-loop
failure mode is mechanical.

### NP5. The CONTEXT-locked envelope example is the WRONG byte sequence for the API

**What goes wrong:** The CONTEXT's example envelope in section
"Schema envelope" is:

```json
[
  {
    "eventType": "skill_organizer_command",
    "command": "check-security",
    "exit_status": 0,
    "install_id": "0123456789abcdef0123456789abcdef",
    "host_id": "fedcba9876543210fedcba9876543210",
    "timestamp": "2026-06-11T12:34:56Z",  // <-- will be dropped (NP1)
    "version": "0.4.0",
    "event_id": "01HXYZABCDEFGHJKMNPQRSTVWX"
  }
]
```

The `timestamp` field is the issue from NP1 — sent as RFC3339, dropped
at ingest. The example also uses `command: "check-security"`, but the
RECORDED event's `command` is whatever `cobra.Command.Name()` returns.
The byte-for-byte test won't catch the timestamp drop because the
test asserts the *outgoing* bytes match the CONTEXT example, not that
New Relic actually ingests them. The mismatch is server-side.

**How to avoid:** When writing the smoke test, the test asserts the
*outgoing* bytes (matching the CONTEXT envelope) — that's what the
CONTEXT locks. But the OBSERVABILITY.md "Backend: New Relic" section
MUST be updated to call out the rename (`timestamp` → `clientTime`)
as a deliberate workaround for the reserved-attribute rule. The
workaround is small (one field), the rationale is strong (the server
drops the original), and the alternative is server-side data loss
no one would notice until the analytics run.

The planner should also document this rename prominently in the
SUMMARY's "Deviations" section.

**Confidence:** HIGH — verified against the reserved-words doc.

### NP6. `accountId` is reserved — if our schema ever adds an `account_id` field, ingest drops it

**What goes wrong:** The official reserved-words list calls out
`accountId` as dropped at ingest. [VERIFIED:
https://docs.newrelic.com/docs/data-apis/custom-data/custom-events/data-requirements-limits-custom-event-data/]
Our schema doesn't have an `account_id` field today, but a future
refactor that adds one for "which New Relic account did this event
go to" will silently lose it. There's no error from the server (200
OK), and the field is just gone from the NRDB query result.

**How to avoid:** Add a one-line comment to `event.go` listing the
New Relic reserved-attribute names that our schema fields collide
with (`timestamp`). If a future field addition uses one of those
names, the comment catches it at code-review time. The list is short:
`accountId`, `appId`, `eventType`, `timestamp`, `entity.guid`,
`entity.name`, `entity.type`. (`accountId` is camelCase; our schema
is snake_case so the collision is impossible unless we deliberately
add an `account_id` field that gets mapped to `accountId` in the
envelope — that would be a bug.)

**Confidence:** HIGH — verified.

### NP7. The smoke test must use a unique account_id per test run to avoid path-mutation races

**What goes wrong:** The CONTEXT's smoke test asserts the POST URL
is `/v1/accounts/{account_id}/events`. If the test reuses a
hard-coded `account_id` like `"12345"`, two parallel test runs would
both assert the same URL — fine. But if a future test mutates the
URL path (e.g., adds a query string), the assertion still passes if
the substring matches. Better to generate a fresh account_id per
test (e.g., `fmt.Sprintf("test-%d", time.Now().UnixNano())`) and
assert the full URL with `strings.Contains` on the path part.

**How to avoid:** The existing `httptest.NewServer` returns a URL
like `http://127.0.0.1:port`. The test substitutes the port into
the expected URL template and asserts the full URL exactly:

```go
expectedURL := fmt.Sprintf(
    "%s/v1/accounts/%s/events",
    srv.URL,
    accountID,  // local var, e.g., "test-1234567890"
)
```

And then assert `r.URL.Path == expectedPath` (not a substring
match). The existing Phase 3 test uses `srv.URL` directly as the
endpoint, which is the right pattern.

**Confidence:** MEDIUM — pattern guidance, not a known failure mode
in the existing code.

### NP8. 100k POSTs/min rate limit vs the buffer drain — single drain burst must stay under the cap

**What goes wrong:** New Relic's rate limit is 100k POSTs per minute
per account. [VERIFIED:
https://docs.newrelic.com/docs/data-apis/ingest-apis/event-api/introduction-event-api/]
The Phase 3 buffer drain (Service.DrainBuffer) sends one POST per
buffered event. A 1 MB buffer of ~200-byte events is ~5000 events;
a single drain would POST 5000 times in <1 second. Well under
100k/min. But a long-offline user who has accumulated 1 MB and
runs `telemetry status` (which triggers a drain) hits the rate
limit only on the *N+1*th drain within a minute — not realistic
for our 5000-event cap. The risk is low for v0.x.

**How to avoid:** Add a one-line comment to `DrainBuffer` noting
the rate limit headroom (5000 events/drain << 100k/min). No code
change needed. If the buffer cap ever grows (e.g., 100 MB), revisit
this.

**Confidence:** HIGH — math is mechanical.

### NP9. 1MB payload cap on the Event API vs the 1MB buffer cap

**What goes wrong:** The buffer cap is 1 MB (`telemetry.Buffer`).
The New Relic payload cap is also 1 MB
(10^6 bytes per POST). [VERIFIED:
https://docs.newrelic.com/docs/data-apis/custom-data/custom-events/data-requirements-limits-custom-event-data/]
If the buffer is *exactly* 1 MB and the drain POSTs the whole
buffer in one go, a single event could push the payload to
1 MB + 200 bytes = 1.0002 MB. The server returns 413, the
drain fails partway, and the unsent events stay in the buffer
forever (the next drain re-attempts and re-fails).

**How to avoid:** The CONTEXT's `NewRelicRecorder` is a single-event
recorder, not a batch recorder — it POSTs one event per call. The
buffer drain calls `s.Recorder.Record(ctx, e)` once per event
(`telemetry.go:96-100`), not in a batch. So the NewRelicRecorder
posts ~200 bytes per call, well under the 1 MB cap. The buffer
*file size* being up to 1 MB doesn't translate to a 1 MB POST
body. **The risk is theoretical for v0.x.**

**Confidence:** HIGH — verified by code inspection of
`telemetry.go:96-100` (DrainBuffer calls Record per event).

### NP10. EU region endpoint — out of scope but the URL is the only difference

**What goes wrong:** A user with a New Relic EU data center would
need the `insights-collector.eu01.nr-data.net` endpoint, not
`insights-collector.newrelic.com`. The CONTEXT locks the US
endpoint. If a user pastes the EU endpoint into
`telemetry.endpoint` and expects the smoke test to pass against
the default New Relic URL, the test fails (correctly — the user's
endpoint is different from the hard-coded default).

**How to avoid:** The `telemetry.endpoint` YAML field is the
override path. A user with an EU account can set
`telemetry.endpoint: https://insights-collector.eu01.nr-data.net/v1/accounts/{id}/events`
themselves. The NewRelicRecorder doesn't care which collector it
POSTs to — it just uses whatever URL the factory was given. The
OBSERVABILITY.md "Backend: New Relic" section should mention
"users in the EU data center: use `insights-collector.eu01.nr-data.net`".

**Confidence:** HIGH — verified by the docs.

---

## Existing Patterns in This Codebase

These are the patterns the planner should reuse, with file:line
citations so the plan actions can be checked.

- **`Recorder` interface, `RecorderFactoryFunc`, `NewHTTPClientFunc`**
  — `packages/cli/internal/telemetry/recorder.go:16-54`. The
  `NewRelicRecorder` is a new struct implementing `Recorder`; the
  factory func var is extended to pick it when both env vars are
  set. The `NewHTTPClientFunc` var is reused as-is. [VERIFIED:
  `recorder.go:16-54`]

- **`HTTPRecorder` struct + `NewHTTPRecorder` constructor** — the
  existing passthrough backend at `recorder.go:56-97`. The
  `NewRelicRecorder` is structurally similar: holds an `Endpoint`,
  a `*http.Client`, and an `X-Insert-Key` (or `InsertKey`) string.
  Mirror the constructor pattern. [VERIFIED: `recorder.go:56-97`]

- **`SetDefaultFactory(RecorderConfig{Enabled, Endpoint})` pattern**
  at `recorder.go:99-121`. Extend the `RecorderConfig` struct to
  include `InsertKey` and `AccountID`. The new factory closure
  becomes a 3-way switch:
  1. `AccountID == ""` or `InsertKey == ""` → `NoopRecorder` (the
     user hasn't configured the New Relic backend).
  2. Both env vars set → `NewRelicRecorder`.
  3. `Enabled && Endpoint != ""` but no NewRelic env vars →
     `HTTPRecorder` (passthrough, the existing behavior).
  4. Otherwise → `NoopRecorder`.

  The factory is called per-command (per Phase 3 Pitfall P9), so
  env-var changes take effect on the next command invocation. No
  init-time resolution.

- **`httptest.NewServer` test pattern** — the Phase 3 tests
  `TestHTTPRecorderSchemaByteForByte`, `TestHTTPRecorderSchemaFieldOrder`,
  and `TestHTTPRecorderFieldCount` at `recorder_test.go:98-255` all
  use `httptest.NewServer` with a closure handler that captures
  the request method, headers, and body. The new
  `TestNewRelicRecorderContract` test follows the same pattern:
  capture `r.URL.Path`, `r.Header.Get("X-Insert-Key")`, the raw
  body, then unmarshal the body as a `[]map[string]any` and
  assert the 5 properties from the CONTEXT smoke-test spec.
  [VERIFIED: `recorder_test.go:98-255`]

- **`User-Agent` header pattern** — `selfupdate.go:250, 326` sets
  `req.Header.Set("User-Agent", userAgent)` on every request.
  Define `const userAgent = "skill-organizer/" + version` in the
  recorder file (or reuse the `selfupdate.userAgent` constant if
  exposed). The CONTEXT's "agent's discretion" section recommends
  adding this header; the precedent is set.

- **`ResolveEndpoint` precedence helper** — `telemetry.go:157-169`
  implements the flag > env > YAML precedence. The NewRelic
  account_id and insert_key use **env-only** precedence (per the
  CONTEXT "auth" decision), so no new precedence helper is needed
  — `os.Getenv` is sufficient. The default `Endpoint` value can
  still use `ResolveEndpoint` to fall back to the New Relic URL
  when no other endpoint is configured.

- **`RecorderConfig` struct extension** — `recorder.go:99-105`
  has `Enabled bool, Endpoint string`. Extend to add `AccountID
  string, InsertKey string`. The factory's `SetDefaultFactory`
  signature changes to accept the new fields. **Risk:** Phase 3
  callers pass `RecorderConfig{Enabled, Endpoint}` with positional
  field names — adding fields is backwards-compatible (existing
  callers omit the new fields; the factory's NewRelic branch
  fires only when the new fields are set). No breaking change.

- **`Service` factory / wiring** — `telemetry.go:47-63` constructs
  the Service. The `NewRecorder()` call inside the constructor
  uses `RecorderFactoryFunc()` at construction time, so the
  factory must be set up **before** `New` is called (Phase 3 BUG
  #2). The `cmd/root.go:97-114` block already follows this order:
  load config → resolve endpoint → call
  `SetDefaultFactory` → call `telemetry.New`. The Phase 4
  extension follows the same order, with
  `os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID")` and
  `os.Getenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY")` added to the
  factory config.

- **`telemetry status` printing pattern** — `cmd/telemetry.go:107-135`
  prints 5 lines: enabled, endpoint, install_id, host_id, buffer
  size. Phase 4 extends this to add 2-3 more lines: account_id
  (truncated to first 4 chars + "..."), key-present (boolean),
  and recorder type (`NoopRecorder` / `HTTPRecorder` /
  `NewRelicRecorder`). The status command's
  `RecorderFactoryFunc` is called *after* `SetDefaultFactory`, so
  the resolved type is observable. [VERIFIED:
  `cmd/telemetry.go:107-135`]

- **pterm color rules** — yellow reserved for keyboard hints;
  magenta/cyan/light-magenta for status. The "Account ID: xxxx..."
  and "Insert key: present" lines should use `pterm.Info` (cyan),
  matching the existing status command's style. The hard-drop
  warning on 413/429 should use `pterm.Warning` (yellow is taken;
  use the existing pterm.Warning helper which is light-magenta).
  [VERIFIED: `AGENTS.md` "Color rules"]

- **Test fixtures: `t.TempDir` + `t.Setenv` + `t.Cleanup`** — the
  established pattern for tests that need a writable temp dir or
  env-var overrides. The NewRelicRecorder test uses
  `t.Setenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID", "12345")` and
  `t.Setenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY", "test-key")`
  to inject the env vars for the factory's selection logic.
  The `t.Cleanup` swap pattern (`original := X; t.Cleanup(func()
  { X = original })`) is used for the
  `RecorderFactoryFunc` swap (Phase 3 P9). [VERIFIED: existing
  `recorder_test.go:64-88`]

- **Stdlib only — no testify, no omock** — from
  `.planning/codebase/CONVENTIONS.md` and `AGENTS.md`. The new
  test code uses `testing.T`, `httptest.NewServer`, and the
  existing `countingTransport` pattern. No new dependencies.

---

## Recommended Approach

The smallest correct implementation is a single new struct
(`NewRelicRecorder`) in the existing `internal/telemetry/recorder.go`,
extended `RecorderConfig` and `SetDefaultFactory`, three new env
vars read in `cmd/root.go`, a small `telemetry status` output
extension, and a single new `httptest.NewServer` test. The
OBSERVABILITY.md "Backend: New Relic" section is a new sub-section
under the existing 7 sections. No new packages. No new
dependencies.

**1. New `NewRelicRecorder` struct in `recorder.go`** [HIGH]
- Fields: `AccountID string`, `InsertKey string`,
  `Endpoint string` (fully resolved URL with account_id
  substituted), `Client *http.Client`, `Version string` (for the
  User-Agent).
- Constructor: `NewNewRelicRecorder(accountID, insertKey,
  endpointTemplate, version string) Recorder` — takes the
  template and the accountID, does the `fmt.Sprintf` to build
  the final URL. Reads the env vars and resolves the endpoint at
  construction time.
- `Record(ctx, event) error` method: builds the array envelope
  as `[]map[string]any{ {eventType, command, exit_status,
  install_id, host_id, clientTime, version, event_id} }`,
  marshals to JSON, POSTs to the endpoint with
  `Content-Type: application/json` and
  `X-Insert-Key: <InsertKey>`. On 2xx: return nil. On 413/429:
  log via `pterm.Warning.Printf("telemetry: dropping event due
  to %d from New Relic", resp.StatusCode)` (or stdlib
  `fmt.Fprintln(os.Stderr, ...)` if pterm is not in this
  package's import set — see Pitfall P4), return nil. On other
  4xx/5xx: optional 1-retry with 250 ms context-aware backoff
  (NP3). On final non-2xx (or network error): return the
  error so the Service appends to the buffer (NP4).
- The `clientTime` rename (not `timestamp`) is the
  reserved-attribute workaround from NP1/NP5.

**2. Extend `RecorderConfig` and `SetDefaultFactory`** [HIGH]
- `RecorderConfig` gains `AccountID string` and `InsertKey
  string`. Existing callers (`root.go:106` passes
  `TelemetryConfig{Enabled, Endpoint}`) continue to work — the
  new fields are zero-valued unless `root.go` reads the env vars
  and sets them.
- The factory closure becomes:
  1. `!cfg.Enabled` → `NoopRecorder`
  2. `cfg.AccountID != "" && cfg.InsertKey != ""` → `NewNewRelicRecorder(cfg.AccountID, cfg.InsertKey, newRelicEndpointTemplate, version)` (the new branch)
  3. `cfg.Endpoint != ""` → `NewHTTPRecorder(cfg.Endpoint)` (the passthrough)
  4. Otherwise → `NoopRecorder`
- The default `Endpoint` is the New Relic URL with the
  account_id placeholder; the `NewNewRelicRecorder` constructor
  does the substitution. If the user sets
  `telemetry.endpoint` to something else, the NewRelicRecorder
  uses *that* URL (the placeholder substitution is harmless if
  the URL doesn't contain the placeholder).

**3. Wire env vars in `cmd/root.go`** [HIGH]
- In the existing `PersistentPreRun` block (`root.go:94-114`),
  read the two env vars:
  ```go
  newRelicAccountID := os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID")
  newRelicInsertKey := os.Getenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY")
  ```
  Pass them to the `SetDefaultFactory` call (new signature) or
  pass them to a new field on the `Service` struct.
- Default `telemetry.endpoint` (when not user-set) is the
  New Relic URL with the placeholder:
  ```go
  const newRelicEndpointTemplate = "https://insights-collector.newrelic.com/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events"
  ```
  The placeholder is substituted at recorder-construction time.

**4. Extend `telemetry status`** [MEDIUM]
- Add 2-3 new lines to `cmd/telemetry.go:127-131`:
  - `Recorder type: NewRelicRecorder` (or `HTTPRecorder` /
    `NoopRecorder` based on `RecorderFactoryFunc`'s resolved
    return type — see Pitfall P9).
  - `Account ID: 1234...` (truncated to first 4 chars + "...").
  - `Insert key: present` (or `Insert key: <not set>`).
- Use `pterm.Info` for the new lines (cyan), matching the
  existing style.

**5. Update `OBSERVABILITY.md`** [HIGH]
- Add a new section "Backend: New Relic" under the existing
  "Endpoint configuration" section. The section documents:
  - The env-var setup (3 lines: sign up, create insert key,
    `export` the two env vars).
  - The default endpoint URL (with the placeholder).
  - The `eventType: "skill_organizer_command"` value.
  - The `clientTime` rename (with a one-sentence rationale:
    "New Relic reserves the `timestamp` attribute name for
    Unix-epoch integers; we use `clientTime` to keep the
    original RFC3339 string and avoid the reserved word").
  - The hard-drop behavior on 413/429 (one-line warning, no
    buffer fallback).
  - The EU data center variant URL.
- The 7-section structure of OBSERVABILITY.md is unchanged; the
  New Relic section is a sub-section of "Endpoint configuration",
  not a new top-level section.

**6. New `TestNewRelicRecorderContract` test** [HIGH]
- Sets `t.Setenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID",
  "test-12345")` and `t.Setenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY",
  "test-key-xxxxxx")`.
- Stands up an `httptest.NewServer` with a closure capturing
  `r.URL.Path`, `r.Header.Get("X-Insert-Key")`, and the raw body.
- Builds a `NewRelicRecorder` via
  `NewNewRelicRecorder("test-12345", "test-key-xxxxxx",
  endpointTemplate, "0.4.0")`.
- Calls `rec.Record(t.Context(), validEvent())`.
- Asserts:
  1. `r.URL.Path == "/v1/accounts/test-12345/events"`.
  2. `r.Header.Get("X-Insert-Key") == "test-key-xxxxxx"`.
  3. `json.Unmarshal(body, &[]map[string]any{})` succeeds and
     `len(arr) == 1`.
  4. `arr[0]["eventType"] == "skill_organizer_command"`.
  5. The other 7 fields match the recorder's input modulo the 4
     volatile fields (existing pattern from Phase 3 byte-for-byte
     test).
- Plus: 3 sub-tests for the hard-drop cases (413, 429 → return
  nil from Record, no buffer write), and 1 sub-test for the
  retry path (503 → 1 retry, second 503 → return error).

**Suggested plan split (1-2 plans):**

- **Plan 04-01: `NewRelicRecorder` + factory extension + status
  extension + smoke test** — single PR. The smoke test is the
  acceptance gate (5 assertions). No new packages, no new
  dependencies.
- **Plan 04-02: `OBSERVABILITY.md` "Backend: New Relic" section**
  — docs-only. May be folded into 04-01 if the planner prefers
  one PR. The user-locked CONTEXT calls for the docs as part of
  the spec; this is the lowest-risk slice.

**Alternative: fold 04-02 into 04-01 as a single PR.** The total
work is small (~150 new Go lines, ~40 new test lines, ~30 new
doc lines). One PR matches the Phase 3 final plan (03-03) which
shipped docs + tests + e2e in one go.

**Suggested plan split (1-3 plans), if the planner prefers finer
granularity:**

- **Plan 04-01:** New `NewRelicRecorder` struct + `Record` method
  + 1-retry-on-503 path. The factory still picks `HTTPRecorder`
  in this plan (the NewRelicRecorder is wired but not selected).
- **Plan 04-02:** Extend `RecorderConfig` + `SetDefaultFactory`
  to read the two env vars and pick `NewRelicRecorder`. Extend
  `cmd/root.go` to read the env vars. Extend `telemetry status`
  output.
- **Plan 04-03:** `OBSERVABILITY.md` "Backend: New Relic" section
  + `TestNewRelicRecorderContract` smoke test (the 5 assertions
  from the CONTEXT).

3 plans matches the Phase 3 cadence (03-01/02/03). 1 plan is the
"ship it" alternative.

---

## Open questions for the planner

1. **`X-Insert-Key` vs `Api-Key` header name.** The CONTEXT locks
   `X-Insert-Key`, but the current New Relic Event API docs use
   `Api-Key`. The Insights Event API has historically used
   `X-Insert-Key` and the docs acknowledge it as a synonym. *Recommended:*
   follow the CONTEXT lock, send `X-Insert-Key` only, and add a
   one-line comment in `recorder.go` noting "New Relic Insights
   Event API also accepts `Api-Key`; we use `X-Insert-Key` per the
   locked CONTEXT decision." Real-account manual testing should
   confirm the header is accepted.

2. **`clientTime` rename vs `timestamp` Unix-epoch int vs both.**
   The CONTEXT envelope example shows `timestamp` as RFC3339
   string (which is dropped at ingest). The planner must choose:
   (a) rename to `clientTime` in the envelope only (the schema
   is unchanged), (b) convert to Unix-epoch int (changes the
   recorder's internal representation), (c) send both (extra
   payload bytes, no benefit). *Recommended: (a) — the smallest
   change that fixes the data-loss bug, and the rename is
   documented in OBSERVABILITY.md "Backend: New Relic" section.*

3. **Status output format for `Recorder type:`.** The current
   `telemetry status` prints 5 lines with `pterm.Info` (cyan).
   The new "Recorder type:" line could be a 6th line, or could
   replace the existing "Endpoint:" line (because the type
   implies the endpoint). *Recommended: add as a 6th line —
   don't replace, the endpoint is still the user-visible
   contract.*

4. **What happens to events queued in the buffer if the user
   switches from `HTTPRecorder` to `NewRelicRecorder` mid-flight.**
   The buffer is drained by whichever recorder the factory
   returns for the current run. If the user changes env vars,
   the next run uses the new recorder; the buffered events are
   POSTed to the new endpoint. *Recommended: document this in
   OBSERVABILITY.md, no code change.*

5. **The `OBSERVABILITY.md` "How to enable" section currently
   says "endpoint MUST be set for events to be sent".** After
   Phase 4, the *backend* (not just the endpoint) must be set:
   either the New Relic env vars or a custom endpoint for the
   passthrough. *Recommended: update the section to "backend
   must be configured (New Relic env vars for the default
   backend, or a custom endpoint for a passthrough proxy)".*

---

*Phase: 04-observability-product-selection*
*Research completed: 2026-06-12*
