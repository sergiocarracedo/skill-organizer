# AGENTS.md

> Local conventions and the planning context that any agent (human or
> AI) working in this repo must read first.

---

## Planning context

This repo is currently under learnship's `/new-project` workflow.
Before making non-trivial changes, read the planning docs in order:

1. `.planning/PROJECT.md` — vision, audience, hard constraints,
   anti-vision, scope cuts, top risk, next big bet. The source of
   truth for "what skill-organizer is and is not."
2. `.planning/REQUIREMENTS.md` — 3 testable requirements
   (REQ-3, REQ-4, REQ-8) for the next version, plus a "Dropped from
   this version" section for traceability.
3. `.planning/ROADMAP.md` — 3 phases:
   - **P1**: Skill security check (REQ-4) — includes extracting the
     agent-selection helper from the existing `skill-overlap` code.
   - **P2**: Overlap refactor (REQ-3) — reuses the P1 helper.
   - **P3**: Observability (REQ-8) — opt-in, anonymous, last.

Codebase maps live in `.planning/codebase/` (STACK, ARCHITECTURE,
CONVENTIONS, TESTING, CONCERNS, INTEGRATIONS, STRUCTURE) — read these
to understand the existing code before adding to it.

## Current Phase

**Milestone:** v0.x — Phase 1 ✓ complete → Phase 2 ✓ complete → next: Phase 3 (Observability, REQ-8)

**Phase:** 2 — Overlap refactor (REQ-3) ✓ complete
**Status:** verifying (14/14 must-haves passed)
**Last updated:** 2026-06-11

See `.planning/STATE.md` for the full state and `.planning/phases/02-overlap-refactor/` for the plan artifacts.

Context: `.planning/phases/02-overlap-refactor/02-CONTEXT.md`
Research: `.planning/phases/02-overlap-refactor/02-RESEARCH.md`

## Anti-vision guards

These came from PROJECT.md. **Do not propose, build, or accept PRs
that add them** without first updating PROJECT.md:

- A skill marketplace, registry, or publishing flow.
- A skill runner (Claude Code / Codex / OpenCode run skills; we
  organize them, they run them).
- A multi-user / team platform (auth, shared workspaces,
  permissions).
- A web app that hosts, browses, renders, or edits skill content.
  The web app is the CLI's landing page + documentation site only.

## Hard constraints

- **AI-tool independence.** The CLI must work with any AI tool that
  consumes the Agent Skills spec, not just Claude Code.
- **Single-binary distribution.** The CLI runs with no Python, no
  Node, no system packages.
- **Offline-first.** Every command works without network, except
  `install` and any remote-backed security lookup.

## Code conventions (read `.planning/codebase/CONVENTIONS.md` for full detail)

- Go: `camelCase` funcs, `PascalCase` types, `newXxxCommand()` cobra
  constructors, package-aliased imports (e.g. `configpkg`), `RunE:`,
  errors wrapped with `fmt.Errorf("...: %w", err)`, stdlib only (no
  testify, no omock).
- Web: oxlint + Prettier; `PascalCase.astro` components, kebab-case
  lib filenames.
- Conventional commits (learnship enforced): `feat:`, `fix:`,
  `docs:`, `refactor:`, `test:`, `chore:`.
- lefthook pre-commit + commitlint run locally; CI runs the same.

## Color rules (CLI output)

- **Yellow** is reserved for interactive key hints and navigation
  help text. Do not use yellow for progress labels or spinner status.
- **Magenta / cyan / light-magenta** are the preferred status /
  progress colors. Use the existing pterm helpers in
  `packages/cli/internal/ui/` rather than introducing new colors.
- **Green / red** are reserved for success / failure; reuse the
  existing helpers in the same package.

## Test conventions (read `.planning/codebase/TESTING.md` for full detail)

- Go: `*_test.go` co-located with the code under test. `TestXxxYyy`
  naming. `t.TempDir` / `t.Setenv` / `t.Cleanup` for fixtures. No
  testify, no omock — package-level function variables are swapped
  in `t.Cleanup` for stubbing. CLI has no coverage thresholds; web
  has 95% thresholds (Vitest + `@vitest/coverage-v8`).
- Run commands: `pnpm test` (web), `go test ./...` (CLI), or use
  the repo's existing pnpm scripts.

## Workflow pointers (learnship)

- **Plan first.** Use OpenSpec for spec-driven changes; align
  requirements with `.planning/REQUIREMENTS.md`.
- **Refactor before extending** when a feature would otherwise
  duplicate logic — that's why P1 extracts the agent-selection
  helper before adding `check-security`.
- **Ship vertical slices.** Each phase in `.planning/ROADMAP.md`
  must end with a demoable behavior, not a half-built module.
- **Conventional commits.** `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`. No emoji. No "wip".
- **Plan mode default.** When in doubt, switch to plan mode and
  ask before touching code.
