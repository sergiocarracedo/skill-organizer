# STACK

Tech-stack reference for the `skill-organizer` monorepo at `/works/opensource/skill-organizer`.

Repo layout: pnpm workspace with two top-level packages (`packages/web`, `packages/cli/packages/npm`) and a Go module under `packages/cli/`. Current version `1.0.2` (see `VERSION:1`, `release-please-manifest.json:2`, `CHANGELOG.md:5`).

## Languages

- **Go** — `1.24.0` declared in `packages/cli/go.mod:3`. Source files use `.go`. CLI binary entry point: `packages/cli/main.go:10` (`package main`, calls `cmd.Execute()` from `packages/cli/cmd/root.go:32`). Production source is all in `packages/cli/cmd/*.go` and `packages/cli/internal/**/*.go`. There is a checked-in prebuilt binary at `packages/cli/skill-organizer` (17 MB).
- **TypeScript** — used by the web package. `packages/web/tsconfig.json:2` extends `astro/tsconfigs/strict`. Source files use `.ts`, `.astro`, `.mjs`. Astro 6 generates MDX + Astro islands.
- **JavaScript (Node)** — minimal, only the npm wrapper at `packages/cli/packages/npm/bin/skill-organizer.js:1` and `packages/cli/packages/npm/scripts/postinstall.js:1` (CommonJS, `require()`).
- **YAML** — config files (`.skill-organizer.yml`, `skill-organizer.yml`, `skills-lock.json`, `release-please-config.*.json`).
- **JSON** — `release-please-manifest.json`, `pnpm-lock.yaml`, `package.json`, `skills-lock.json`.
- **CSS** — Tailwind CSS v4 (utility classes only, see `packages/web/astro.config.mjs:17`).
- **MDX / Markdown** — docs site content under `packages/web/src/content/docs/`.
- **Shell** — `lefthook.yml` hooks and helper scripts in `scripts/release-*.sh`.

## Runtime

- **Node.js** — `>=18` required by the npm wrapper at `packages/cli/packages/npm/package.json:34`. CI pins Node `24` (`packages/web/.../ci.yml:47`, `deploy-web.yml:42`, `publish-npm.yml:27`). There is no `.nvmrc` or `.tool-versions` file at the repo root.
- **Go toolchain** — `1.24.0` per `packages/cli/go.mod:3`. CI uses `actions/setup-go@v6` with `go-version-file: packages/cli/go.mod` (`ci.yml:31`, `release.yml:24`).
- **Package manager** — **pnpm `10.18.3`** pinned in `package.json:4` (`"packageManager": "pnpm@10.18.3"`). Workspace declared in `pnpm-workspace.yaml:1`:
  ```yaml
  packages:
    - packages/web
    - packages/cli/packages/npm
  ```
  Allowed build script: `esbuild` (`package.json:30`).
- **Lockfiles** — `pnpm-lock.yaml` (root), `packages/cli/go.sum`. The npm wrapper directory `packages/cli/packages/npm/` is part of the pnpm workspace and ships its own `package.json` but no nested lockfile (it is published, not consumed internally as a dependency).
- **OS support for the CLI binary** — `linux`/`darwin`/`windows` × `arm`/`amd64`/`arm64`, ignoring several matrix cells per `packages/cli/.goreleaser.yaml:24-30`. The npm `postinstall.js:18-26` only supports `linux`+`darwin`+`win32` and `x64`+`arm64` (it does NOT support 32-bit ARM).

## Frameworks

- **UI framework (web)** — **Astro `^6.1.9`** (`packages/web/package.json:21`) with the MDX integration `@astrojs/mdx ^5.0.4` and `astro-icon ^1.1.5`. Tailwind CSS v4 is wired in via the Vite plugin `@tailwindcss/vite ^4.2.4`. The site is configured for GitHub Pages at `https://skill-organizer.sergiocarracedo.es` (`packages/web/astro.config.mjs:11`). Tailwind typography or daisyUI are not used.
- **CLI framework (Go)** — **spf13/cobra `v1.9.1`** (`packages/cli/go.mod:10`) plus the interactive terminal library **pterm `v0.12.83`** (`packages/cli/go.mod:9`) and **atomicgo/keyboard `v0.2.9`** (`packages/cli/go.mod:6`). Cobra root is wired in `packages/cli/cmd/root.go:26`; commands are registered in `root.go:94-105` (sync, status, about, completion, onboard, project, skill, watched, watch, service, self-update).
- **Test framework (Go)** — standard `go test ./...` (see `package.json:14` and `ci.yml:35`). E2E tests in `packages/cli/e2e_test.go:1` (22.9 KB) use `github.com/creack/pty` for pty-based runs.
- **Test framework (web)** — **Vitest `^4.1.5`** with the **v8** coverage provider and `@vitest/coverage-v8`, plus **Playwright `^1.59.1`** for E2E (`packages/web/package.json:28-38`). Coverage thresholds are 0.95 for lines/statements/functions/branches (`packages/web/vitest.config.ts:19-22`). Playwright runs against `pnpm preview` on `http://127.0.0.1:4321` (`playwright.config.ts:7-15`).
- **Build tool** — Astro CLI (`astro dev` / `astro build` / `astro check`) is the front-end driver; Go uses `go build ./...` (`package.json:7-8`).
- **Background service runtime** — **`github.com/kardianos/service v1.2.4`** (`packages/cli/go.mod:8`). On Linux, a custom `systemctl --user` path is used (`packages/cli/internal/service/service.go:155-207` writes `~/.config/systemd/user/skill-organizer.service`).

## Key Libraries

The most important runtime/CLI libraries, in priority order:

- **`spf13/cobra v1.9.1`** — CLI command tree. `packages/cli/cmd/root.go:26-119` registers every command and its persistent flags (`--config`, `rootCmd.Version`).
- **`pterm/pterm v0.12.83`** — terminal output, sections, spinners, interactive selectors. Used in almost every command (e.g. `packages/cli/cmd/onboard.go:31`, `cmd/prompt.go`).
- **`atomicgo.dev/keyboard v0.2.9`** — keyboard input primitive for the interactive path selector (`packages/cli/cmd/editable_path_selector.go:1`).
- **`github.com/fsnotify/fsnotify v1.9.0`** — filesystem watcher for the `watch` command and background service (`packages/cli/internal/watch/watch.go:12`).
- **`github.com/kardianos/service v1.2.4`** — cross-platform service abstraction for `service install/start/stop` (`packages/cli/internal/service/service.go:12`).
- **`gopkg.in/yaml.v3 v3.0.1`** — config parsing (project YAML, app YAML, manifests, frontmatter; see `packages/cli/internal/config/registry.go:9` and `packages/cli/internal/skills/frontmatter.go:11`).
- **`github.com/pmezard/go-difflib v1.0.0`** — diff rendering for `skill check-updates` (`packages/cli/cmd/skill_check_updates.go:13`).

Indirect-but-notable: **`github.com/creack/pty v1.1.24`** (used by E2E tests), **`lithammer/fuzzysearch v1.1.8`** (pterm dep, used for fuzzy filter), **`mattn/go-runewidth v0.0.20`** (pterm dep).

Web key libraries:

- **`astro ^6.1.9`**, **`@astrojs/mdx ^5.0.4`**, **`astro-icon ^1.1.5`**.
- **`tailwindcss ^4.2.4`** + **`@tailwindcss/vite ^4.2.4`** (no PostCSS config; Tailwind v4 is configured entirely from CSS).
- **`typescript ^5.9.3`**, **`vitest ^4.1.5`**, **`@vitest/ui ^4.1.5`**, **`@vitest/coverage-v8 ^4.1.5`**, **`@playwright/test ^1.59.1`**, **`playwright ^1.59.1`**.
- **`oxlint ^1.62.0`** + **`oxc-parser ^0.128.0`** + **`@commitlint/cli ^20.5.2`** + **`@commitlint/config-conventional ^20.5.0`** (`packages/web/package.json:24-38`; the web has its own `lefthook.yml` running oxlint, prettier format check, `astro check`, vitest, and commitlint).

## Build & Tooling

- **Root scripts** (`package.json:5-24`):
  - `build` → `build:cli` (`cd packages/cli && go build ./...`) + `build:web` (`pnpm --filter web build`).
  - `lint` → `lint:web` (`pnpm --filter web lint` = `oxlint .`).
  - `type-check` → `type-check:web` (`pnpm --filter web type-check` = `astro check`).
  - `test` → `test:cli` (`go test ./...`) + `test:web` (`vitest run --coverage`).
  - `test:cli:e2e` → runs three Go e2e tests against a compiled binary, gated by `packages/cli/lefthook.yml:3-9` on commits that touch CLI sources.
  - `test:cli:e2e:real-npx` → requires `SKILL_ORGANIZER_E2E_REAL_NPX=1` and exercises the real `npx skills` CLI (see `packages/cli/e2e_test.go:162-171`).
  - `dev:web` → `pnpm --filter web dev`.
  - `hooks:install` → `pnpm exec lefthook install`. `prepare` runs it on `pnpm install` (`package.json:23`).
- **Pre-commit hook (root)** — `lefthook.yml:1-10` only runs `pnpm run test:cli:e2e` when staged files match `package.json`, `pnpm-lock.yaml`, `lefthook.yml`, `.github/workflows/cli-e2e.yml`, or anything under `packages/cli/**`.
- **Pre-commit hook (web)** — `packages/web/lefthook.yml:1-15` runs `oxlint .`, `prettier --check`, `astro check`, `vitest run`, and `commitlint` for commit messages.
- **Release tooling**:
  - **release-please** is run via `googleapis/release-please-action@v5` from `.github/workflows/release-please.yml:18-47`. Three configs are present:
    - `release-please-config.alpha.json` (alpha track, `prerelease-type: alpha`).
    - `release-please-config.beta.json` (beta track, `prerelease-type: beta`).
    - `release-please-config.stable.json` (main, no prerelease).
    Each one bumps `VERSION`, `release-please-manifest.json`, `CHANGELOG.md`, and `packages/cli/packages/npm/package.json` (jsonpath `$.version`). Branches: `main`, `alpha`, `beta` (`ci.yml:6-9`, `release-please.yml:6-8`).
  - **release-please auto-merge** is performed by `.github/workflows/auto-merge-release-please.yml:1-72` which validates the PR provenance (branch pattern, no cross-repo, no prerelease into `main`) before calling `gh pr merge --merge --delete-branch=false`.
  - **GoReleaser v2** is invoked by `.github/workflows/release.yml:27-36` with `goreleaser/goreleaser-action@v7`, `distribution: goreleaser`, `version: '~> v2'`, `workdir: packages/cli`, `args: release --clean`. The `before.hooks` block in `packages/cli/.goreleaser.yaml:5-8` runs `go mod tidy` and `go test ./...` before the build. Archives are `tar.gz` for Linux/macOS and `zip` for Windows (`packages/cli/.goreleaser.yaml:38-49`); the name template injects `-X` ldflags for `cmd.version`, `cmd.commit`, `cmd.date` (`packages/cli/.goreleaser.yaml:33-35`).
  - **Homebrew tap** is updated by GoReleaser's `brews` section (`packages/cli/.goreleaser.yaml:67-85`) into `sergiocarracedo/homebrew-tap`, using secret `HOMEBREW_TAP_GITHUB_TOKEN`. `skip_upload: auto` causes prereleases to be skipped automatically.
  - **npm publish** runs from `.github/workflows/publish-npm.yml:1-62`, callable as `workflow_call` (release workflow) or manually. It uses npm trusted publishing (`id-token: write`, no `NPM_TOKEN`). The dist-tag is decided from the package version (`-alpha.*` → `alpha`, `-beta.*` → `beta`, else `latest`).
  - Local release pre-flight: `scripts/release-alpha.sh`, `scripts/release-beta.sh`, `scripts/release-stable.sh` and the shared `scripts/release-common.sh`.
- **Conventional Commits** — enforced by `ci.yml:12-20` (PR title regex) and web `commitlint.config.js:1-3` (commit message via `commitlint --edit {1}` in `packages/web/lefthook.yml:13-15`).

## Environment

- **No `.nvmrc` / `.tool-versions` / `.node-version`** is present at the repo root. The npm wrapper sets `engines.node: ">=18"` (`packages/cli/packages/npm/package.json:34`).
- **CI Node version** — `node-version: 24` in `ci.yml:47`, `deploy-web.yml:42`, `publish-npm.yml:27`.
- **Go version** — read from `packages/cli/go.mod` (`go 1.24.0`).
- **Environment variables consumed by the CLI** (sourced by `grep os.Getenv` over `packages/cli/**/*.go`):
  - `npm_execpath`, `npm_config_user_agent` → identifies an npm install (`packages/cli/internal/selfupdate/selfupdate.go:168`).
  - `HOMEBREW_PREFIX`, `HOMEBREW_CELLAR` → identifies a Homebrew install (`selfupdate.go:171`).
  - `TERM=dumb` and `NO_COLOR` → disable color/logo output (`packages/cli/cmd/logo.go:25,28,35,38`).
  - `HOME` → user home (used in `internal/config/path.go:35,43` and tests).
  - `XDG_CONFIG_HOME` → user config dir (default Linux location for `os.UserConfigDir()`); tests stub it (`packages/cli/internal/status/status_test.go:37`).
  - `SKILL_ORGANIZER_E2E_REAL_NPX` → gates the real `npx skills` e2e (`packages/cli/e2e_test.go:162`).
  - `SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES`, `SKILL_ORGANIZER_FAKE_SKILLS_WORKDIR`, `SKILL_ORGANIZER_FAKE_SKILLS_STATE`, `SKILL_ORGANIZER_FAKE_PUBLIC_REPOS` → control the fake `skills` CLI fixture used in e2e tests (`packages/cli/testdata/fake-skills-cli.go:123,243,254,322-323`).
  - Sandbox env used when invoking the `skills` CLI: `FORCE_COLOR=0`, `NO_COLOR=1`, `CI=1`, `npm_config_update_notifier=false`, plus `HOME=<tmp>` (`packages/cli/internal/remote/skillscli.go:133-146`).
- **CI secrets** referenced in `.github/workflows/*.yml`:
  - `GITHUB_TOKEN` (default) — used by GoReleaser (`release.yml:35`).
  - `HOMEBREW_TAP_GITHUB_TOKEN` — used to push into `sergiocarracedo/homebrew-tap` (`release.yml:36`).
  - `RELEASE_PLEASE_TOKEN` — PAT for release-please PRs and the auto-merge job (`release-please.yml:22,34,46`; `auto-merge-release-please.yml:25,69`).
  - `NPM_TOKEN` is intentionally not used; npm uses trusted publishing (`publish-npm.yml:9`, `docs/releasing.md:194`).
- **Repo-level state files**:
  - `VERSION` (`1.0.2`) — single source of truth.
  - `release-please-manifest.json` — tracks the current version per release track.
  - `skills-lock.json` — used by the e2e fixture story for `asciinema-recorder` imported from `terrylica/cc-skills`.
