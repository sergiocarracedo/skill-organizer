# Phase 5 — Local-only anonymous telemetry (REQ-10) — Discussion Log

**Gathered:** 2026-06-12
**Mode:** standard
**Status:** Ready for planning (companion to `05-CONTEXT.md`)

This file is for human audit trails only. Downstream agents
(read 05-CONTEXT.md) should not consult it.

---

## Area A — `telemetry enable` without an endpoint

The original question was "what does `telemetry enable` do when
no `telemetry.endpoint` is configured?" The user pivoted in the
first answer to a substantially different design.

**Options considered:**

1. Require endpoint at enable time (recommended in
   `discuss-phase`'s script).
2. Allow enable without endpoint, warn on each run.
3. Allow enable, prompt for endpoint interactively.

**User's verbatim choice (free-text, question 1):**

> "no endpoint, put attention. We should have an observability
> interface, and a noop implementation (to use in test and whe
> the user dont want to be tracked), and a NewRelic implmentation,
> which includes the endpoitn the token, etc, those are no thing
> use can config (are development vars). Again user only can set
> if we will use real telemetry or noop via (telemetry.enabled)
> prop in the config"

**Rationale captured:** The endpoint and token are
**build-time vars** (set via `-ldflags` at release-build time).
The user never configures them. The only user-facing config
is `telemetry.enabled`. The recorder interface has exactly two
implementations: `Noop` and `NewRelic`. The `HTTPRecorder`
passthrough is dropped entirely.

**Trade-off explicitly accepted by the user:** the NewRelic
API key is now in the binary. Anyone with the binary can spam
the NewRelic account. The user accepted this in exchange for
"no backend to run" and "one-key opt-in for users."

**Questions 2 and 3 were skipped** — they were about the
status display when enabled-without-endpoint, which is no
longer a reachable state under the new design (the endpoint is
baked in or the build routes to noop).

---

## Area B — `telemetry wipe` semantics and `telemetry disable`

**Options considered for wipe scope:**

1. Buffer + both IDs (recommended).
2. Buffer + install_id only.
3. Buffer only.

**User's verbatim choice (question 1):** "Buffer + both IDs
(Recommended)."

**Options considered for disable-vs-wipe:**

1. Disable stays non-destructive; wipe is explicit (recommended).
2. Disable also wipes, with a confirmation prompt.
3. Add `disable --wipe` flag.

**User's verbatim choice (question 2):** "Disable stays non-
destructive; wipe is explicit (Recommended)."

**Rationale captured:** the two commands have distinct
intents. `disable` is "stop recording" (reversible, keeps the
buffer so re-enabling doesn't lose pending data). `wipe` is
"forget me" (deletes the on-disk buffer; this is the GDPR
right-to-erasure command).

**Note for the planner:** since the schema simplification in
Area D drops `install_id` and `host_id`, "wipe" effectively
collapses to "delete the on-disk buffer file." Both IDs being
gone means there's no ID file to delete or rotate. The
planner should still implement `wipe` as a distinct command
(it has the GDPR semantics) but the implementation is just
`os.Remove(bufferPath)`.

---

## Area C — Privacy doc content + structure

**Options considered for doc location:**

1. Section in OBSERVABILITY.md (recommended in the script).
2. Separate PRIVACY.md.
3. Both.

**User's verbatim choice (question 1):** "Separate PRIVACY.md."

**Options considered for content (multi-select):**

- Field-by-field disclosure.
- Legal basis + retention.
- Data-controller statement.
- Schema-change protocol.

**User's verbatim choice (question 2):** "Field-by-field
disclosure, Legal basis + retention, Data-controller
statement, Schema change protocol" — all four.

**Options considered for first-run prompt copy:**

1. Update prompt copy to mention the 7 fields (recommended).
2. Don't change the prompt.
3. Verbose prompt with all fields listed.

**User's verbatim choice (question 3):** "Don't change the
prompt." (with the implicit understanding that the new
schema is 5 fields, not 7 — see Area D).

**Rationale captured:** a separate `PRIVACY.md` is more
visible and easier to share with a DPO or auditor. The
first-run prompt stays minimal; the user can read
`OBSERVABILITY.md` (linked from `--help`) and `PRIVACY.md`
(linked from `OBSERVABILITY.md`) for full detail.

**Note for the planner:** the user later said the first-run
prompt should mention `telemetry disable` as the off-ramp.
This is a copy tweak, not a structural change.

---

## Area D — `install_id` vs `host_id` (schema simplification)

**Options considered:**

1. Keep both (recommended in the script).
2. Collapse to one ID.
3. Make them semantically different.

**User's verbatim choice (question 1):** "we want anonimous
data, so those ids i guess makes no sense"

**This is a major schema change.** The user's intent is
"true anonymity" — no persistent identifiers at all, even
random ones, because random UUIDs are still pseudonymous and
technically linkable. Dropping both IDs is the only way to
fully eliminate linkability.

**Implications captured:**

- The 7-field schema in Phase 3 shrinks to a 5-field schema:
  `command`, `exit_status`, `timestamp`, `version`, `event_id`.
  `event_id` is per-event random, not per-user, so it cannot
  link events.
- `identity.go` is deleted. `LoadOrCreate` and `RotateHostID`
  are gone. The `telemetry rotate-host-id` subcommand is
  removed.
- The `<appDir>/install_id` and `<appDir>/host_id` files are
  no longer written. Existing files on disk after upgrade are
  ignored.
- The Phase 3 byte-for-byte schema test is updated to assert
  the new 5-field shape.
- `wipe` (from Area B) collapses to "delete the buffer file."

**Options considered for randomness test:**

1. Source-lock to `crypto/rand` (recommended).
2. Property test (1000 runs, no collisions, no machine bias).
3. Both.

**User's verbatim choice (question 2):** "Lock source: must use
crypto/rand (Recommended)."

**Rationale captured:** static check, catches accidental
refactors that swap to `math/rand`. The runtime property test
adds noise without catching new failure modes.

---

## Area E — `telemetry status` output shape

**Options considered:**

1. Compact (5 lines: Enabled, Recorder, Endpoint, Buffer size,
   Version) — recommended in the script.
2. Verbose (8 lines: also Account ID prefix, Insert key
   presence, Last event sent).
3. Minimal (2 lines: Enabled, Recorder).

**User's verbatim choice (question 1):** "just if enabled or not"

**Rationale captured:** the user wants the absolute minimum.
Two lines: `Enabled` and `Recorder`. The user can read
`OBSERVABILITY.md` for everything else.

**Note for the planner:** the verbose 8-line version was
introduced in Phase 4 because there was user-configurable
account_id + key presence to display. With the build-time
wiring, the user has no action to take on those values, so
they don't need to be in `status`.

**Options considered for first-run behavior:**

1. Keep prompt + add `telemetry disable` mention (recommended).
2. Add prompt footer with PRIVACY.md link.
3. Require explicit `telemetry` subcommand.

**User's verbatim choice (question 2):** "Keep prompt + add
`telemetry disable` to exit (Recommended)."

**Rationale captured:** the existing first-run prompt stays.
The only change is a one-line copy tweak: append "(use
`telemetry disable` to turn off at any time)" so the user
knows the off-ramp.

---

## Areas delegated to agent's discretion

The `discuss-phase` script invited the planner to make these
choices without further user input:

- Whether to keep `User-Agent: skill-organizer/<version>` on
  `NewRelicRecorder` POSTs (Phase 4 added it; planner decides).
- Whether `NewRelicRecorder` keeps the 413/429 hard-drop and
  503 single-retry from Phase 4.
- Whether `telemetry wipe` prints a one-line warning before
  deleting the buffer.
- Whether to rename the `Recorder` interface to `Observer`.

See `05-CONTEXT.md` "Agent's Discretion" for the captured
defaults.

---

## Deferred ideas (carried forward from prior phases)

- Multi-tenant account routing (one user, multiple projects,
  each with its own account_id) — not in scope.
- Per-tool breakdown dashboards in New Relic — not in scope.
- Migration to a paid tier — not in scope.
- Other managed backends (Datadog, Sentry, BetterStack,
  Grafana Cloud) — not in scope.
- Custom proxy that ingests our flat schema and forwards to
  New Relic — not in scope; the `HTTPRecorder` is dropped,
  so this path is gone in v0.x.

---

## Net effect on v0.x

The CLI ships with one observability implementation
(`NewRelicRecorder`) plus a noop, anonymous by construction,
opt-in only, with one user-facing knob (`telemetry.enabled`)
and an explicit erasure command (`telemetry wipe`). The
maintainer is the only person who ever configures a key,
and they do it once at build time via `-ldflags`.

---

*Phase: 05-local-only-anonymous-telemetry*
*Discussion log: 2026-06-12*
