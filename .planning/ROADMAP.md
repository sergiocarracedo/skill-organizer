# ROADMAP.md

> 3 phases. Each phase is a vertical slice that ships a demoable
> behavior end-to-end. Phases are ordered so the next big bet
> (skill security check, REQ-4) ships first and unblocks the rest.

---

## Phase 1 — Skill security check (REQ-4) ✓ Complete

**Goal:** A user can run `skill-organizer check-security` and get a
per-skill risk score; high-risk skills prompt to be disabled; the
score is persisted and gates re-enable.

**Why first:** This is the next-big-bet feature. Shipping it early
exposes the agent-selection helper, which Phase 2 (overlap) will
re-use — so we refactor once, in P1, and P2 inherits the
abstraction.

**Scope:**

- Refactor: extract the agent-selection logic from the existing
  `skill-overlap` command into a shared internal helper (no
  behavior change, just moves code into a common file).
- Implement `skill-organizer check-security`:
  - Agent-selection flow (cached tool → detect installed → prompt
    → fall back to printing the prompt for manual use).
  - All-files analysis (not just `SKILL.md`).
  - Minimum checks: obfuscated text/code, binary files flagged as
    un-evaluable, dangerous instructions (env exfiltration, secret
    reads, payload-injection-style downloads).
  - Structured report with **0-100 risk score**.
  - If score > threshold, prompt to disable (default = yes).
  - Persist score in skill metadata; re-prompt on enable.
- Hook into `skill add`: after a successful install, prompt to run
  `check-security` for the new skill.
- Discovery pass: refine the analysis prompt (user-flagged as TBD).
  Treat as a small, focused spike before locking the prompt.

**Acceptance:**

- `skill-organizer check-security` exits 0 on a known-safe skill
  and non-zero (or shows warning) on a known-bad fixture.
- The score is stored in the manifest and survives a reload.
- Enabling a high-risk skill shows the reason and asks for
  confirmation.
- The shared helper has at least one test that exercises cache /
  detect / prompt / fall-back-to-print flows.

**Covers:** REQ-4.

---

## Phase 2 — Overlap refactor (REQ-3) ✓ Complete

**Goal:** `skill-organizer check-overlap` (or whatever it is called
now) uses the same shared agent-selection helper that P1 extracted,
and surfaces conflict pairs with the offending trigger conditions.

**Why now:** The helper from P1 already exists. P2 is a focused
refactor + the actual conflict-detection rule.

**Scope:**

- Refactor: switch overlap's agent selection to the shared helper
  (no behavior change expected).
- Implement the overlap rule: pairwise compare skills' declared
  triggers / descriptions, output structured conflict pairs.
- Exit non-zero on overlap unless `--allow-overlap` is passed.
- Tests: curated fixtures with overlapping and non-overlapping
  skills.

**Acceptance:**

- The shared helper has exactly one call site per command
  (`check-overlap`, `check-security`).
- `check-overlap` returns the right conflict list on fixtures.
- The existing 27 `*_test.go` files still pass.

**Covers:** REQ-3.

---

## Phase 3 — Observability (REQ-8) ✓ Complete

**Goal:** Opt-in, anonymous telemetry that records command
invocations without args / paths / PII. Disabled by default.
Documented schema and endpoint.

**Why last:** The success metric depends on this data, but we want
to define the metric **after** we have at least a few weeks of
behavior data from the security check (P1) and the overlap refactor
(P2). Shipping it last means we're not measuring in a vacuum.

**Scope:**

- First-run prompt: ask whether to enable telemetry. Skippable.
  Default = off. Sticky in user config.
- A small event recorder: command name (e.g. `check-security`),
  exit status, anonymized install/host ID (random, rotatable),
  timestamp. No args. No paths. No PII.
- Endpoint: configurable, default to a no-op sink for
  offline-first compliance. When telemetry is disabled, **zero**
  network egress on the telemetry path.
- `OBSERVABILITY.md`: schema, endpoint, opt-out procedure, data
  retention.
- Tests: unit test asserts zero network calls when disabled;
  integration test asserts the documented schema on enable.

**Acceptance:**

- Default install emits no telemetry.
- First-run opt-in flow is documented and works.
- Schema doc matches the emitted payload byte-for-byte.
- After 30 days of data, we revisit the "Success metric" section
  of `PROJECT.md` and pick a concrete number.

**Covers:** REQ-8.

---

## Coverage matrix

| Requirement                       | Phase |
| --------------------------------- | ----- |
| REQ-3 Overlap evaluation          | P2    |
| REQ-4 Skill security check        | P1    |
| REQ-8 Observability (opt-in)      | P3    |
| REQ-9 Telemetry backend selection  | P4    |

## Cross-cutting

- **Top risk (PROJECT.md):** low ROI / no users. We mitigate by
  shipping P3 (observability) as the **last** phase, then using
  the data to set a real success metric. Until then, success =
  "you use it daily and it stays out of your way."
- **Open questions** in PROJECT.md stay open until their owning
  phase picks them up (security-prompt wording → P1, observability
  surface → P3, success metric → after P3, self-update re-enable →
  not in this roadmap).
- **Anti-vision guards:** the roadmap adds no registry, no runner,
  no multi-user, no skill-content web app. The PR template should
  reference PROJECT.md and REQUIREMENTS.md so reviewers can spot
  drift early.
- **Dropped requirements** (REQ-1, REQ-2, REQ-5, REQ-6, REQ-7,
  REQ-9) are listed in REQUIREMENTS.md under "Dropped from this
  version" so the next cycle can pick them up cleanly.

---

## Phase 4 — Telemetry backend selection (REQ-9)

**Goal:** A user can run `skill-organizer telemetry status` and see
a real, free-of-charge telemetry backend that accepts the events
the CLI emits (7-field snake_case JSON, one POST per command).

**Status:** [ ] Not started
**Depends on:** Phase 3 (Observability) — the recorder, buffer,
endpoint config, and the byte-for-byte schema test already exist
in `internal/telemetry/` and `OBSERVABILITY.md`. Phase 4 picks a
real product (or self-hosted sink) to point the recorder at.

**Why now:** Phase 3 ships the **emit side** (event schema, buffer
on disk, opt-in prompt, no-op when disabled) and the **schema
doc**. What it does not ship is a real **receive side** — the
endpoint currently defaults to a no-op sink. We need a real
backend before we can ship the v0.x "first-run opt-in, real
data flowing" story end-to-end.

### Scope (proposed)

- **Research and select a free / open-source telemetry backend.**
  Candidates to evaluate (each with a free tier or self-hosted
  option, all accepting the project's 7-field JSON schema):
  - **Managed free tiers:** New Relic (100 GB/month free),
    Grafana Cloud (10k metrics free), Sentry (5k events/month
    free), BetterStack, Logtail, Highlight.io.
  - **Self-hosted / open source:** Grafana Loki + Prometheus +
    Tempo (logs/metrics/traces), OpenObserve (single binary,
    MIT), SigNoz (Apache-2.0, OTLP), Quickwit (logs search,
    AGPL-3), HyperDX (MIT, ClickHouse-based).
  - **Project-agnostic simple sinks:** a tiny self-hosted Go
    receiver (httptest-style), a JSONL-on-disk + `tail -f`
    workflow, or a Cloudflare Worker.
- **Document the decision** in a new file
  `.planning/PHASE-4-DECISION.md`: chosen product, why, free-tier
  limits, how the project points the recorder at it (env var or
  YAML endpoint), and the per-event ingestion math (events/day ×
  bytes/event × free-tier GB/month).
- **No code change to the recorder** — `HTTPRecorder` already
  POSTs JSON to whatever endpoint the user configures. The
  work is configuration, documentation, and an end-to-end
  smoke test against the chosen backend.

### Acceptance

- `OBSERVABILITY.md` is updated with a "How to point at the
  chosen backend" section (env var, YAML, free-tier quota).
- The recorder's existing byte-for-byte schema test continues to
  pass — the schema is unchanged.
- A short smoke test (Go test or shell script) POSTs a single
  fake event at the configured endpoint and asserts the chosen
  backend accepted it (HTTP 2xx or a backend-specific check).
- A section in `OBSERVABILITY.md` lists the chosen product, the
  free-tier limits, the per-event payload size, and the roll-over
  behavior once the free tier is hit.

### Out of scope (this phase)

- **Server-side retention policy** — defined by the chosen
  product, not by us.
- **Per-tool breakdown dashboards** — comes after we have data.
- **Alerting rules** — not a v0.x concern; the product gives us
  dashboards, we don't ship alerts.
- **Migration to a paid tier** — if the free tier proves too
  small, that's a future phase with its own decision.

**Status:** ✓ Complete (2026-06-12). See `.planning/PHASE-4-DECISION.md` for the decision audit record.
