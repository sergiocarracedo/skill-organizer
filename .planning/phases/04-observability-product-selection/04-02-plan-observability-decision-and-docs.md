---
wave: 2
depends_on:
  - 04-01-plan-newrelic-recorder-and-factory.md
files_modified:
  - OBSERVABILITY.md
  - .planning/PHASE-4-DECISION.md
  - packages/cli/internal/telemetry/recorder_test.go
  - .planning/phases/04-observability-product-selection/04-02-plan-SUMMARY.md
autonomous: true
single_layer_justified: false
requirement: REQ-9
objective: "Close out Phase 4 with the user-facing documentation: add a 'Backend: New Relic' sub-section under the existing 'Endpoint configuration' section in OBSERVABILITY.md (env-var setup, default endpoint URL, eventType value, the clientTime rename with rationale, hard-drop on 413/429, EU data center variant), and write .planning/PHASE-4-DECISION.md (the human-audit record of the product decision — chosen product, why, free-tier limits, per-event ingestion math, roll-over behavior). Add one e2e test for the NewRelicRecorder that exercises the recorder end-to-end through a fake httptest server with no real backend contact, asserting the recorder's behavior matches the OBSERVABILITY.md doc (the 5 CONTEXT assertions + the User-Agent + the hard-drop + the retry). Verifiable by go test ./... passing, OBSERVABILITY.md updated with the new sub-section, and .planning/PHASE-4-DECISION.md existing with all 6 required sections."
must_haves:
  - "go test ./... -count=1 passes (no regression on the 200+ existing tests + plan 04-01 tests)"
  - "OBSERVABILITY.md has a new sub-section 'Backend: New Relic' (or equivalently named) under the existing 'Endpoint configuration' section"
  - "OBSERVABILITY.md 'Backend: New Relic' sub-section documents: (a) the 4-line env-var setup, (b) the default endpoint URL with the $ACCOUNT_ID placeholder, (c) the eventType value 'skill_organizer_command', (d) the clientTime rename with the rationale (New Relic reserves 'timestamp' for Unix-epoch integers), (e) the hard-drop on 413/429 behavior, (f) the EU data center variant URL (insights-collector.eu01.nr-data.net)"
  - "OBSERVABILITY.md 'How to enable / disable' section is updated to mention the New Relic env vars (or the new 'Backend: New Relic' sub-section explicitly references them)"
  - ".planning/PHASE-4-DECISION.md exists at the repo root and contains at minimum: (1) chosen product (New Relic Insights Events API), (2) why (100 GB/month free, simple JSON ingest, X-Insert-Key auth), (3) free-tier limits (100 GB/month, 1 full user, 8-day retention), (4) per-event ingestion math (~60 MB/month at projected scale), (5) roll-over behavior (hard drop on 413/429), (6) the timestamp → clientTime rename rationale"
  - "TestNewRelicRecorderE2E_NoRealBackendContact passes (the end-to-end test from plan 04-01 is sufficient; this plan re-verifies the recorder works through a fake server, no DNS resolution, no real account)"
  - "OBSERVABILITY.md is between 150 and 220 lines (it grows from 157 lines by ~30-50 lines for the new sub-section)"
  - "The 'Backend: New Relic' sub-section is a sub-section of 'Endpoint configuration' (H2 within an H1, or a higher-level H2/H3 structure) — it does NOT introduce a new top-level section that violates the 7-section structure from Phase 3"
  - ".planning/PHASE-4-DECISION.md is between 30 and 100 lines (the audit record, not a full design doc)"
---

# Plan 04-02: OBSERVABILITY.md "Backend: New Relic" section, PHASE-4-DECISION.md, end-to-end verification

## Objective

Close out REQ-9 (Phase 4) with the user-facing documentation and the
human-audit record. This plan adds a "Backend: New Relic" sub-section
to `OBSERVABILITY.md` (under the existing "Endpoint configuration"
section) documenting the env-var setup, the default endpoint URL, the
`eventType` value, the `clientTime` rename with its rationale, the
hard-drop on 413/429, and the EU data center variant. It also writes
`.planning/PHASE-4-DECISION.md` — the audit record of the product
selection (chosen product, why, free-tier limits, ingestion math,
roll-over behavior, the `timestamp` → `clientTime` rename rationale).
The plan re-uses the `httptest.NewServer` test from plan 04-01 as the
end-to-end acceptance gate (the test exercises the recorder through a
fake server with no real backend contact).

## Context

Plan 04-01 shipped the `NewRelicRecorder` struct, the factory
extension, the env-var wiring in `cmd/root.go`, the `telemetry status`
output extension, and the 5-assertion `httptest.NewServer` smoke
test. This plan is the **closing** slice: it documents the new
backend for the user (OBSERVABILITY.md) and records the product
decision for the next planning cycle (PHASE-4-DECISION.md). No new
code; the only test is the e2e re-verification of the
`NewRelicRecorder` test from plan 04-01 (run with `-count=1` to
bypass the test cache and prove the test passes on a fresh run).

The OBSERVABILITY.md doc has 7 top-level sections from Phase 3
(per the Phase 3 plan 03 must-have):
1. What is collected
2. Schema
3. How to enable / disable
4. Endpoint configuration
5. Data retention
6. Privacy guarantees
7. FAQ

The new "Backend: New Relic" sub-section is a **sub-section** of
"Endpoint configuration" (an H3 under the existing H2), NOT a new
top-level section. This preserves the 7-section structure that the
Phase 3 `TestOBSERVABILITYHasAllSevenSections` test asserts.

The `.planning/PHASE-4-DECISION.md` file is the human-audit record.
It is NOT a design doc (that's `04-CONTEXT.md`) and NOT a
research report (that's `04-RESEARCH.md`). It is a short, single
source of truth for "we chose New Relic" with the supporting
numbers, the rationale, and the roll-over behavior. The next
planning cycle reads this file before considering a backend change.

The free-tier math from the CONTEXT (200 bytes × 10 invocations/user/day
× 1000 active users = ~2 MB/day ≈ 60 MB/month, comfortably under
100 GB/month) is captured in the DECISION file as the projected
ingestion rate, with the calculation made explicit. The roll-over
behavior (hard drop on 413/429, no buffer fallback) is the
operational contract.

## Tasks

<task id="04-02-01">
<name>Add the 'Backend: New Relic' sub-section to OBSERVABILITY.md</name>
<files>
- OBSERVABILITY.md
</files>
<action>
Append a new sub-section to `OBSERVABILITY.md`, just before the
"Data retention" section (which is at line 100 in the current
157-line file). The new sub-section is `### Backend: New Relic`
(H3 under the existing `## Endpoint configuration` H2, at
line 83). The content:

```markdown
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

Enumerate the envelope (8 keys per event):

- `eventType` — always `"skill_organizer_command"`. New Relic uses
  this to group events in the NRDB UI.
- `command`, `exit_status`, `install_id`, `host_id`, `version`,
  `event_id` — the 6 schema fields, sent with their snake_case
  names verbatim (NRQL is case-sensitive; RESEARCH NP2).
- `clientTime` — the RFC3339 UTC string from the `timestamp` field.
  **Renamed** from `timestamp` to dodge the New Relic reserved-
  attribute rule: the server reserves `timestamp` for Unix-epoch
  integers and silently drops an RFC3339 string sent in that
  field. The rename is an envelope-only transform; the flat
  7-field schema in OBSERVABILITY.md is unchanged. The
  HTTPRecorder (passthrough) still emits the field as `timestamp`.

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
```

This is a 50-60 line addition. The OBSERVABILITY.md doc grows from
157 to ~210 lines, well within the Phase 3 plan 03 must-have of
"between 140 and 200 lines" — RECOMMENDED: relax the upper bound
to 220 lines in this plan, since the new sub-section is required
for REQ-9 acceptance and is short and direct (no marketing prose).

Also update the existing "How to enable / disable" section
(currently lines 59-81) to add ONE line referencing the new
sub-section:

> The default backend is the New Relic Insights Events API — see
> the [Backend: New Relic](#backend-new-relic) sub-section below
> for setup. For a custom proxy, point `telemetry.endpoint` at
> the proxy URL and the CLI will POST the flat 7-field object
> (the HTTPRecorder passthrough mode).

This addition is 2-3 lines and keeps the 7-section structure
intact.

The 7 top-level sections (Phase 3 must-have) are unchanged. The
Phase 3 `TestOBSERVABILITYHasAllSevenSections` test continues to
pass.
</action>
<verify>
- `OBSERVABILITY.md` exists at the repo root
- `wc -l OBSERVABILITY.md` is between 150 and 220 lines
- The 7 top-level section headers from Phase 3
  (`## What is collected`, `## Schema`, `## How to enable / disable`,
  `## Endpoint configuration`, `## Data retention`,
  `## Privacy guarantees`, `## FAQ`) are all present
- A new `### Backend: New Relic` H3 sub-section exists under
  `## Endpoint configuration`
- The sub-section mentions: `eventType`, `clientTime`, the rename
  rationale, the 413/429 hard-drop, the EU data center variant
- `go test ./internal/telemetry/... -count=1 -run TestOBSERVABILITYHasAllSevenSections` passes
- `go test ./internal/telemetry/... -count=1` passes
- `go test ./... -count=1` passes
</verify>
<done>[ ]</done>
</task>

<task id="04-02-02">
<name>Write .planning/PHASE-4-DECISION.md (the human-audit record)</name>
<files>
- .planning/PHASE-4-DECISION.md
</files>
<action>
Create `.planning/PHASE-4-DECISION.md` at the repo root (next to
the existing `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`,
etc.). The file is 30-100 lines, written in the style of
`STATE.md`'s "Recent decisions" section: terse, dated, with
source links. The structure:

```markdown
# Phase 4 — Telemetry backend decision (REQ-9)

> Audit record of the v0.x telemetry backend selection. Read this
> file before considering a backend change in a future planning
> cycle. NOT referenced by downstream agents — for human audit
> only.
>
> Decided: 2026-06-12
> Phase: 04-observability-product-selection
> Source discussions: `.planning/phases/04-observability-product-selection/04-CONTEXT.md`,
> `04-RESEARCH.md`, `04-DISCUSSION-LOG.md`

## Decision

**Backend:** New Relic Insights Events API.
**URL template:** `https://insights-collector.newrelic.com/v1/accounts/$SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID/events`
**Auth:** `X-Insert-Key` header (from `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY` env var).

## Why New Relic

- 100 GB / month of free ingest (permanent free tier).
- Simple JSON event ingest (POST an array, set an auth header).
- Single header for auth (`X-Insert-Key`); no SDK, no client lib.
- 1 full user on the free tier (sufficient for a CLI author).
- 8-day retention on the free tier (sufficient for weekly review).

The other candidates (Grafana Cloud, Sentry, BetterStack,
Logtail, Highlight.io, OpenObserve, SigNoz, HyperDX) are
documented in `04-RESEARCH.md`. The deciding factors were
(1) the largest free tier by 2-3 orders of magnitude and (2)
the simplest ingest contract (one POST, one header).

## Ingestion math (projected)

- Event payload: ~200 bytes (the 7-field schema, JSON-encoded).
- Invocations per user per day: ~10 (a developer using the CLI
  during the workday).
- Active users (projected): ~1,000 in the first 6 months.
- Daily volume: 200 B × 10 × 1,000 = 2 MB / day.
- Monthly volume: 2 MB × 30 = 60 MB / month.
- Free tier cap: 100,000 MB / month (100 GB).
- Headroom: 60 / 100,000 = 0.06% of the free tier cap.

The free tier is comfortable for the projected scale by 3
orders of magnitude. If the actual scale is 10× higher (10,000
active users), the monthly volume is 600 MB, still 0.6% of the
cap. The 100 GB cap is the soft limit that triggers hard drops.

## Roll-over behavior

When the recorder receives 413 (Payload Too Large) or 429 (Too
Many Requests) from New Relic, it:

1. Logs a one-line warning via pterm (cyan, per the project's
   color rules).
2. Returns `nil` to the caller (the event is dropped, NOT
   buffered).

This is the v0.x contract. The local on-disk buffer (1 MB
JSONL spool) is for network-down / offline cases, NOT for
server-quota cases. Buffering a server-rejected event would
create an infinite drain loop (next drain re-POSTs the same
event, server rejects again, FIFO eviction eventually drops it
after thousands of events). The hard drop is the smallest
correct behavior.

When the 100 GB free tier is reached, New Relic returns 429 on
all subsequent POSTs. The recorder hard-drops until the next
monthly reset. There is no paid-upgrade flow in v0.x; if the
free tier proves too small, that's a future phase with its own
decision.

## `timestamp` → `clientTime` rename

The New Relic Insights Events API reserves the `timestamp`
attribute for Unix-epoch integers. Sending an RFC3339 string
in the `timestamp` field is silently dropped at ingest (no
error, no warning — the field is just absent from the
resulting NRDB event).

To preserve the RFC3339 string (which carries the user's local
command time, not the server's receive time), the
`NewRelicRecorder` renames the field to `clientTime` in the
**envelope only**. The flat 7-field schema in
`OBSERVABILITY.md` is unchanged. The HTTPRecorder (passthrough
mode) still emits the field as `timestamp`. The rename is
documented in the OBSERVABILITY.md "Backend: New Relic"
sub-section.

## Future changes (out of scope for v0.x)

- Multi-tenant account routing (one user, multiple projects,
  each with its own account_id) — defer to a future phase.
- Migration to a paid tier — defer; the free tier is sufficient
  for the projected scale by 3 orders of magnitude.
- Other backends (Datadog, Sentry, BetterStack, Grafana Cloud) —
  defer; a single NewRelicRecorder is the v0.x contract.
- EU data center support — already supported via the
  `telemetry.endpoint` override; no code change needed.
- Custom proxy that ingests the flat schema and forwards to
  New Relic — already supported via the HTTPRecorder
  passthrough; no code change needed.
```

The file is ~75 lines, well within the 30-100 line range. It
references `04-CONTEXT.md`, `04-RESEARCH.md`, and
`04-DISCUSSION-LOG.md` for the full design context, so a future
planner can read the trail in order.
</action>
<verify>
- `.planning/PHASE-4-DECISION.md` exists at the repo root
- `wc -l .planning/PHASE-4-DECISION.md` is between 30 and 100
  lines
- The file contains all 6 required sections (in any order):
  chosen product (New Relic), why (free-tier + simple ingest),
  free-tier limits (100 GB/month, 1 user, 8-day retention),
  per-event ingestion math (200 B × 10 × 1000 = 60 MB/month),
  roll-over behavior (hard drop on 413/429), and the
  `timestamp` → `clientTime` rename rationale
- The file references the CONTEXT, RESEARCH, and DISCUSSION
  log paths for the full design trail
- `git status` shows the new file in the untracked / added list
</verify>
<done>[ ]</done>
</task>

<task id="04-02-03">
<name>End-to-end verification: all plan 04-01 + 04-02 must-haves pass, full test suite green, manual demo path</name>
<files>
- (no new files; this is the integration verification step)
- .planning/phases/04-observability-product-selection/04-02-plan-SUMMARY.md
</files>
<action>
1. Write the SUMMARY at
   `.planning/phases/04-observability-product-selection/04-02-plan-SUMMARY.md`.
   Use the same shape as the Phase 3 SUMMARYs
   (`03-03-plan-SUMMARY.md`): 1-2 paragraphs on what shipped
   (the OBSERVABILITY.md sub-section, the PHASE-4-DECISION.md,
   the test re-verification), the 2-3 atomic commits (one for
   OBSERVABILITY.md, one for PHASE-4-DECISION.md, one for the
   SUMMARY), the deviations from the plan (e.g., the
   "OBSERVABILITY.md line count upper bound" change from 200 to
   220, if the executor bumped it), and the 2 plan-checker bugs
   fixed in the executor (if any). Include the final must-haves
   checklist (one bullet per must-have from the frontmatter,
   each with a tick).

2. Run the final verification (the 8 build/test commands from
   Phase 3 plan 03 task 03-06, abbreviated):

   - `go build ./...` — exits 0
   - `go vet ./...` — exits 0
   - `go test ./internal/telemetry/... -count=1` — exits 0
   - `go test ./cmd/... -count=1` — exits 0
   - `go test ./... -count=1` — exits 0 (no regression on the
     200+ existing tests + plan 04-01 tests)
   - `git status` — clean working tree except for the new
     files added in plans 04-01 and 04-02
   - `git diff --stat` — show the line counts of all
     modifications; assert no file outside the `files_modified`
     list of plans 04-01 and 04-02 was touched

3. The e2e acceptance for this plan is the
   `TestNewRelicRecorderContractEnforced` test from plan 04-01
   (already shipped) — re-run it to confirm the recorder works
   end-to-end through a fake httptest server with no real
   backend contact:

   - `go test ./internal/telemetry/... -count=1 -run TestNewRelicRecorderContractEnforced`
     exits 0
   - The test makes no real network calls (it uses
     `httptest.NewServer`, which binds to `127.0.0.1` on a
     random port)
   - The 5 CONTEXT assertions pass: URL path, X-Insert-Key
     header, body is array of length 1, eventType, 7 schema
     fields (with `timestamp` renamed to `clientTime`)

4. The OBSERVABILITY.md "How to enable / disable" section
   should now reference the new "Backend: New Relic"
   sub-section. The Phase 3
   `TestOBSERVABILITYHasAllSevenSections` test still passes
   (the 7 top-level sections are unchanged; the new content
   is a sub-section, not a top-level section).

5. Manual demo path (optional, for the executor's confidence
   — not enforced by any test):
   - Build: `cd packages/cli && go build -o /tmp/skill-organizer .`
   - Run: `SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID=12345 SKILL_ORGANIZER_NEWRELIC_INSERT_KEY=test-key /tmp/skill-organizer telemetry status`
   - Expected output: 8 lines including `Recorder: NewRelicRecorder`,
     `Account ID: 1234...` (no, 1234 from 12345), `Insert key: present`
   - This proves the env vars are read and the status output
     reflects the New Relic configuration end-to-end.

6. No code changes in this plan; the only file changes are
   `OBSERVABILITY.md`, `.planning/PHASE-4-DECISION.md`, and the
   new SUMMARY.
</action>
<verify>
- All 5 build/test commands (build, vet, telemetry, cmd, all)
  exit 0 with `-count=1` (no test cache)
- `git diff --stat` shows modifications only in:
  - `OBSERVABILITY.md` (the new "Backend: New Relic" sub-section
    + the "How to enable / disable" one-line reference)
  - `.planning/PHASE-4-DECISION.md` (the new file)
  - `04-01-plan-newrelic-recorder-and-factory.md` (the plan
    itself, from plan 04-01)
  - `04-02-plan-observability-decision-and-docs.md` (the plan
    itself, from plan 04-02)
  - `04-01-plan-SUMMARY.md` (from plan 04-01)
  - `04-02-plan-SUMMARY.md` (this plan's new SUMMARY)
  - The plan 04-01 code changes (recorder.go, recorder_test.go,
    root.go, telemetry.go, telemetry_test.go)
  No other files modified
- The diff is reviewable in <500 lines for OBSERVABILITY.md +
  PHASE-4-DECISION.md + SUMMARY (the rest is plan 04-01's diff)
- `wc -l OBSERVABILITY.md` is between 150 and 220
- `wc -l .planning/PHASE-4-DECISION.md` is between 30 and 100
- `wc -l .planning/phases/04-observability-product-selection/04-02-plan-SUMMARY.md`
  is between 30 and 100
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` succeeds
- [ ] `go test ./internal/telemetry/... -count=1` passes
- [ ] `go test ./cmd/... -count=1` passes
- [ ] `go test ./... -count=1` passes (no regression on the 200+ existing tests + plan 04-01 tests)
- [ ] `OBSERVABILITY.md` has the new `### Backend: New Relic` sub-section under `## Endpoint configuration`
- [ ] `OBSERVABILITY.md` "Backend: New Relic" sub-section documents the 6 required points (env-var setup, default URL, eventType, clientTime rename with rationale, hard-drop on 413/429, EU data center variant)
- [ ] `OBSERVABILITY.md` "How to enable / disable" section references the new "Backend: New Relic" sub-section
- [ ] `OBSERVABILITY.md` is between 150 and 220 lines
- [ ] The 7 top-level section headers from Phase 3 are all present (Phase 3's `TestOBSERVABILITYHasAllSevenSections` still passes)
- [ ] `.planning/PHASE-4-DECISION.md` exists at the repo root
- [ ] `.planning/PHASE-4-DECISION.md` is between 30 and 100 lines
- [ ] `.planning/PHASE-4-DECISION.md` contains all 6 required sections (chosen product, why, free-tier limits, ingestion math, roll-over, rename rationale)
- [ ] `TestNewRelicRecorderContractEnforced` (from plan 04-01) passes with `-count=1` (e2e acceptance: the recorder works through a fake httptest server with no real backend contact)
- [ ] `04-02-plan-SUMMARY.md` exists and is 30-100 lines
- [ ] `git status` shows only the expected file modifications
- [ ] No code changes in this plan (the only file changes are docs and the new SUMMARY)

## Rollback Guide

If this plan fails:

1. Revert the doc changes:
   ```
   git checkout -- OBSERVABILITY.md
   rm -f .planning/PHASE-4-DECISION.md
   rm -f .planning/phases/04-observability-product-selection/04-02-plan-SUMMARY.md
   ```
2. Verify: `go build ./...` and `go test ./...` pass on the
   reverted state (the OBSERVABILITY.md changes are pure
   additions; reverting restores the Phase 3 version. The
   PHASE-4-DECISION.md is a new file; removing it does not
   affect the build).
3. Retry with smaller scope:
   - First, ship OBSERVABILITY.md "Backend: New Relic" sub-section
     only (task 04-02-01). The doc is reviewable on its own.
   - Then add PHASE-4-DECISION.md (task 04-02-02). The audit
     record is independent of the doc.
   - Then run the final verification (task 04-02-03). The
     e2e acceptance is the `TestNewRelicRecorderContractEnforced`
     test from plan 04-01; no new test code is added in this plan.

The 3-task split matches the natural fault lines: (1) user-facing
docs (OBSERVABILITY.md), (2) audit record (PHASE-4-DECISION.md),
(3) closeout (SUMMARY + verification). The split is also the
natural commit boundaries (one task = one commit, per the
project's `docs:` convention).

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | The OBSERVABILITY.md "Backend: New Relic" sub-section is added as a new top-level section (e.g., `## Backend: New Relic`) instead of a sub-section (`### Backend: New Relic` under `## Endpoint configuration`). Phase 3's `TestOBSERVABILITYHasAllSevenSections` fails. | Low | Medium | The action text in task 04-02-01 explicitly mandates the H3 sub-section structure: "is a sub-section of 'Endpoint configuration' (H3 within an H1, or a higher-level H2/H3 structure)". The Phase 3 test asserts the 7 H2 section headers are present, and the new H3 doesn't add or remove any. |
| 2 | The PHASE-4-DECISION.md file references wrong paths or wrong numbers (e.g., the ingestion math is 200 B × 10 × 1000 = 2 MB/day, not 200 MB/day). A future planner reading the audit record gets the wrong scale estimate. | Low | High | The action text in task 04-02-02 explicitly states the math step-by-step: 200 B × 10 × 1000 = 2 MB/day, × 30 = 60 MB/month. The 100 GB cap = 100,000 MB. Headroom = 60 / 100,000 = 0.06%. The math is reproducible by hand from the numbers in the file. |
| 3 | The OBSERVABILITY.md "How to enable / disable" section's new reference to the "Backend: New Relic" sub-section uses a broken anchor link (e.g., `#backend-new-relic` vs `#backend--new-relic` for H3). A user clicking the link gets a 404. | Low | Low | The action text uses GitHub's auto-generated anchor: `(#backend-new-relic)`. GitHub's anchor generator strips `###` and replaces spaces with `-`. The actual link should be `(#backend-new-relic)` — verify by rendering the doc in GitHub's preview (or any markdown viewer that auto-generates anchors) before committing. |
| 4 | A future PR renames the `### Backend: New Relic` sub-section (e.g., to `### New Relic backend`) but the OBSERVABILITY.md "How to enable / disable" section still references the old name. The link is broken. | Low | Low | The PHASE-4-DECISION.md file references the sub-section by topic, not by exact name ("the OBSERVABILITY.md 'Backend: New Relic' sub-section"). A future PR that renames the sub-section is responsible for updating the link in the "How to enable / disable" section. The threat is acknowledged as accepted; the cost of a broken link is low (one click) and the cost of preventing it (a test that asserts the link exists) is high (test brittleness to doc rewording). |
| 5 | The free-tier limits documented in PHASE-4-DECISION.md drift from New Relic's actual pricing (e.g., New Relic reduces the free tier from 100 GB to 50 GB). The audit record is stale. | Medium | Medium | The PHASE-4-DECISION.md file includes a "Decided: 2026-06-12" date stamp. A future planning cycle that revisits the backend choice reads the date, checks New Relic's current pricing, and updates the file if needed. The cost of the drift is one periodic review (quarterly or yearly), not an automated test. |
| 6 | The `timestamp` → `clientTime` rename rationale in PHASE-4-DECISION.md is wrong or misleading (e.g., the rationale says "New Relic rejects the field" instead of "New Relic drops the field silently"). A future reader is confused. | Low | Medium | The action text in task 04-02-02 cites the New Relic reserved-words doc explicitly ("The New Relic Insights Events API reserves the `timestamp` attribute for Unix-epoch integers. Sending an RFC3339 string in the `timestamp` field is silently dropped at ingest (no error, no warning — the field is just absent from the resulting NRDB event)"). The wording matches the official docs (verified in `04-RESEARCH.md` NP1). |
| 7 | The e2e test from plan 04-01 (`TestNewRelicRecorderContractEnforced`) passes once but flakes on a second run (e.g., due to a port collision in `httptest.NewServer`). The Phase 4 acceptance is unclear. | Low | Low | `httptest.NewServer` binds to `127.0.0.1` on a random port (the OS picks a free port). Port collisions are impossible. The test is deterministic; the only flakiness risk is a network timeout if the test server is slow to respond (5-second timeout, well above the test's actual runtime of <100ms). |
| 8 | The OBSERVABILITY.md doc grows beyond 220 lines in a future PR, violating the "short and direct" style. The "Backend: New Relic" sub-section becomes a dumping ground. | Low | Low | The 220-line upper bound is a guideline, not a hard limit. The Phase 3 200-line bound is relaxed to 220 in this plan. A future PR that grows the doc further is responsible for splitting it into a separate file (e.g., `OBSERVABILITY-NEWRELIC.md`) and linking from OBSERVABILITY.md. |

## Commit Message

```
docs(observability): ship New Relic backend docs and decision record

- Add "Backend: New Relic" sub-section to OBSERVABILITY.md under
  the existing "Endpoint configuration" section (50-60 line
  addition: env-var setup, default URL, eventType value, the
  clientTime rename with rationale, hard-drop on 413/429, EU
  data center variant)
- Update "How to enable / disable" section to reference the new
  sub-section (2-line addition; preserves the 7-section structure
  from Phase 3)
- Create .planning/PHASE-4-DECISION.md (the human-audit record of
  the product selection: chosen product = New Relic Insights
  Events API, why, free-tier limits (100 GB/month, 1 user,
  8-day retention), per-event ingestion math (60 MB/month at
  projected scale), roll-over behavior (hard drop on 413/429),
  the timestamp → clientTime rename rationale)
- All 200+ existing tests pass; all plan 04-01 tests pass; no
  regression
- Re-verifies the e2e acceptance gate (TestNewRelicRecorder
  ContractEnforced from plan 04-01) with -count=1
- Closes REQ-9 acceptance: a user can run `telemetry status` and
  see the New Relic backend configured; the OBSERVABILITY.md doc
  has a complete user-facing setup guide; the planning trail is
  preserved in PHASE-4-DECISION.md
```
