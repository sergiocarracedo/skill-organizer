# Phase 4 — Telemetry backend decision (REQ-9)

> Audit record of the v0.x telemetry backend selection. Read this
> file before considering a backend change in a future planning
> cycle. NOT referenced by downstream agents — for human audit only.
>
> Decided: 2026-06-12 · Phase: 04-observability-product-selection
> Source discussions: `04-CONTEXT.md`, `04-RESEARCH.md`, `04-DISCUSSION-LOG.md`
> (all in `.planning/phases/04-observability-product-selection/`)

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

1. Logs a one-line warning via pterm (light-magenta, per the
   project's color rules).
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
