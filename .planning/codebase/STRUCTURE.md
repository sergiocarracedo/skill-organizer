# Structure

Directory layout reference for the `skill-organizer` monorepo at `/works/opensource/skill-organizer`. The repo follows the pnpm-workspace convention declared in `pnpm-workspace.yaml:1-3`:
```yaml
packages:
  - packages/web
  - packages/cli/packages/npm
```
The Go module under `packages/cli/` is independent of pnpm and is built with `go build ./...` from the root (`package.json:7-8`).

## Top-Level Layout

| Path | Purpose |
| --- | --- |
| `.agents/` | Agent skill inputs (opencode config etc.) — local-only, not committed application code. |
| `.github/workflows/` | Seven CI/CD workflows: `ci.yml`, `cli-e2e.yml`, `release.yml`, `release-please.yml`, `auto-merge-release-please.yml`, `publish-npm.yml`, `deploy-web.yml`. |
| `.opencode/` | Local opencode CLI skill definitions; not part of the shipped product. |
| `.planning/codebase/` | The learnship codebase maps (this directory) — `ARCHITECTURE.md`, `STRUCTURE.md`, plus prior `STACK.md`/`CONVENTIONS.md`/`TESTING.md`. |
| `assets/` | Brand assets used by both packages: `logo.png`, `logo_color.png`, `logo_color2.png`, `logo_color_simplified.png`, `logo_color_simplified_for_ascii.png`, `skill-creator.png`, `ascii-text-art.txt`, and `demo.mp4`. |
| `docs/` | Repo-level Markdown: `releasing.md` (release playbook), `homebrew-formula-template.md`, `asciinema-demo.md`, plus two `.cast` files for the README demo. |
| `node_modules/` | pnpm-managed dependencies (gitignored). |
| `openspec/` | Spec-driven change artifacts. `config.yaml:1` declares `schema: spec-driven`. The only in-flight change is `openspec/changes/build-skill-organizer-cli/` with `proposal.md`, `design.md`, `tasks.md`, and four capability specs under `specs/` (`location-configuration`, `managed-skill-sync`, `skill-status-and-control`, `watched-location-registry`). `openspec/archive/` holds completed changes. |
| `packages/cli/` | The Go CLI module (`module github.com/sergiocarracedo/skill-organizer/cli`). Source under `cmd/`, `internal/`, `main.go`, `e2e_test.go`, plus a checked-in prebuilt `skill-organizer` binary, `.goreleaser.yaml`, `lefthook.yml`, `go.mod`, `go.sum`. |
| `packages/cli/packages/npm/` | The npm-published shim that downloads the Go binary. This is the second pnpm workspace member. |
| `packages/web/` | Astro 6 marketing + docs site. See **packages/web Structure** below. |
| `scripts/` | Local release helpers: `release-alpha.sh`, `release-beta.sh`, `release-stable.sh`, `release-common.sh`, `asciinema-install-demo.sh`, `asciinema-install-move-demo.sh`. |
| `.gitignore` | Standard Node + Go ignores. |
| `AGENTS.md` | Local agent rules (yellow-reserved, progress-color guidance). |
| `AGENTS_README.md` | How to onboard an agent to this repo. |
| `CHANGELOG.md` | release-please-generated changelog. |
| `README.md` | Top-level project README. |
| `UBIQUITOUS_LANGUAGE.md` | The canonical glossary (Source Tree, Target Folder, Project Config, Watch Registry, etc.). |
| `VERSION` | Single source of truth for the current release (`1.0.2`). Read by the web build (`packages/web/astro.config.mjs:8`) and baked into the CLI via GoReleaser `-ldflags`. |
| `lefthook.yml` | Root pre-commit hook: only runs `pnpm run test:cli:e2e` when CLI-related files are staged (`lefthook.yml:1-10`). |
| `logo_color2.png` | The brand mark used in the README and on the site. |
| `package.json` | Root pnpm workspace. Scripts: `build`, `build:cli`, `build:web`, `lint` (= `lint:web`), `type-check`, `test`, `test:cli`, `test:cli:e2e`, `test:cli:e2e:real-npx`, `test:web`, `dev:web`, `hooks:install`. `packageManager: pnpm@10.18.3` (`package.json:4`). |
| `pnpm-lock.yaml` | pnpm lockfile (committed). |
| `pnpm-workspace.yaml` | Workspace declaration (the two members above). |
| `release-please-config.{alpha,beta,stable}.json` | Three release-please configs, one per track. |
| `release-please-manifest.json` | release-please's per-package version map. |
| `skills-lock.json` | Tracked import source for the `asciinema-recorder` skill used by the demo/e2e. |

## packages/cli Structure

```
packages/cli/
├── main.go                          # process entry; calls cmd.Execute() and exits 1 on error
├── go.mod                           # module github.com/sergiocarracedo/skill-organizer/cli, go 1.24.0
├── go.sum                           # Go module checksums
├── e2e_test.go                      # End-to-end tests that drive the compiled binary via pty
├── README.md                        # CLI-specific install + verify doc
├── .goreleaser.yaml                 # GoReleaser v2 config: archives, ldflags, homebrew tap
├── lefthook.yml                     # CLI-local pre-commit (not currently used; root owns the hook)
├── skill-organizer                  # Checked-in prebuilt Go binary (17 MB, gitignored in most setups)
│
├── cmd/                             # Cobra command tree — one file per command plus shared helpers
│   ├── root.go                      # Root command; PersistentPreRun header + maintenance; subcommand registration
│   ├── sync.go                      # `skill-organizer sync`
│   ├── status.go                    # `skill-organizer status`
│   ├── status_render.go             # Pretty-printer for status reports
│   ├── about.go                     # `skill-organizer about`
│   ├── completion.go                # `skill-organizer completion <shell>`
│   ├── onboard.go                   # `skill-organizer onboard` — guided first-time setup
│   ├── project.go                   # `project` group (`add`/`edit`/`remove`)
│   ├── add.go                       # `project add`
│   ├── edit.go                      # `project edit`
│   ├── remove.go                    # `project remove`
│   ├── skill.go                     # `skill` group (parent of skill subcommands)
│   ├── skill_add.go                 # `skill add <source>` (root alias `add`)
│   ├── skill_delete.go              # `skill delete <path-or-pattern>` (root alias `delete`)
│   ├── skill_check_updates.go       # `skill check-updates` (root alias `check-updates`)
│   ├── skill_try_find_metadata.go   # `skill try-find-metadata`
│   ├── skill_overlap.go             # `skill check-overlap` (agent-driven analysis)
│   ├── enable.go                    # `skill enable <path>` (root alias `enable`)
│   ├── disable.go                   # `skill disable <path>` (root alias `disable`)
│   ├── move_unmanaged.go            # `skill move-unmanaged`
│   ├── watched.go                   # `watched` group (`list`/`add`/`remove`)
│   ├── watch.go                     # `watch` — foreground fsnotify runner
│   ├── service.go                   # `service` group (install/start/stop/.../log-level)
│   ├── self_update.go               # `self-update` — wrapper around internal/selfupdate
│   ├── aliases.go                   # Root-level alias generator (e.g. `add` → `skill add`)
│   ├── context.go                   # loadResolvedLocation / loadResolvedProject — config resolution
│   ├── prompt.go                    # Interactive selectors, path autocompleter, confirm helpers
│   ├── editable_path_selector.go    # Keyboard-driven path picker (atomicgo/keyboard)
│   ├── logo.go                      # ASCII/ANSI logo, gradient sampling
│   ├── assets/                      # Logo PNG + ASCII text (go:embed)
│   ├── *test.go                     # Co-located tests for each command file
│
├── internal/                        # Domain packages (Go `internal/`, only importable by this module)
│   ├── config/                      # Project + app config: Location, AppConfig, registry, paths, cache
│   │   ├── config.go                #   Type definitions, normalization, validation
│   │   ├── registry.go              #   Load/Save app config + registry (the YAML file at ~/.config/skill-organizer/)
│   │   ├── discovery.go             #   Walk-up config discovery + home fallback
│   │   ├── path.go                  #   ResolvePath (handles `~` and env-var expansion)
│   │   ├── cache.go                 #   Update cache, updates state, SkillUpdateRecord
│   │   ├── *_test.go                #   Co-located tests
│   ├── skills/                      # Source tree + SKILL.md handling
│   │   ├── scanner.go               #   ScanSource, ResolveSourceSkill, FlattenName
│   │   ├── frontmatter.go           #   YAML frontmatter Document, ManagedMetadata, RewriteManagedFields*
│   │   ├── *_test.go
│   ├── sync/                        # The reconciler
│   │   ├── sync.go                  #   Run(location) → Result, reconcileTargetEntry
│   │   ├── manifest.go              #   Per-target .skill-organizer.manifest.yml
│   │   ├── *_test.go
│   ├── status/                      # Read-only inspector
│   │   ├── status.go                #   Build → Report, SkillState constants
│   │   ├── *_test.go
│   ├── agenttools/                  # Registry of supported agent CLIs (Claude/Codex/OpenCode/Cursor/Antigravity)
│   │   ├── agenttools.go
│   │   ├── *_test.go
│   ├── overlap/                     # LLM-driven overlap analysis
│   │   ├── overlap.go               #   BuildPrompt, BuildApplyPlanPrompt, Run, ParseReport
│   │   ├── process_unix.go          #   SIGINT/process-tree handling (build tag !windows)
│   │   ├── process_windows.go       #   Windows variant
│   │   ├── *_test.go
│   ├── remote/                      # `skills` CLI integration (the only outbound HTTP for skill import)
│   │   ├── skillscli.go             #   DetectSkillsCLI, Sandbox, NewSandbox, FetchSkillBundle, FindSkills, InstalledSkill, SkillSummary, SkillBundle
│   │   ├── *_test.go
│   ├── backup/                      # `.old/` retention
│   │   ├── backup.go                #   MoveSkill, PruneExpired, List, Root
│   │   ├── *_test.go
│   ├── mover/                       # Unmanaged target → source tree moves
│   │   ├── mover.go                 #   Plan, SetRelativeTarget, Apply
│   │   ├── *_test.go
│   ├── watch/                       # Foreground/background fsnotify runner
│   │   ├── watch.go                 #   Runner, Run, flush, reloadWatchedProjects, maybeCheckSkillUpdates
│   │   ├── *_test.go
│   ├── service/                     # Background-service wrapper around kardianos/service + Linux systemd-user
│   │   ├── service.go
│   │   ├── *_test.go
│   ├── selfupdate/                  # GitHub release check + install-method detection
│   │   ├── selfupdate.go
│   │   ├── *_test.go
│   ├── maintenance/                 # Periodic backup GC + skill-update reminders
│   │   ├── maintenance.go
│   │   ├── *_test.go
│   ├── logging/                     # Logger interface + 3 constructors
│   │   ├── logging.go
│   ├── versionfmt/                  # Display helpers for version strings/dates
│   │   ├── versionfmt.go
│
├── testdata/
│   └── fake-skills-cli.go           # A complete fake `skills` binary (Go source) used by the e2e tests
│
└── packages/npm/                    # The npm-published wrapper (pnpm workspace member)
    ├── package.json                 # `name: "skill-organizer"`, preferGlobal, bin → bin/skill-organizer.js
    ├── README.md                    # README shown on npmjs.com
    ├── bin/
    │   └── skill-organizer.js       # CommonJS shim that spawns the vendored Go binary
    └── scripts/
        └── postinstall.js           # Downloads the right archive from GitHub Releases, verifies SHA-256, extracts
```

There is no `pkg/` directory; all shared Go code is under `internal/` per Go convention.

## packages/web Structure

```
packages/web/
├── package.json                     # name: "web", type: "module", scripts: dev/build/preview/lint/format/test/test:e2e
├── astro.config.mjs                 # Astro 6 + MDX + astro-icon + Tailwind v4 via Vite plugin; reads ../VERSION
├── tsconfig.json                    # extends astro/tsconfigs/strict; path alias @/* → src/*
├── vitest.config.ts                 # Vitest with v8 coverage; thresholds 0.95 for lines/statements/functions/branches
├── playwright.config.ts             # Playwright config; webServer = `pnpm preview --host 127.0.0.1 --port 4321`
├── commitlint.config.js             # Conventional commits (commitlint reads this)
├── lefthook.yml                     # Pre-commit: oxlint + prettier + astro check + vitest + commitlint
├── DESIGN.md                        # Site design language reference
│
├── public/                          # Static assets served as-is at site root
│   ├── CNAME                        # Custom domain for GitHub Pages
│   ├── favicon.svg
│   ├── logo_color2.png
│   ├── hero/                        # Demo videos (demo.mp4, demo.webm)
│   └── og/                          # Open Graph card (skill-organizer-social.png + .svg)
│
├── src/
│   ├── env.d.ts                     # Astro/Vite env type augmentation
│   ├── content.config.ts            # Defines the `docs` Astro content collection (Zod schema)
│   ├── styles/
│   │   └── global.css               # Tailwind v4 entry, imported by SiteLayout
│   │
│   ├── pages/                       # File-based routes
│   │   ├── index.astro              # Home: composes 9 home sections from views/home/*
│   │   └── docs/
│   │       ├── index.astro          # Docs landing (guides + reference cards)
│   │       └── [section]/[slug].astro  # Dynamic MDX page; getStaticPaths over the docs collection
│   │
│   ├── views/                       # Page-level section composition
│   │   ├── home/
│   │   │   ├── hero/                # HeroSection, HeroWaves, HeroInstallSwitcher, HeroTransformPanel/
│   │   │   ├── features/            # FeaturesSection
│   │   │   ├── compatibility/       # CompatibilitySection (logo grid)
│   │   │   ├── advantages/          # AdvantagesSection
│   │   │   ├── terminal-demo/       # TerminalDemoSection + TerminalScene
│   │   │   ├── skill-updates/       # SkillUpdatesSection
│   │   │   ├── service/             # ServiceSection
│   │   │   ├── overlap/             # OverlapSection
│   │   │   ├── faq/                 # FaqSection
│   │   │   └── docs-cta/            # DocsCtaSection
│   │   └── docs/
│   │       └── components/          # DocsLayout, DocsSidebar, DocsToc
│   │
│   ├── components/                  # Reusable UI components
│   │   ├── Header.astro
│   │   ├── Footer.astro
│   │   ├── SectionHeader.astro
│   │   ├── SectionShell.astro
│   │   ├── TerminalFrame.astro
│   │   ├── CodePanel.astro
│   │   ├── CopyButton.astro
│   │   ├── AgentToolLogo.astro
│   │   ├── LogoMark.astro
│   │   ├── GitHubStarButton.astro
│   │   └── logos/                   # Per-tool brand marks: Antigravity, Apple, Claude, Codex, Copilot, Cursor, Linux, Opencode, Pii
│   │
│   ├── layouts/
│   │   └── SiteLayout.astro         # Single HTML shell: <head>, data-reveal observer, page-noise/glow overlays
│   │
│   ├── content/                     # Astro content collections
│   │   └── docs/
│   │       ├── guides/              # MDX: disable-and-enable-skills, first-project, install-and-verify,
│   │       │                        #       move-unmanaged-skills, onboard-a-tool, watch-mode-and-service
│   │       └── reference/           # MDX: about, completion, onboard, project, self-update, service, skill, status
│   │
│   ├── data/                        # Typed static content (the canonical marketing surface)
│   │   ├── commands.ts              # CommandGroup[] — the table of every CLI command the site advertises
│   │   └── home.ts                  # installSnippets, heroInstallMethods, terminalScenarios, featureCards, faqItems, …
│   │
│   ├── icons/                       # Astro Icon sprite sources (loaded by astro-icon integration)
│   │   ├── Solid/
│   │   ├── outline/
│   │   └── tech/
│   │   └── README.md
│   │
│   └── lib/                         # kebab-case helpers
│       ├── cli-version.ts           # Reads PUBLIC_CLI_VERSION (built from ../VERSION)
│       ├── docs.ts                  # docsSections, sortDocs, getDocsBySection, getCommandBySlug, trimCollectionSlug
│       └── with-base.ts             # Prefix asset paths with import.meta.env.BASE_URL
│
└── test/                            # Test files (see vitest.config.ts include)
    ├── docs.test.ts                 # Vitest unit tests (cliVersion, getCommandBySlug, trimCollectionSlug)
    └── e2e/
        └── smoke.spec.ts            # Playwright smoke test
```

## File Naming Conventions

| Surface | Convention | Examples |
| --- | --- | --- |
| Go source files | **snake_case** | `skill_add.go`, `frontmatter.go`, `skillscli.go`, `discovery_test.go` |
| Go test files | **snake_case** with `_test.go` suffix, co-located with the file they test | `cmd/sync.go` ↔ `cmd/sync.go` (no separate test) and `cmd/skill_add_test.go` |
| Go packages | single-word lower case | `config`, `skills`, `sync`, `overlap`, `agenttools`, `skillscli`, `versionfmt` |
| Go types / functions / constants | **PascalCase** exported, **camelCase** unexported | `Skill`, `ScanSource`, `newSkillCommand`; `loadResolvedLocation`, `runCommand`, `resolveConfigPath` |
| Go function-variable DI | lowerCamelCase, prefixed with the command name | `skillDeleteLoadResolvedLocation`, `skillAddSandbox`, `runSkillAddSync` |
| Go cobra command names | lowercase, single word or `<verb> <object>` | `sync`, `add`, `check-updates`, `move-unmanaged` |
| Go cobra aliases | lowercase verbs | `install`/`import` aliases for `skill add` (`cmd/skill_add.go:45`); `remove`/`rm` for `skill delete` |
| Astro components | **PascalCase.astro** | `HeroSection.astro`, `AgentToolLogo.astro`, `DocsLayout.astro` |
| TypeScript lib helpers | **kebab-case.ts** | `cli-version.ts`, `with-base.ts`, `docs.ts` |
| TypeScript type names | **PascalCase** | `CommandGroup`, `TerminalScenario`, `FeatureCard`, `InstallMethod` |
| Web data modules | **kebab-case.ts** for files; **PascalCase** types inside | `commands.ts`, `home.ts` |
| Web content (MDX/Markdown) | **kebab-case.mdx** | `install-and-verify.mdx`, `onboard-a-tool.mdx`, `disable-and-enable-skills.mdx` |
| Web test files | **kebab-case.test.ts** (Vitest) and **kebab-case.spec.ts** (Playwright) | `docs.test.ts`, `smoke.spec.ts` |
| npm wrapper scripts | **kebab-case.js** (CommonJS) | `skill-organizer.js`, `postinstall.js` |
| YAML config | **.yml** extension, lower-case named | `.skill-organizer.yml` (per-project), `skill-organizer.yml` (global registry) |
| Brand assets | **kebab-case + extension** | `logo_color.png`, `logo_color2.png`, `logo_color_simplified.png` |
| Web icons | **PascalCase.astro** (under `components/logos/`) or kebab-case in icon sub-collections | `Claude.astro`, `Apple.astro`; `cli-version.ts` |

There is no `PascalCase` or `snake_case` Go source outside the rules above. There are no `SCREAMING_SNAKE_CASE` Go constants. Astro `pages/` filenames are intentionally lowercase (e.g. `index.astro`).

## Where to Find Things

A quick-reference for "I want to look at X, where do I start?"

- **"How is a cobra command built?"** — `packages/cli/cmd/root.go` for the wiring, then any `packages/cli/cmd/skill_*.go` for the convention.
- **"Where does a single skill get loaded from disk?"** — `packages/cli/internal/skills/scanner.go` (path) and `packages/cli/internal/skills/frontmatter.go` (YAML).
- **"How are target symlinks created or repaired?"** — `packages/cli/internal/sync/sync.go` (`reconcileTargetEntry` at `sync.go:117-154`).
- **"Where is the manifest file format defined?"** — `packages/cli/internal/sync/manifest.go` (`Manifest` struct at `manifest.go:15-17`).
- **"How is a skill imported from skills.sh?"** — `packages/cli/internal/remote/skillscli.go` (the sandbox + bundle machinery) and `packages/cli/cmd/skill_add.go` (the cobra command that drives it).
- **"How does the watcher avoid loops with its own writes?"** — `packages/cli/internal/watch/watch.go` `ignoredUntil` map (`watch.go:361-377`) plus the `markIgnored` calls around the `syncpkg.Run` call (`watch.go:265`).
- **"How does the service start, and where is the Linux fallback?"** — `packages/cli/internal/service/service.go` (`New` at `service.go:38-56`, Linux path at `service.go:155-207`).
- **"What env vars does the CLI read?"** — Already documented in `.planning/codebase/STACK.md` ("Environment" section). Most-cited: `TERM`, `NO_COLOR`, `HOME`, `XDG_CONFIG_HOME`, `npm_execpath`, `HOMEBREW_PREFIX`, `SKILL_ORGANIZER_E2E_REAL_NPX`.
- **"How does the CLI detect how it was installed?"** — `packages/cli/internal/selfupdate/selfupdate.go` `DetectInstallMethod` at `selfupdate.go:167-192`.
- **"Where is the CLI command surface that the docs site advertises?"** — `packages/web/src/data/commands.ts` (the `commandGroups` table) and `packages/web/src/data/home.ts` (install methods, feature cards, FAQ).
- **"Where do the docs pages come from?"** — `packages/web/src/content/docs/{guides,reference}/*.mdx` (files) + `packages/web/src/content.config.ts` (schema) + `packages/web/src/pages/docs/[section]/[slug].astro` (route).
- **"Where is the CLI version rendered on the site?"** — `packages/web/src/lib/cli-version.ts` reads `import.meta.env.PUBLIC_CLI_VERSION`, set by `packages/web/astro.config.mjs:8,15`. Used in `packages/web/src/views/home/hero/HeroSection.astro:21`.
- **"Where is the home page composed?"** — `packages/web/src/pages/index.astro:15-28` imports 9 `views/home/*` sections into `SiteLayout`.
- **"Where are the e2e tests for the CLI?"** — `packages/cli/e2e_test.go` (22.9 KB). Backed by the fake `skills` binary in `packages/cli/testdata/fake-skills-cli.go`.
- **"Where is the open-spec change in flight?"** — `openspec/changes/build-skill-organizer-cli/` (`proposal.md`, `design.md`, `tasks.md`, `specs/`). Archived changes are in `openspec/archive/`.
- **"Where is the CI pipeline defined?"** — `.github/workflows/ci.yml` (PR + push to main/alpha/beta), `cli-e2e.yml` (binary e2e + npx smoke, manual trigger), `release-please.yml` + `auto-merge-release-please.yml` (versioning), `release.yml` (GoReleaser + Homebrew tap), `publish-npm.yml` (npm publish via trusted publishing), `deploy-web.yml` (GitHub Pages).
- **"Where is the release playbook for humans?"** — `docs/releasing.md` and the per-track shell scripts in `scripts/release-{alpha,beta,stable}.sh` + `scripts/release-common.sh`.
