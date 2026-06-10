# PROJECT.md

> Source of truth for what skill-organizer is, who it serves, and what we are
> explicitly not building. The roadmap in `ROADMAP.md` must be traceable back
> to this file.

---

## What it is

`skill-organizer` is a Go CLI that installs, organizes, audits, and updates
Agent Skills across AI tools (Claude Code, Codex, OpenCode, Cursor,
Antigravity). It wraps `npx skills` for the install protocol, owns the
local manifest as the single source of truth, and routes files into the
right tool-specific directories.

The web app (`packages/web/`) is an Astro site on GitHub Pages. It is the
**landing page and documentation site for the CLI tool** — it explains
the CLI, links to install instructions, and renders docs. It is **not**
about skills themselves: no skill catalog, no skill browsing, no skill
content. The CLI is the product; the web app is the surface that helps
people find and learn the CLI.

## Audience

Open-source community — anyone using AI tools that consume the Agent Skills
spec who wants a single manifest, folder organization, and a security
audit before installing community skills.

## Why it exists (vs. running `npx skills` directly)

`npx skills` is an imperative, single-shot install. `skill-organizer` adds
the things `npx skills` does not do:

- **Folder organization** — group skills by topic/project, not by tool.
- **Disable / enable** — turn skills off without removing them.
- **Overlap evaluation** — detect when two skills cover the same trigger
  conditions and surface the conflict.
- **Skill security check** (next major feature) — static + heuristic
  analysis of skill content before installation.
- **Service / daemon mode** (already imported: `kardianos/service`) —
  background sync and config watching, deferred to a later phase.

## Hard constraints

- **AI-tool independence.** Must work with any AI tool that consumes the
  Agent Skills spec. Do not bake in Claude-Code-only assumptions.
- **Single-binary distribution.** The CLI must run with no Python, no
  Node, no system packages.
- **Offline-first.** The CLI must work fully without network; only
  optional features (install, security lookup) may require it.

## Anti-vision (explicitly NOT building)

- **Not a skill marketplace or registry.** `skills.sh` and `npx skills`
  own discovery and publishing. We manage local state, not publishing.
- **Not a runner.** Claude Code / Codex / OpenCode execute skills; we
  organize them, they run them.
- **Not a multi-user / team platform.** No auth, no shared workspaces,
  no permissions. Single-user, local-first, project-scoped.
- **Not a skills web app.** The web app is a landing page and docs site
  for the CLI tool — it does not host, browse, render, or edit skill
  content. The CLI is the product; the web app is its marketing and
  documentation surface.

## Scope cuts (decisions from Round 4)

- **Keep the web app** as a landing page + documentation site for the
  CLI (Astro on GitHub Pages). It is in scope for the next version
  **as docs/marketing**, not as a product feature.
- **Keep** the full AI-tool matrix (Claude Code, Codex, OpenCode, Cursor,
  Antigravity) — that's the "AI-tool independence" hard constraint.
- **Keep** the alpha prerelease branch (for now).
- **Keep** the Go self-update path in code (intentionally disabled in
  release config — revisit only after a security review of the extractors
  flagged in `CONCERNS.md`).

## Top risk

**Low ROI / no users.** Maintenance effort must not exceed actual
adoption. Every new feature needs to justify itself against this risk.
We will add lightweight observability (anonymous usage telemetry, opt-in
and disabled by default) to measure adoption before defining a success
metric.

## Success metric

**To be defined.** We will ship a minimal, opt-in observability layer
first, observe what people actually do, then set a concrete success
metric. Until that data exists, success = "you use it daily and it stays
out of your way."

## Next big bet

**Skill security check.** The killer feature that justifies a v0.x → v1
jump. Initial scope: local static pattern matching (deny-list of
dangerous patterns: `eval`, `exec`, `curl|sh`, secret-looking strings)
with a per-skill risk score. Heuristic weighting and remote reputation
lookups are a follow-up phase, gated on a milestone the user will
define.

## Lean on, don't rebuild

- `npx skills` — install protocol and registry calls.
- Cobra + pterm — CLI ergonomics and progress UI (already in use).
- Agent-tool CLIs (claude, codex, opencode, cursor, antigravity) —
  invoke them, don't reverse-engineer their file layouts.
- `kardianos/service` — daemon mode (already imported).
- `release-please` + `GoReleaser` + npm trusted publishing — release
  pipeline (already wired).

## Out of scope (for the next version)

- Registry / publishing (conflicts with "not a marketplace" anti-vision).
- Multi-user / team workspaces.
- Any skill content in the web app (catalog, browser, renderer, editor) —
  the web app is docs/marketing only.

## Open questions (park, do not block)

- **Security check depth** — local static only, or static + remote
  reputation? Milestone TBD by user.
- **Observability surface** — what events, what schema, where to send.
- **Success metric** — to be set after observability ships.
- **Self-update re-enable** — blocked on security review of the
  extractors (see `CONCERNS.md`).
