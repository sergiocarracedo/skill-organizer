# Phase 5 — Local-only anonymous telemetry (REQ-10) — Context

**Gathered:** 2026-06-12
**Mode:** standard
**Status:** Ready for planning

<domain>
## Phase Boundary

`skill-organizer` ships an opt-in, **anonymous** telemetry layer with
**no user-facing backend configuration** and **no pseudonymous
identifiers**. The CLI exposes a `telemetry` subcommand
(`enable`, `disable`, `status`, `wipe`) and a single config key
(`telemetry.enabled`). The schema, privacy posture, and build-time
backend wiring are documented in a new top-level `PRIVACY.md`
and an updated `OBSERVABILITY.md`.

Concretely Phase 5:

1. **Collapses the recorder API to two implementations**
   (`Noop`, `NewRelic`) — drops `HTTPRecorder` and the
   "bring your own endpoint" passthrough path.
2. **Bakes the New Relic endpoint and token into the binary** at
   build time via `-ldflags`. The user never configures these.
3. **Drops `install_id` and `host_id` from the event schema.**
   The schema shrinks from 7 fields to 5. There is no way for the
   maintainer to link two events from the same machine.
4. **Adds `telemetry wipe`** as the GDPR right-to-erasure command.
   Wipe deletes the on-disk buffer (the only persistent data on
   the device). Since there are no IDs, that's all there is to
   delete.
5. **Adds `PRIVACY.md`** as the user-facing legal text. The first
   -run prompt and the existing `OBSERVABILITY.md` schema doc stay
   in their current shape, with `PRIVACY.md` linked from
   `OBSERVABILITY.md`.

The hard constraint "telemetry.enabled is the only user-facing
telemetry knob" is non-negotiable. Every other choice flows from it.

</domain>

<decisions>
## Implementation Decisions

### Recorder surface

- **Two implementations, not three.** The recorder interface
  stays the same (`Record(ctx, event) error`) but only two
  concrete types remain: `NoopRecorder` and `NewRelicRecorder`.
  The `HTTPRecorder` from Phase 3 is **removed** along with the
  factory branch that selects it.
- **Factory order: `NewRelicRecorder` > `NoopRecorder`.** When
  `telemetry.enabled` is true AND the binary was built with a
  New Relic endpoint + token, the factory returns a
  `NewRelicRecorder`. Otherwise it returns `NoopRecorder`.
- **No third Recorder type.** Power users who want a custom sink
  have no in-tree path in v0.x. Adding one is a future phase.

### New Relic endpoint + token are build-time vars

- The endpoint URL and insert key live in two `var` declarations
  inside the `internal/telemetry` package, e.g.

  ```go
  var (
      NewRelicEndpoint = "" // set via -ldflags at build time
      NewRelicAPIKey   = "" // set via -ldflags at build time
  )
  ```

- The release build sets them via
  `go build -ldflags "-X .../telemetry.NewRelicEndpoint=https://... -X .../telemetry.NewRelicAPIKey=..."`.
- A dev/CI build leaves them empty. The factory's emptiness check
  (see below) routes the dev build to `NoopRecorder` even when
  `telemetry.enabled` is true.
- The user **never** sees these vars and **never** sets them.
  They are not in `OBSERVABILITY.md` and not in the first-run
  prompt. The build script is the only surface.

### User-facing config: one key

- `telemetry.enabled` (bool, default `false`).
- The flag > env > YAML precedence from Phase 3 still applies:
  `--telemetry-enabled` flag > `SKILL_ORGANIZER_TELEMETRY_ENABLED`
  env var > `telemetry.enabled` YAML key.
- `telemetry.endpoint` (the Phase 3 user-configurable endpoint)
  is **removed** along with the env var and flag.
- `OBSERVABILITY.md` documents the one-key model and the build-time
  wiring in a short "How the maintainer builds the binary" section.

### Schema shrinks from 7 fields to 5

- **Dropped fields:** `install_id`, `host_id`. Rationale: the user
  wants true anonymity. A random pseudonymous identifier is
  still linkable in principle; the cleanest fix is to have no
  identifier at all.
- **Final schema (5 fields):**
  - `command` (string): the cobra subcommand name.
  - `exit_status` (int): 0 = success, 1 = error.
  - `timestamp` (string): RFC3339 UTC.
  - `version` (string): CLI semver.
  - `event_id` (string): 32 hex chars from 16 random bytes per
    event. **Per-event, not per-user** — useful for local buffer
    dedup, but cannot be used to link events.
- The Phase 3 byte-for-byte test is **updated** to assert the
  new 5-field schema. The test's role (canonical source of truth
  for the schema) is preserved.
- The `Event` struct in `internal/telemetry/event.go` drops
  `InstallID` and `HostID` fields and the `Validate` method is
  simplified accordingly.

### Identity module: removed

- `internal/telemetry/identity.go` is **deleted** along with
  `identity_test.go`.
- `LoadOrCreate`, `RotateHostID`, and the `telemetry rotate-host-id`
  subcommand are **deleted**.
- A **source-lock test** is added that asserts all random-byte
  generation in the package reads from `crypto/rand` (the source
  identifier of any linkable ID we keep — currently `event_id`).
  Static check via the existing `observability_test.go` pattern,
  no runtime property test needed.
- The two app-dir files `<appDir>/install_id` and
  `<appDir>/host_id` are no longer written. Existing files left
  on disk after upgrade are ignored (the loader doesn't read
  them).

### `telemetry wipe`

- New subcommand. Single action: delete the on-disk buffer file
  `<appDir>/telemetry-buffer.jsonl` if it exists.
- **No IDs to rotate** (they're gone). Wipe = "delete the
  buffer" is the entire implementation.
- Idempotent: running `wipe` on a clean app dir is a no-op that
  prints "Nothing to wipe."
- Reports what it did: "Wiped N bytes from telemetry-buffer.jsonl."

### `telemetry disable` stays non-destructive

- Same as Phase 3: flips `telemetry.enabled` to false, leaves
  the buffer file alone. Re-enabling doesn't lose pending data.
- The first-run prompt and `telemetry enable` both reference
  `telemetry disable` in their copy so the user knows how to opt
  out after opting in.

### `telemetry status` is minimal

- Two lines:
  - `Enabled: yes|no`
  - `Recorder: newrelic|noop`
- No endpoint, no account ID, no buffer size, no version. The
  user can read `OBSERVABILITY.md` or `PRIVACY.md` for the rest.
- The `recorderTypeName` helper in `cmd/telemetry.go` collapses
  from 3-way (`noop|http|newrelic`) to 2-way (`noop|newrelic`).

### Privacy documentation: separate `PRIVACY.md`

- New top-level file `PRIVACY.md` (sibling of `OBSERVABILITY.md`).
- Four required sections:
  1. **Field-by-field disclosure** — table of the 5 schema
     fields, what each is, and why it is anonymous. (The
     "anonymity" argument per field: `command` and `exit_status`
     are intrinsic to the call; `timestamp` is bucketed server-
     side or coarse; `version` is a public string; `event_id` is
     per-event random with no linkable seed.)
  2. **Legal basis and data retention** — consent (opt-in);
     backend retention limited by the New Relic free-tier
     8-day window; on-device buffer is 1 MB FIFO and survives
     only until the device is wiped or the buffer is rotated.
  3. **Data-controller statement** — the maintainer of
     `skill-organizer` is the data controller; the chosen
     processor is New Relic (US, DPA executed). Contact:
     GitHub issues on the repo.
  4. **Schema-change protocol** — explicit list of fields the
     project will **not** collect (paths, args, usernames, IPs,
     machine serials, hostnames, file contents). Any future
     schema change is a breaking change to `OBSERVABILITY.md`
     and `PRIVACY.md`, and a new versioned event type.
- `OBSERVABILITY.md` gains a one-line link to `PRIVACY.md` at
  the top: "For the privacy and data-protection posture, see
  `PRIVACY.md`."

### First-run prompt copy

- The first-run prompt is **unchanged** in structure: y/N
  question, default = no, sticky. Only one copy tweak: append
  "(use `telemetry disable` to turn off at any time)" so the
  user knows the off-ramp.
- The schema and privacy details are not surfaced in the prompt.
  The user can read `OBSERVABILITY.md` (linked from the CLI's
  help text) and `PRIVACY.md` (linked from `OBSERVABILITY.md`)
  for full detail.

### Agent's Discretion

- Whether to add `User-Agent: skill-organizer/<version>` to the
  `NewRelicRecorder` POSTs (Phase 4 added it; planner can keep
  or drop — the maintenance cost is trivial, the ops value is
  real, but the privacy argument is neutral).
- Whether the `NewRelicRecorder` keeps the 413/429 hard-drop and
  503 single-retry behavior from Phase 4 (recommendation: yes,
  these are backend-specific and stay as-is).
- Whether `telemetry wipe` should print a one-line warning that
  the user is about to delete buffered events (recommendation:
  yes, but no y/N confirmation — wipe is idempotent and
  opt-in-only; the user is consenting to deletion by running
  the command).
- Whether the recorder interface itself should be renamed
  (e.g. `Observer`) for clarity. Recommendation: no — the
  `Recorder` name is already in the v0.x code, in the docs,
  and on the cobra subcommand. Renaming has no behavioral value
  and breaks git blame.

</decisions>

<specifics>
## Specific Ideas

- The user explicitly stated: "We should have an observability
  interface, and a noop implementation (to use in test and when
  the user dont want to be tracked), and a NewRelic
  implementation, which includes the endpoint and the token,
  etc, those are not things user can config (are development
  vars). Again user only can set if we will use real telemetry
  or noop via (telemetry.enabled) prop in the config."
- The user explicitly stated: "we want anonymous data, so those
  ids i guess makes no sense" — referring to `install_id` and
  `host_id`.
- The user explicitly stated: "just if enabled or not" — for
  the `telemetry status` output.
- The first-run prompt stays as-is structurally; only a one-
  line copy tweak is added to mention `telemetry disable`.
- The user accepted that the **build-time key in the binary**
  is a known trade-off: anyone with the binary can spam the
  New Relic account. The maintainer accepts this in exchange for
  "no backend to run" and "one-key opt-in for users."

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `OBSERVABILITY.md` — the schema contract (will be updated to
  5 fields in plan 05-NN).
- `packages/cli/internal/telemetry/recorder.go` — current
  3-way factory; Phase 5 collapses to 2-way.
- `packages/cli/internal/telemetry/identity.go` — current
  install_id/host_id files; Phase 5 deletes the file.
- `packages/cli/internal/telemetry/event.go` — current 7-field
  `Event` struct; Phase 5 shrinks to 5 fields.
- `packages/cli/internal/telemetry/buffer.go` — on-disk JSONL
  spool, unchanged in shape.
- `packages/cli/internal/telemetry/recorder_test.go` — current
  byte-for-byte schema test; updated for the 5-field schema.
- `packages/cli/cmd/root.go:109-110` — current
  `SKILL_ORGANIZER_NEWRELIC_*` env-var reads; removed in
  Phase 5.
- `packages/cli/cmd/telemetry.go` — current
  enable/disable/status/rotate-host-id subcommands;
  `rotate-host-id` removed, `wipe` added.
- `.planning/phases/03-observability/03-CONTEXT.md` and
  `03-RESEARCH.md` — Phase 3 decisions that Phase 5 inherits
  (opt-in flow, buffer semantics, byte-for-byte test pattern).
- `.planning/phases/04-observability-product-selection/04-CONTEXT.md`
  and `04-RESEARCH.md` — Phase 4 decisions that Phase 5
  refines (New Relic as the backend, 413/429 hard-drop,
  503 single-retry).
- `.planning/REQUIREMENTS.md` — REQ-10 (the new requirement
  for Phase 5; planner should confirm it captures the 5-field
  schema, the build-time wiring, the no-HTTPRecorder rule, the
  `telemetry wipe` command, and the `PRIVACY.md`).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `Recorder` interface (`recorder.go:25-29`) — `Record(ctx, event)
  error`. Phase 5 keeps the interface name and signature;
  drops the `HTTPRecorder` implementation. The factory is
  rewritten in place.
- `RecorderFactoryFunc` package var (`recorder.go:38`) —
  swappable func var for test injection. Phase 5's new 2-way
  closure reuses this same pattern.
- `NoopRecorder` (`recorder.go:30-50`) — unchanged; stays as
  the default and the test fallback.
- `NewRelicRecorder` (Phase 4, `recorder.go:90-180`) — kept
  with the 413/429 hard-drop, 503 single-retry, and
  `X-Insert-Key` header behavior. Only the **construction**
  changes (reads from build-time `var`s instead of env vars).
- `telemetrypkg.ResolveEndpoint` (Phase 3, `internal/config/
  registry.go`) — Phase 5 **deletes** this function. No more
  user-configurable endpoint.
- `httptest.NewServer` pattern (Phase 3 and Phase 4) — the
  existing `TestNewRelicRecorder*` tests stay valid; the
  factory-swap tests collapse from 3 to 2 expected branches.
- `Event` struct (`event.go:23-50`) — Phase 5 drops
  `InstallID` and `HostID` fields and their `Validate` logic.

### Established Patterns

- **Func-var test injection** — every external dependency is a
  package-level func var. Phase 5 reuses this for the
  `NewRelicEndpoint` / `NewRelicAPIKey` build-time vars.
- **Atomic commits per task** — one task = one commit. The
  planner writes tasks in this style and the executor follows.
- **Byte-for-byte schema test** — Phase 3's strongest
  assertion. Phase 5's updated test is the canonical source of
  truth for the 5-field schema.
- **Source-lock test for `crypto/rand`** — recommended for
  Phase 5 to assert the `event_id` (and any future per-event
  random) reads from `crypto/rand` and not `math/rand`.
- **Empty-string guard for build-time vars** — when the
  `-ldflags` injection is missing, the factory routes to
  `NoopRecorder`. This is the dev-build escape hatch.

### Integration Points

- `recorder.go:38` (`RecorderFactoryFunc`) — rewrite the default
  closure to inspect `NewRelicEndpoint` and `NewRelicAPIKey`
  build-time vars and return `NewRelicRecorder` if both are
  non-empty AND `telemetry.enabled` is true; otherwise return
  `NoopRecorder`.
- `recorder.go` (new top-level section) — declare the two
  build-time `var`s with empty defaults and a doc comment
  explaining the `-ldflags` contract.
- `event.go:23-50` (`Event` struct) — drop `InstallID` and
  `HostID`. Update `Validate` to no longer check those fields.
- `identity.go` (whole file) — delete the file. Delete
  `identity_test.go` too.
- `telemetry.go` (`cmd/telemetry.go:30-31`) — remove the
  `telemetryNewRelicAccountID` and `telemetryNewRelicInsertKey`
  package-level func vars. Remove their call sites in the
  factory. Remove the `rotate-host-id` subcommand. Add the
  `wipe` subcommand.
- `root.go:109-110` — remove the env-var reads; remove
  `RecorderVersion` if the User-Agent decision goes to "drop"
  (defer to agent's discretion).
- `OBSERVABILITY.md` — update the schema example to 5 fields;
  remove the "Bring your own endpoint" section; remove the
  `SKILL_ORGANIZER_TELEMETRY_ENDPOINT` env var; remove the
  `SKILL_ORGANIZER_NEWRELIC_*` env vars; add a one-line link
  to `PRIVACY.md`.
- `PRIVACY.md` — new file, 4 sections per the decisions above.

</code_context>

<deferred>
## Deferred Ideas

- **End-to-end smoke test against the real New Relic account.**
  Out of scope for v0.x — Phase 4 used `httptest.NewServer` for
  the same reason (no real-account contact). The maintainer
  does the manual smoke test in a release candidate. A future
  phase can add a build-tag-gated test that reads the
  real endpoint from a secret and asserts a single POST.
- **Per-event anonymization review** (e.g. bucketing `timestamp`
  to hour granularity on the server side). Out of scope; the
  Phase 5 schema is what it is. If a future privacy review
  shows the timestamp is too granular, that's a schema change
  with a versioned event type.
- **Pluggable backend abstraction** (`telemetry.backend: newrelic
  | noop | future-sink`). Out of scope; the binary ships with
  New Relic hardcoded. A future phase can introduce a
  `Recorder` factory with backend selection.
- **Different backend** (Datadog, Sentry, BetterStack, Grafana
  Cloud, self-hosted). Out of scope; New Relic is the
  v0.x-only choice. Re-evaluating the backend is a future
  phase with its own decision record.
- **Per-user opt-in audit log** (a record of which users have
  enabled telemetry, kept on the maintainer's side). Out of
  scope; New Relic is the only place the data lives, and the
  maintainer queries it directly.
- **Telemetry dashboard for end users** (a public page showing
  aggregated usage). Out of scope; the maintainer may share
  aggregated stats in a future blog post or release notes, but
  that is a docs decision, not a CLI feature.

</deferred>

---
*Phase: 05-local-only-anonymous-telemetry*
*Context gathered: 2026-06-12*
