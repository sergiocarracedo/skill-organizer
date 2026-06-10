# REQUIREMENTS.md

> 3 testable requirements for the next version of `skill-organizer`.
> Traced back to `.planning/PROJECT.md`. Each requirement is sized
> so that "done" is observable: a passing test, a documented command,
> a visible behavior, or a measurable metric.

---

## REQ-3 — Overlap evaluation

Detect when two skills cover the same trigger conditions and surface
the conflict.

- **Acceptance:** `skill-organizer check-overlap` (or equivalent)
  outputs a structured list of overlapping skill pairs with the
  conflicting trigger conditions. Exit code is non-zero when
  overlaps are found unless `--allow-overlap` is passed.
- **Tests:** Unit tests with curated overlapping and non-overlapping
  fixtures; CLI integration test.
- **Source:** `PROJECT.md` → "Why it exists" → "Overlap evaluation".

## REQ-4 — Skill security check

A command to evaluate each skill for security risks. Reuses the
agent-selection machinery from REQ-3 (extracted to a shared helper).
Local-first in v1, opt-in.

- **Acceptance:**
  - A new `skill-organizer check-security` command (and a hook that
    prompts to run it on add) is implemented.
  - Agent selection flow:
    1. Detect installed agentic tools (reuse REQ-3's discovery code,
       refactored into a shared helper).
    2. If a tool was chosen before (cached in user config), use it.
    3. If none detected, return the analysis prompt to stdout so
       the user can run it manually.
    4. Otherwise, prompt the user to pick one.
  - The chosen agent analyzes the skill, including **all files**, not
    only `SKILL.md`. Minimum checks:
    - No obfuscated text or code.
    - Presence of binary files → flagged as **not evaluable** (do
      not execute them).
    - Dangerous instructions: exfiltration of environment
      variables, passwords, API keys, secrets; or download-code
      patterns that could be payload-injection vectors.
  - Output: a structured report with a per-skill **risk score
    0-100** (100 = max risk). Above a configurable threshold,
    prompt the user to disable the skill (default = yes).
  - The risk score is stored in `skill-organizer`'s metadata.
    Enabling a high-risk skill again prompts for confirmation and
    shows the reason it is not recommended.
- **Tests:** Unit tests for the agent-selection helper
  (no-network mode returns the prompt); fixtures for the report
  schema; integration test that stores the score and re-prompts
  on enable.
- **Open:** the exact wording of the analysis prompt — to be
  refined after a short discovery pass, not blocked.
- **Source:** `PROJECT.md` → "Next big bet" + user spec on
  2026-06-10.

## REQ-8 — Observability (opt-in)

Anonymous usage telemetry, opt-in, disabled by default, to measure
adoption before defining a success metric.

- **Acceptance:** A first-run prompt (skippable) asks whether to
  enable telemetry. When enabled, a small, anonymous event stream
  records command invocations (no args, no paths, no PII). When
  disabled, zero network egress is emitted by the telemetry path.
  The schema and endpoint are documented in `OBSERVABILITY.md`
  (to be created with this requirement).
- **Tests:** Unit test asserts zero network calls when disabled;
  integration test asserts the documented schema is emitted when
  enabled.
- **Source:** `PROJECT.md` → "Top risk" + "Success metric".

---

## Dropped from this version (recorded for traceability)

The following were considered in PROJECT.md and REQUIREMENTS.md
drafts but are explicitly **not** in the next version. They are
preserved here so the next planning cycle can pick them up cleanly.

- **REQ-1 Folder organization** — group skills by topic/project.
- **REQ-2 Disable / enable** — flip a flag, keep files.
- **REQ-5 AI-tool matrix** — smoke tests per Claude Code / Codex /
  OpenCode / Cursor / Antigravity.
- **REQ-6 Offline-first CLI** — every command works without
  network except install and remote security lookup.
- **REQ-7 Single-binary distribution** — CGO-disabled, scratch
  container smoke.
- **REQ-9 Web app = docs / marketing** — Astro site stays as CLI's
  landing + docs; no skill content.

## Out of scope (carried from PROJECT.md)

- Registry / marketplace / publishing (anti-vision).
- Multi-user / team workspaces (anti-vision).
- Web-app skill catalog, browser, renderer, editor.
- Re-enabling the Go self-update path (blocked on security review
  per `CONCERNS.md`).
- Heuristic-weighted or remote-reputation security check
  (REQ-4 ships static-only in v1; deeper modes are a follow-up).
- Defining a hard success metric (gated on REQ-8 observability
  data).
