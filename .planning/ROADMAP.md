# ROADMAP.md

> 3 phases. Each phase is a vertical slice that ships a demoable
> behavior end-to-end. Phases are ordered so the next big bet
> (skill security check, REQ-4) ships first and unblocks the rest.

---

## Phase 1 — Skill security check (REQ-4)

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

## Phase 2 — Overlap refactor (REQ-3)

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

## Phase 3 — Observability (REQ-8)

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
