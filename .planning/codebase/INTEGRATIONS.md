# INTEGRATIONS

External integrations, network calls, on-disk state, and distribution channels for the `skill-organizer` monorepo.

The product keeps two views of agent-skill directories: an organized source tree you edit, and a flat target folder (typically `~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`, `~/.agent/skills`) that agent tools consume. See `README.md:7-9` and `packages/cli/internal/config/discovery.go:138-145` (`candidateTargetPatterns`).

## External APIs

The CLI does not talk to most public APIs directly. The two real network integrations it owns are:

- **GitHub REST API — Releases endpoint.** Used by the in-CLI self-update check.
  - URL: `https://api.github.com/repos/sergiocarracedo/skill-organizer/releases/latest` (`packages/cli/internal/selfupdate/selfupdate.go:29`).
  - Headers: `Accept: application/vnd.github+json`, `User-Agent: skill-organizer-cli` (`selfupdate.go:249-250`).
  - Timeout: 10 s for the metadata request, 30 s for the asset download (`selfupdate.go:252, 328`).
  - Response shape: `{ "tag_name": "...", "html_url": "..." }` (`selfupdate.go:43-46`).
  - Cached on disk in the user config dir (see File System below).
  - Unauthenticated; rate-limited by GitHub (60 req/h/IP for unauthenticated). Periodic check interval is 12 h (`selfupdate.go:26`).
  - Asset download URL is built as `https://github.com/sergiocarracedo/skill-organizer/releases/download/<tag>/skill-organizer_<version>_<OS>_<arch>.tar.gz` (or `.zip` on Windows) — see `archiveNameFor` at `selfupdate.go:286-302`.
  - Direct binary self-update is currently disabled and returns an error pointing the user at the release page (`selfupdate.go:304-308`).

- **`skills.sh` via the `skills` CLI (or `npx skills`).** Used by `skill add`, `skill check-updates`, `skill try-find-metadata`, and the `find` workflow.
  - The CLI shells out to the `skills` binary; if it is not on `PATH` it falls back to `npx skills` (`packages/cli/internal/remote/skillscli.go:97-105`).
  - Sandbox env: `HOME=<tmp>`, `FORCE_COLOR=0`, `NO_COLOR=1`, `CI=1`, `npm_config_update_notifier=false` (`skillscli.go:133-146`).
  - Provider label stored in summaries is `skills.sh` (`skillscli.go:194, 405, 596`).
  - It is the third-party `skills.sh` registry, not a project-owned service.
  - Real network smoke test in CI: `npx skills add https://github.com/vercel-labs/agent-browser --skill agent-browser` (`packages/cli/e2e_test.go:171`, gated by `SKILL_ORGANIZER_E2E_REAL_NPX=1`).

- **Agent tool CLIs (local exec, not network).** The CLI shells out to installed binaries for overlap analysis. Detected by `lookPath` in `packages/cli/internal/agenttools/agenttools.go:87-102`:
  - `claude` (Claude Code) — `Args: ["-p", prompt]`, `PlanArgs: ["--permission-mode", "plan", prompt]` (`agenttools.go:30-36`).
  - `codex` (OpenAI Codex) — `Args: ["exec", prompt]`.
  - `opencode` (OpenCode) — `Args: ["run", prompt]`.
  - `agent` (Cursor) — `Args: ["-p", prompt]`.
  - `antigravity-cli` or `agcl` (Antigravity) — `Args: [prompt]`.
  - These calls may spend the user's paid API credits. The CLI prints a one-time notice that must be acknowledged and stored in the global config (`README.md:283-287`, `packages/cli/internal/config/config.go:36-39`).

- **Web app — GitHub REST API for star count.** `packages/web/src/components/GitHubStarButton.astro:15` calls `https://api.github.com/repos/sergiocarracedo/skill-organizer` from the browser, sets `Accept: application/vnd.github+json` (`:84-86`), reads `stargazers_count`, and falls back to a hardcoded `1` on failure (`:16, 92-103`).

- **Web app — CDN scripts for the hero animation.** `packages/web/src/views/home/hero/HeroWaves.astro:55-56` lazy-loads `https://cdnjs.cloudflare.com/ajax/libs/three.js/r121/three.min.js` and `https://cdn.jsdelivr.net/npm/vanta@latest/dist/vanta.waves.min.js`. These are the only third-party script origins the static site loads at runtime.

- **Outbound in the npm postinstall.** `packages/cli/packages/npm/scripts/postinstall.js:42-43` downloads `https://github.com/sergiocarracedo/skill-organizer/releases/download/v<version>/skill-organizer_<version>_<OS>_<arch>.{tar.gz|zip}` and the matching `checksums.txt`. Redirects are only followed to `github.com` or `release-assets.githubusercontent.com` (`:17, 105-115`).

There is **no authentication / login / OAuth flow** anywhere in the codebase. All API calls are unauthenticated.

## Authentication

The CLI does not authenticate against any service. Concrete shape of auth-related code:

- The web GitHub star button calls the unauthenticated REST endpoint and silently falls back to `1` star on any error (`packages/web/src/components/GitHubStarButton.astro:101-103`).
- The npm postinstall hardcodes owner/repo (`sergiocarracedo` / `skill-organizer`) with no env override (`:13-17`).
- The CLI's self-update check uses the public unauthenticated GitHub API (`packages/cli/internal/selfupdate/selfupdate.go:29, 244-256`).
- CI uses:
  - `secrets.GITHUB_TOKEN` (Actions default) for GoReleaser (`release.yml:35`).
  - `secrets.HOMEBREW_TAP_GITHUB_TOKEN` to push into `sergiocarracedo/homebrew-tap` (`release.yml:36`; configured in `packages/cli/.goreleaser.yaml:75`).
  - `secrets.RELEASE_PLEASE_TOKEN` for release-please PRs (`release-please.yml:22, 34, 46`) and for the auto-merge validation step (`auto-merge-release-please.yml:25, 69`).
  - npm publishing via trusted publishing OIDC (`id-token: write` in `publish-npm.yml:9`); no `NPM_TOKEN` is used (`docs/releasing.md:194`).
- The CLI itself reads `npm_execpath` / `HOMEBREW_PREFIX` only to *detect* how it was installed; it never reads tokens (`packages/cli/internal/selfupdate/selfupdate.go:167-192`).

## Databases / Storage

The project has no SQL/NoSQL database. All persistent state is files on disk.

- **User-level app config** — YAML in the OS user config dir:
  - Location: `os.UserConfigDir() + "/skill-organizer/skill-organizer.yml"` (`packages/cli/internal/config/registry.go:12-28`).
  - On Linux this resolves to `~/.config/skill-organizer/skill-organizer.yml` (documented in `README.md:175-176`).
  - Contents: watched project paths, service log level, overlap defaults, backup retention (`packages/cli/internal/config/config.go:45-50`).
- **User-level update cache** — `<appDir>/.cache.yml` (per `CachePath()` in `packages/cli/internal/config/cache.go:47-54`). Stores `last-checked-at`, `latest-version`, `latest-page-url`, plus per-task last-checked timestamps for `backup-gc` and `skill-updates`.
- **User-level updates state** — `<appDir>/.updates` (`UpdatesPath()` at `cache.go:56-62`). Written by the periodic 30-day "no updates checked" reminder (`packages/cli/internal/maintenance/maintenance.go:15, 42-48`).
- **Backups** — `<appDir>/.old/<timestamp>-<flattened-name>` (per `README.md:262-268`). Pruned by `maintenance.MaybeRunBackupGC` with default retention `10` days (`config.go:117`, `maintenance/maintenance.go:55-85`).
- **Per-project config** — `<dir>/.skill-organizer.yml` (constant `FileName` at `packages/cli/internal/config/discovery.go:13`). YAML shape (`config.go:9-12`):
  ```yaml
  source: /repo/.agents/skills-organized
  target: /repo/.agents/skills
  ```
  Discovered by walking up from cwd until a `.skill-organizer.yml` is found (`discovery.go:57-83`).
- **Per-project target manifest** — hidden file written by `sync.Run` next to the target tree (`packages/cli/internal/sync/sync.go:70-114`; `manifest.go:1` declares it). Tracks which flattened names are managed so `sync` can remove stale symlinks.
- **The skills themselves** — `SKILL.md` files (YAML frontmatter + body). The CLI rewrites the `name` field and the `metadata.skill-organizer` block during sync (`packages/cli/internal/skills/frontmatter.go:159-189`).
- **Service unit (Linux)** — `~/.config/systemd/user/skill-organizer.service`, written by `installUserUnit` (`packages/cli/internal/service/service.go:209-256`).

## Network

Summary of every outbound network call in the project:

| Origin | Destination | Trigger | Auth | File |
| --- | --- | --- | --- | --- |
| CLI self-update | `https://api.github.com/repos/sergiocarracedo/skill-organizer/releases/latest` | every CLI run, 12 h cache, plus `self-update` subcommand | none | `packages/cli/internal/selfupdate/selfupdate.go:29, 244-256` |
| CLI self-update | `https://github.com/sergiocarracedo/skill-organizer/releases/download/<tag>/skill-organizer_<version>_<OS>_<arch>.{tar.gz,zip}` | `self-update` for non-Direct install methods (npm/Homebrew) | none | `selfupdate.go:282, 321-350` |
| CLI overlap | local `claude` / `codex` / `opencode` / `agent` / `antigravity-cli` / `agcl` via `exec.Command` | `skill check-overlap` | inherits local user | `packages/cli/internal/agenttools/agenttools.go:24-77` |
| CLI skills add / find | `npx skills ...` or local `skills` binary (which itself talks to skills.sh) | `skill add`, `skill check-updates`, `skill try-find-metadata` | inherits | `packages/cli/internal/remote/skillscli.go:97-105, 114-131` |
| npm postinstall | `https://github.com/sergiocarracedo/skill-organizer/releases/download/v<version>/...` and `.../checksums.txt` | `npm install -g skill-organizer` postinstall | none, redirect allowlist `github.com`, `release-assets.githubusercontent.com` | `packages/cli/packages/npm/scripts/postinstall.js:42-57, 102-127` |
| Web (browser) | `https://api.github.com/repos/sergiocarracedo/skill-organizer` | GitHub star button | none | `packages/web/src/components/GitHubStarButton.astro:14-103` |
| Web (browser) | `https://cdnjs.cloudflare.com/ajax/libs/three.js/r121/three.min.js`, `https://cdn.jsdelivr.net/npm/vanta@latest/dist/vanta.waves.min.js` | hero wave animation | none | `packages/web/src/views/home/hero/HeroWaves.astro:55-56` |

Protocols: only **HTTPS** is used (`http.NewRequest` is never given an `http://` URL by the CLI; the npm postinstall asserts `redirect.protocol === "https:"` at `postinstall.js:107`). The web site is served via GitHub Pages at `https://skill-organizer.sergiocarracedo.es` (`packages/web/astro.config.mjs:11`, `deploy-web.yml:62-68`).

The CLI also performs **in-process HTTP via `os/exec`** against the local `claude`/`codex`/`opencode`/`agent`/`antigravity-cli` binaries (see above). It does not open a TCP port of its own.

## File System

Where the CLI reads and writes on the user's machine. All paths are absolute after `config.ResolvePath`, which expands `~` and `$ENV` vars (`packages/cli/internal/config/path.go:10-51`).

**Per-project (managed by `sync` and related commands):**
- Source tree — recursively scanned for `SKILL.md`; only directories containing `SKILL.md` are treated as skills, and recursion stops there (`packages/cli/internal/skills/scanner.go:42-88`).
- Source `SKILL.md` files — read for name, description, and `metadata.skill-organizer`; rewritten in place to inject managed metadata (`packages/cli/internal/skills/frontmatter.go:32-60, 159-202, 313-333`).
- Target tree — the flat folder agent tools read. The CLI creates `os.Symlink` entries pointing back into the source tree (`packages/cli/internal/sync/sync.go:117-154`) and a hidden manifest of managed names. It also drops an `IMPORTANT.md` notice (mentioned in `README.md:130-135`).
- `.skill-organizer.yml` next to the target — read/written by every `project` subcommand (`packages/cli/internal/config/discovery.go:36-55`).
- `.tmp/` and `vendor/` inside the npm package — created and removed by `postinstall.js` (`packages/cli/packages/npm/scripts/postinstall.js:44-75`).

**User-level (`os.UserConfigDir()` on Linux = `~/.config`):**
- `skill-organizer/skill-organizer.yml` — app config (watched paths, service log level, overlap defaults, backup retention) (`packages/cli/internal/config/registry.go:12-28`).
- `skill-organizer/.cache.yml` — self-update cache + backup-GC + skill-update task timestamps (`packages/cli/internal/config/cache.go:47-54`).
- `skill-organizer/.updates` — periodic 30-day "no updates checked" reminder state (`cache.go:56-62`).
- `skill-organizer/.old/<ts>-<flattened-name>` — deleted/updated skill backups (`README.md:262-268`).
- `skill-organizer/cache/skills/` — reserved cache directory (declared in `CacheDir()` at `packages/cli/internal/remote/skillscli.go:468-474`).
- `systemd/user/skill-organizer.service` on Linux — installed by `service install` (`packages/cli/internal/service/service.go:209-256`).

**Sandboxes (created in `os.MkdirTemp`):**
- The CLI spins up throwaway temp dirs when invoking `skills` / `npx skills` (`packages/cli/internal/remote/skillscli.go:148-168, 417-448`) so its reads/writes never pollute the real home or project. These temp dirs are removed via `defer sandbox.Close()` (`:170-172`).
- The `e2e_test.go` builds a fake `skills` CLI on disk under `os.MkdirTemp` and puts it on `PATH` for the test binary (`packages/cli/e2e_test.go:304, 441`).

**Read-only / informational reads:**
- `os.Executable()` to detect the install method (npm vs Homebrew vs direct) (`packages/cli/internal/selfupdate/selfupdate.go:175-191`).
- `os.UserHomeDir()` to resolve `~/` and to locate the Linux systemd unit (`internal/config/path.go:35-50`, `internal/service/service.go:251-256`).

## Distribution

The CLI ships through three independent channels, all driven by the same `VERSION` and tag:

- **GitHub Releases (primary) — GoReleaser v2.**
  - Triggered by `.github/workflows/release.yml:1-43` on any `v*` tag.
  - Uses `goreleaser/goreleaser-action@v7`, `distribution: goreleaser`, `version: '~> v2'`, `workdir: packages/cli`, `args: release --clean` (`release.yml:27-36`).
  - Builds a CGO-disabled static binary for `linux`/`darwin`/`windows` × `amd64`/`arm64`/`arm` (with several ignored cells in `packages/cli/.goreleaser.yaml:16-30`).
  - Archive layout: `tar.gz` (Linux/macOS) and `zip` (Windows) named `skill-organizer_<version>_<OS>_<arch>.<ext>` (`:38-49`).
  - Each archive includes `checksums.txt` and `README.md` (`:50-54`).
  - Injecting version metadata via `-X` ldflags (`packages/cli/.goreleaser.yaml:31-35`).
  - Prerelease detection uses `prerelease: auto`, `make_latest: false` (`:59-62`).

- **Homebrew tap — `sergiocarracedo/homebrew-tap`.**
  - Updated by GoReleaser's `brews` section (`packages/cli/.goreleaser.yaml:67-85`).
  - Formula directory: `Formula`, license: `UNLICENSED`, description: "Organize structured skill trees into flat tool-readable targets" (`:79-82`).
  - `skip_upload: auto` automatically skips prerelease versions (`:85`).
  - Auth via `secrets.HOMEBREW_TAP_GITHUB_TOKEN` (`release.yml:36`).
  - User-facing install: `brew tap sergiocarracedo/tap && brew install skill-organizer` (`README.md:40-45`, `docs/releasing.md:228-238`).

- **npm — `skill-organizer` (thin wrapper around the GitHub binary).**
  - Wrapper package at `packages/cli/packages/npm/` (`package.json:1-36`). Declared as part of the pnpm workspace (`pnpm-workspace.yaml:2`).
  - `bin` field points at `bin/skill-organizer.js`, which `spawn()`s the vendored Go binary with the user's args (`packages/cli/packages/npm/bin/skill-organizer.js:1-22`).
  - `postinstall` script (`scripts/postinstall.js:1-185`) downloads the matching GoReleaser archive from GitHub Releases, verifies the SHA-256 against `checksums.txt`, extracts the binary into `vendor/`, and `chmod 0o755`s it (`:53-75, 155-180`). Detects a source-checkout install by walking up the tree to find a `package.json` named `skill-organizer-monorepo` (`:78-100`) and skips the download.
  - Published by `.github/workflows/publish-npm.yml:1-62` via npm trusted publishing OIDC (`id-token: write`). Dist-tag is auto-derived from the version: `*-alpha.*` → `alpha`, `*-beta.*` → `beta`, else `latest` (`:33-47`). The release workflow calls this as `workflow_call` after GoReleaser finishes (`release.yml:38-42`).
  - The package itself has no source-build path; the version in `packages/cli/packages/npm/package.json` is bumped by release-please (`release-please-config.*.json:18-22`).

- **Web — GitHub Pages.**
  - Built with Astro, deployed by `.github/workflows/deploy-web.yml:1-69` on push to `main` (filtered by `paths: [VERSION, packages/web/**, pnpm-lock.yaml, pnpm-workspace.yaml, package.json, .github/workflows/deploy-web.yml]`) or via `workflow_dispatch`.
  - Uses `actions/configure-pages@v5` + `actions/upload-pages-artifact@v3` + `actions/deploy-pages@v4` (`:52-68`).
  - Published to `https://skill-organizer.sergiocarracedo.es` (CNAME at `packages/web/public/CNAME:1`, `site` in `astro.config.mjs:11`). Concurrency group `pages` with `cancel-in-progress: true` (`deploy-web.yml:25-28`).
  - The web build reads the repo `VERSION` and exposes it as `import.meta.env.PUBLIC_CLI_VERSION` so install snippets can show the current version (`packages/web/astro.config.mjs:8-16`).

- **Release-please plumbing (not user-facing, but orchestrates the three channels above):**
  - `.github/workflows/release-please.yml:1-49` — three jobs keyed on `github.ref_name` (`alpha`/`beta`/`main`) using `googleapis/release-please-action@v5`, each with its own `config-file`.
  - `.github/workflows/auto-merge-release-please.yml:1-72` — guarded auto-merge of release-please PRs (branch-name match, not cross-repo, not a prerelease targeting `main`).
  - Local pre-flight scripts: `scripts/release-alpha.sh`, `scripts/release-beta.sh`, `scripts/release-stable.sh`, sharing `scripts/release-common.sh`.
