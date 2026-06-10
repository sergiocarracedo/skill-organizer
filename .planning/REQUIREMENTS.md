# REQUIREMENTS.md

> 9 testable requirements for the next version of `skill-organizer`.
> Traced back to `.planning/PROJECT.md`. Each requirement is sized so
> that "done" is observable: a passing test, a documented command, a
> visible behavior, or a measurable metric.

---

## REQ-1 — Folder organization

Group installed skills by topic/project, not by tool, in the manifest.

- **Acceptance:** The manifest supports a `groups` (or equivalent)
  field; `skill-organizer list --group X` returns the right set; skills
  in the same group are co-located on disk under a folder the user
  controls.
- **Tests:** Unit tests for the manifest read/write round-trip; CLI
  integration test that creates a group, adds a skill, and lists it.
- **Source:** `PROJECT.md` → "Why it exists" → "Folder organization".

## REQ-2 — Disable / enable

Turn skills off without removing them; the manifest records an enabled
flag.

- **Acceptance:** `skill-organizer disable <name>` keeps the skill
  files but flips the `enabled: false` flag in the manifest; the
  targeted tool directory no longer sees the skill. `enable` reverses.
  No skill data is destroyed.
- **Tests:** Round-trip test for enable/disable; integration test that
  verifies the tool dir is updated.
- **Source:** `PROJECT.md` → "Why it exists" → "Disable / enable".

## REQ-3 — Overlap evaluation

Detect when two skills cover the same trigger conditions and surface
the conflict.

- **Acceptance:** `skill-organizer check-overlap` (or equivalent)
  outputs a structured list of overlapping skill pairs with the
  conflicting trigger conditions. Exit code is non-zero when overlaps
  are found unless `--allow-overlap` is passed.
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
    3. If none detected, return the analysis prompt to stdout so the
       user can run it manually.
    4. Otherwise, prompt the user to pick one.
  - The chosen agent analyzes the skill, including **all files**, not
    only `SKILL.md`. Minimum checks:
    - No obfuscated text or code.
    - Presence of binary files → flagged as **not evaluable** (do not
      execute them).
    - Dangerous instructions: exfiltration of environment variables,
      passwords, API keys, secrets; or download-code patterns that
      could be payload-injection vectors.
  - Output: a structured report with a per-skill **risk score 0-100**
    (100 = max risk). Above a configurable threshold, prompt the user
    to disable the skill (default = yes).
  - The risk score is stored in `skill-organizer`'s metadata. Enabling
    a high-risk skill again prompts for confirmation and shows the
    reason it is not recommended.
- **Tests:** Unit tests for the agent-selection helper (no-network
  mode returns the prompt); fixtures for the report schema; integration
  test that stores the score and re-prompts on enable.
- **Open:** the exact wording of the analysis prompt — to be refined
  after a short discovery pass, not blocked.
- **Source:** `PROJECT.md` → "Next big bet" + user spec on 2026-06-10.

## REQ-5 — AI-tool matrix

All 5 supported AI tools continue to work: Claude Code, Codex,
OpenCode, Cursor, Antigravity.

- **Acceptance:** A smoke test exists for each tool that adds a skill
  via the CLI and verifies the file lands in the tool's expected
  directory. Adding a 6th tool is a single, well-scoped change in one
  place (no scattered edits).
- **Tests:** One smoke test per tool, gated on the tool's binary
  being on `PATH` (skip, not fail, when absent — `CONCERNS.md`
  flags the current N-serial `npx skills` check-updates as a related
  perf concern, but each smoke test is independent).
- **Source:** `PROJECT.md` → "Hard constraints" → "AI-tool
  independence".

## REQ-6 — Offline-first CLI

Every command works without network, except install and security
lookup.

- **Acceptance:** A network-down smoke test (e.g. `unshare -n` or
  equivalent) confirms that all commands except `install` and any
  remote-backed security lookup succeed and produce the same output
  as on network.
- **Tests:** `testdata/` fixture-driven commands run under
  `unshare -n`; install is the only documented network consumer.
- **Source:** `PROJECT.md` → "Hard constraints" → "Offline-first".

## REQ-7 — Single-binary distribution

The CLI runs with no Python, no Node, no system packages.

- **Acceptance:** `go build` produces a static binary (or
  cgo-disabled where possible); CI smoke-tests the binary on a
  clean `alpine` image with no runtime dependencies. The npm
  wrapper, Homebrew formula, and GitHub Release artifacts all
  resolve to the same binary.
- **Tests:** CI job that runs the built binary in a scratch-like
  container.
- **Source:** `PROJECT.md` → "Hard constraints" → "Single-binary
  distribution".

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

## REQ-9 — Web app = docs / marketing

The Astro site stays as the CLI's landing page and documentation.
No skill content, no new product features.

- **Acceptance:** The web app has a landing page that explains the
  CLI and links to install instructions, plus a docs section.
  CI for the web app builds the static site and deploys to GitHub
  Pages. No new feature work is scoped to the web app for this
  version.
- **Tests:** Existing Playwright smoke (`test/e2e/`) continues to
  pass; CI build of the Astro site is green.
- **Source:** `PROJECT.md` → "What it is" + "Scope cuts".

---

## Out of scope (for traceability)

These came up in PROJECT.md and are explicitly **not** requirements
for the next version:

- Registry / marketplace / publishing (anti-vision).
- Multi-user / team workspaces (anti-vision).
- Web-app skill catalog, browser, renderer, editor (REQ-9).
- Re-enabling the Go self-update path (blocked on security review
  per `CONCERNS.md`).
- Heuristic-weighted or remote-reputation security check
  (REQ-4 ships static-only in v1; deeper modes are a follow-up).
- Defining a hard success metric (gated on REQ-8 observability
  data).
