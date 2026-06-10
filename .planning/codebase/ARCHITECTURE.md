# Architecture

System design for the `skill-organizer` monorepo at `/works/opensource/skill-organizer`. All citations use `path:line` so they can be opened directly in the editor.

## Overall Pattern

A **two-package monorepo** with one product and three delivery surfaces:

1. **Go CLI** (`packages/cli/`) — a single binary (`skill-organizer`) that organizes a nested source skill tree and exposes a flat first-level target folder that agent tools read. The Go module is declared at `packages/cli/go.mod:1` (`module github.com/sergiocarracedo/skill-organizer/cli`, `go 1.24.0`). Entry point: `packages/cli/main.go:10` calling `cmd.Execute()` from `packages/cli/cmd/root.go:32`.
2. **Static marketing / docs site** (`packages/web/`) — an Astro 6 project (`packages/web/package.json:21`) that renders at `https://skill-organizer.sergiocarracedo.es` (`packages/web/astro.config.mjs:11`). The site documents the CLI and embeds the canonical command surface that the CLI implements.
3. **npm wrapper** (`packages/cli/packages/npm/`) — a small CommonJS shim that downloads the matching Go binary from the GitHub release for the user's OS/arch and spawns it. `packages/cli/packages/npm/package.json:22-24` exposes the `skill-organizer` bin; the install hook is `packages/cli/packages/npm/scripts/postinstall.js:28-76`.

**Deployment model** is a 1:1 mapping per channel:
- **GitHub Releases** for the Go binary — `.github/workflows/release.yml:27-36` runs `goreleaser/goreleaser-action@v7` with `workdir: packages/cli` and `args: release --clean`. Archives are `tar.gz` (Linux/macOS) and `zip` (Windows). Homebrew tap `sergiocarracedo/homebrew-tap` is updated from GoReleaser's `brews` block.
- **npm registry** — `.github/workflows/publish-npm.yml:46-47` publishes `packages/cli/packages/npm/` with `--tag` decided from the version (`-alpha.*` → `alpha`, `-beta.*` → `beta`, else `latest`). Uses trusted publishing (`id-token: write` at `publish-npm.yml:9`), no `NPM_TOKEN` secret.
- **GitHub Pages** for the site — `.github/workflows/deploy-web.yml:55-69` uploads `packages/web/dist` to Pages.
- **Three release tracks** — `main`, `alpha`, `beta`. `release-please` is run per branch with three configs (`release-please-config.{alpha,beta,stable}.json`); `auto-merge-release-please.yml:51-62` enforces that no prerelease PR is auto-merged into `main`.

The CLI does not call the web and the web does not call the CLI. The web reads the same `VERSION` file that the CLI ships with: `packages/web/astro.config.mjs:8` reads `../../VERSION` and exposes it as `import.meta.env.PUBLIC_CLI_VERSION`, which the site renders via `packages/web/src/lib/cli-version.ts:1`. This is the only cross-package shared artifact, and it is file-based, not an API.

## Layers

### CLI layers (`packages/cli/`)

The Go module is split into three concentric layers, each with strict import direction:

```
main.go → cmd/* → internal/** (and cmd-shared helpers)
```

- **`main.go`** — process entry. `packages/cli/main.go:10-15` calls `cmd.Execute()` and exits 1 on error. The `version`/`commit`/`date` variables are package-level vars in `cmd/root.go:18-23`, populated by `-ldflags` (see `packages/cli/.goreleaser.yaml:33-35`).
- **`cmd/`** — the cobra command tree. One `newXxxCommand()` constructor per command, all wired in `packages/cli/cmd/root.go:94-105`. Subcommand groups (`skill`, `project`, `watched`, `service`) live as `newXxxCommand()` parents in `skill.go:5`, `project.go:5`, `watched.go:14`, `service.go:12`. Each command file is co-located with a `*_test.go` (e.g. `skill_delete.go` and `skill_delete_test.go`). Shared concerns: `context.go` (config resolution), `prompt.go` (interactive selectors), `logo.go` (ASCII/ANSI logo), `editable_path_selector.go` (path autocomplete), `aliases.go` (root-level command aliases), `status_render.go` (status rendering).
- **`internal/`** — domain packages. Each owns one concern and exposes either functions or a small surface area. All packages are unexported and only consumed by `cmd/`:
  - `internal/config/` — `Location` (project config), `AppConfig` (registry + service config), path resolution, registry load/save, update cache, discovery. See `internal/config/{config,registry,path,cache,discovery}.go`.
  - `internal/skills/` — source tree scanning (`ScanSource`, `ResolveSourceSkill`, `ResolveSourceSkillTarget` in `scanner.go:22,95,151`), YAML frontmatter parsing and rewrite (`frontmatter.go`).
  - `internal/sync/` — the actual reconciler (`sync.go:23 Run`) and the per-target manifest (`manifest.go`).
  - `internal/status/` — read-only reporter (`status.go:77 Build`).
  - `internal/agenttools/` — registry of supported agent CLIs (Claude Code, Codex, OpenCode, Cursor, Antigravity) with binary detection (`agenttools.go:24-77,87-101`).
  - `internal/overlap/` — overlap analysis prompt builder, command runner, and `process_unix.go`/`process_windows.go` for cross-platform process control.
  - `internal/remote/` — `skills` CLI integration (the binary that actually fetches skills from skills.sh). `skillscli.go:25-30` defines `Sandbox`; `skillscli.go:97-105` is `DetectSkillsCLI`; `skillscli.go:148-168` is `NewSandbox`; `skillscli.go:397-415` is `FetchSkillBundle`; `skillscli.go:417-448` is `FindSkills`.
  - `internal/backup/` — `MoveSkill` to `.old/` with metadata, `PruneExpired` for retention (`backup.go`).
  - `internal/mover/` — plan/apply for moving unmanaged target entries into the source tree (`mover.go:20,42,64`).
  - `internal/watch/` — `Runner` (the fsnotify loop, `watch.go:23-52`) and the periodic update check (`watch.go:112-195`).
  - `internal/service/` — `kardianos/service` plumbing plus a Linux `systemctl --user` path (`service.go:155-207`).
  - `internal/selfupdate/` — GitHub release check and the install-method detector (`selfupdate.go:167-192`).
  - `internal/maintenance/` — backup-GC and update reminders (`maintenance.go:19,55`).
  - `internal/logging/` — `Logger` interface, `NewStd` / `NewForService` / `LoadForRegistry` (`logging.go:49,56,69`).
  - `internal/versionfmt/` — display helpers for version strings and dates (`versionfmt.go`).
- **`testdata/fake-skills-cli.go`** — a fake `skills` binary used by the e2e tests. Detected via env vars (`SKILL_ORGANIZER_FAKE_SKILLS_FIXTURES`, `SKILL_ORGANIZER_FAKE_SKILLS_WORKDIR`, etc., `testdata/fake-skills-cli.go:123,243,254,322-323`) and shimmed onto `PATH` from `e2e_test.go`.
- **`packages/cli/packages/npm/`** — the published npm shim. Not part of the Go build; `pnpm-workspace.yaml:3` includes it as a pnpm workspace member so the lockfile tracks the directory.

There is no `pkg/` directory. All shared Go code lives under `internal/`. The Go module has no `cmd/<subcommand>/` subdirectories — every command is a flat `cmd/<name>.go` file.

### Web layers (`packages/web/src/`)

Astro 6 site organized as `pages → views → components → data/lib`, with content collections for the docs:

- **`src/pages/`** — file-based routes. Three files: `index.astro` (home, `pages/index.astro:15-28` composes 9 home sections from `@/views/home/*`), `docs/index.astro` (docs landing, two sections: guides and reference), and `docs/[section]/[slug].astro` (dynamic MDX page, `[section]/[slug].astro:8-18` uses `getStaticPaths()` over the `docs` collection).
- **`src/views/`** — page-level section composition. `views/home/<section>/<Section>Section.astro` for home, `views/docs/components/DocsLayout.astro` for docs.
- **`src/components/`** — reusable UI: `Header.astro`, `Footer.astro`, `SectionHeader.astro`, `TerminalFrame.astro`, `CodePanel.astro`, `CopyButton.astro`, `AgentToolLogo.astro`, `LogoMark.astro`, `GitHubStarButton.astro`, `SectionShell.astro`. `components/logos/` is a sub-folder of brand mark components (`Antigravity.astro`, `Apple.astro`, `Claude.astro`, `Codex.astro`, `Copilot.astro`, `Cursor.astro`, `Linux.astro`, `Opencode.astro`, `Pii.astro`).
- **`src/layouts/SiteLayout.astro`** — the single HTML shell (`<head>`, `data-reveal` IntersectionObserver, `page-noise`/`page-glow` overlays).
- **`src/data/`** — typed static content. `commands.ts` (the canonical command surface; `commands.ts:1-9` defines `CommandGroup`, `commands.ts:16-178` is the table), `home.ts` (install methods, terminal scenarios, feature cards, FAQ, overlap demo groups). This file is the single source of truth for what the site advertises.
- **`src/lib/`** — kebab-case helpers. `cli-version.ts` (read build version), `with-base.ts` (prefix asset paths with `import.meta.env.BASE_URL`), `docs.ts` (docs section helpers and `getCommandBySlug`).
- **`src/content/`** — Astro content collections. `content.config.ts:5-15` defines the `docs` collection with a Zod schema (`title`, `description`, `section: "guides" | "reference"`, `order`, optional `commandSlug`/`summary`). `content/docs/guides/*.mdx` and `content/docs/reference/*.mdx` are the source files.
- **`src/icons/`** — Astro Icon sprite sources. Three sub-collections: `Solid/`, `outline/`, `tech/`. Wired into Astro via `astro-icon` integration (`astro.config.mjs:4,12`).
- **`src/styles/global.css`** — Tailwind CSS v4 entry, imported from `SiteLayout.astro:4`.
- **`public/`** — static assets served as-is: `CNAME`, `favicon.svg`, `logo_color2.png`, plus `public/hero/` (demo video) and `public/og/` (Open Graph social image).

## Data Flow

End-to-end path for the canonical user action **`skill-organizer add <source>`** (e.g. `add https://github.com/terrylica/cc-skills`). This is the most layered path; everything else is a strict subset.

1. **Process start** — `packages/cli/main.go:10` calls `cmd.Execute()` (`cmd/root.go:32-48`). The root context is wrapped with `signal.NotifyContext` for `os.Interrupt`/`SIGTERM` (`root.go:33`).
2. **Persistent pre-run** — `cmd/root.go:72-85` `PersistentPreRun` runs once for any non-root subcommand: prints the CLI header, then calls `maintenancepkg.MaybeRunBackupGC` (12-hour-cadence prune of `~/.config/skill-organizer/.old/`) and `selfupdatepkg.MaybeNotify` (24-hour-cadence GitHub release check). It also calls `maintenancepkg.MaybeNotifySkillUpdates` unless the command is `check-updates`. The maintenance package calls a `IsServiceRunningFunc` injected from `cmd/root.go:62-68` so it can skip the reminder while the service is up.
3. **Cobra dispatch** — `add` is registered as a root-level alias for `skill add` by `cmd/root.go:107-115` (only `add`, `delete`, `enable`, `disable`, `check-updates` get root aliases; `move-unmanaged`, `try-find-metadata`, and `check-overlap` are nested under `skill` only). Dispatch lands in `cmd/skill_add.go:40 newSkillAddCommand`.
4. **Config resolution** — `cmd/skill_add.go:49` calls `loadResolvedLocation` (`cmd/context.go:72-79`) which delegates to `loadResolvedProject` (`context.go:46-70`). The resolver first honors `--config`; if absent, it walks up the directory tree via `configpkg.DiscoverFrom` (`internal/config/discovery.go:57-83`) looking for `<target-parent>/.skill-organizer.yml`. If nothing is found, it falls back to `HomeFallbackTarget` (`discovery.go:115-136`) which scans for `~/.agents/skills`, `~/.claude/skills`, `~/.codex/skills`, `~/.agent/skills` and synthesizes a default source at `<parent>/skills-organized`.
5. **Source scan** — `cmd/skill_add.go:54` calls `skills.ScanSource(location.Source)` (`internal/skills/scanner.go:22-40`). The scanner walks the source tree depth-first; the first directory containing `SKILL.md` is a "terminal skill" (see `scanner.go:42-88` and `UBIQUITOUS_LANGUAGE.md:11`). For each terminal skill it records `Dir`, `SkillFile`, `RelativePath`, and `FlattenedName = path.ReplaceAll("/", "--")` (`scanner.go:90-93`). Flattening collisions are an error.
6. **Detect `skills` CLI** — `cmd/skill_add.go:60` calls `remotepkg.DetectSkillsCLI` (`internal/remote/skillscli.go:97-105`), which tries `which skills` and falls back to `npx skills`.
7. **Build a sandbox** — `cmd/skill_add.go:68` calls `newSkillAddSandbox` → `remotepkg.NewSandbox` (`skillscli.go:148-168`). The sandbox is a temp directory with `project/` and `home/` subdirs. All `skills` CLI calls run inside it with a forced non-color env (`skillscli.go:133-146`).
8. **Invoke `skills add`** — `cmd/skill_add.go:103 sandbox.Run("add", source, "-y", "--copy", …)`. The `skills` binary installs the requested skills into the sandbox's `~/.agents/skills/`.
9. **Diff before/after** — `cmd/skill_add.go:88 sandbox.InstalledSkills()` (snapshot of pre-install) and `cmd/skill_add.go:108 sandbox.InstalledSkills()` (post-install). The two are diffed in `cmd/skill_add.go:112 newlyInstalledSkills` (`skill_add.go:301-310`) to identify only the new skills.
10. **Bundle each new skill** — for each new skill, `cmd/skill_add.go:119 sandbox.LoadInstalledBundle` (`skillscli.go:264-315`) walks the installed directory and reads `metadata.json` and `skills-lock.json` to populate a `SkillSummary` (`skillscli.go:53-64`) with `Source`, `SourceURL`, `SourceType`, `Version`, `Hash`, `RepoSkillPath`.
11. **Choose target folders in source** — `cmd/skill_add.go:135 chooseSkillAddTargets` calls `selectImportedSkillTargets` (`skill_add.go:240-266`), which runs the `editablePathSelector` (in `cmd/editable_path_selector.go`) so the user can rename the source path for each new skill.
12. **Confirm reinstalls** — for any skill that already exists in source (`existingNames[installed.Name]`), the user is prompted to reinstall; on yes, `backuppkg.MoveSkill` (`internal/backup/backup.go:31-62`) moves the previous source skill into `~/.config/skill-organizer/.old/<stamp>-<flattenedName>/` with a `.skill-organizer-backup.yml` metadata sidecar.
13. **Write the bundle to source** — `cmd/skill_add.go:175 writeImportedBundle` (`skill_add.go:312-326`) walks the bundle files and writes them to `targetSkill.Dir`.
14. **Rewrite source frontmatter** — `cmd/skill_add.go:179 skills.RewriteManagedFieldsWithMetadata` (`internal/skills/frontmatter.go:317-333`) sets the canonical flattened name as `name` and writes/updates `metadata.skill-organizer.{source, source-type, installed-version, installed-at, repo-skill-path, last-updated-at}`.
15. **Sync** — `cmd/skill_add.go:193 runSkillAddSync` → `syncpkg.Run(location)` (`internal/sync/sync.go:23-115`). The sync:
    - calls `skills.ScanSource` to list all terminal skills,
    - for each, calls `skills.LoadDocument` to read its frontmatter, and `skills.RewriteManagedFields` to keep source metadata in sync (`sync.go:48`),
    - skips disabled skills,
    - creates, updates, or removes symlinks in the target folder (`sync.go:117-154 reconcileTargetEntry`),
    - removes stale managed entries (`sync.go:101-108`),
    - writes the new `.skill-organizer.manifest.yml` (`internal/sync/manifest.go:46-62`).
16. **Result printing** — `cmd/skill_add.go:197 printSyncResult` (defined in `cmd/status_render.go`) emits `Enabled N, Disabled N, Created N, Updated N, Removed N` lines and ends the command.

The end state: the user can drop into `~/.agents/skills/<flattenedName>` (a symlink) and the agent tool that reads `~/.agents/skills` sees the skill, while the human-edited source of truth remains in `skills-organized/<possibly-nested>/<name>/SKILL.md`.

## Key Abstractions

The Go layer's domain model is concentrated in a handful of files. There are no interfaces in the Go layer's domain code — the unit of abstraction is the struct plus its constructor functions.

| Type | File | Lines | Purpose |
| --- | --- | --- | --- |
| `configpkg.Location` | `internal/config/config.go` | 9-26 | `Source` and `Target` paths; `Validate` enforces they differ and both are non-empty. |
| `configpkg.WatchRegistry` / `AppConfig` | `internal/config/config.go` | 28-50 | Watched project paths, service config (log level), overlap config, backup config — all stored in one YAML file at `~/.config/skill-organizer/skill-organizer.yml`. |
| `configpkg.SkillUpdateRecord` | `internal/config/cache.go` | 37-45 | One pending skill update, with `RelativePath`, `FlattenedName`, `InstalledVersion`, `LatestVersion`, `Source`, `RepoSkillPath`, `CheckedAt`. |
| `skills.Skill` | `internal/skills/scanner.go` | 15-20 | The on-disk terminal skill: `Dir`, `SkillFile`, `RelativePath`, `FlattenedName`. |
| `skills.ManagedMetadata` | `internal/skills/frontmatter.go` | 15-25 | The `metadata.skill-organizer` block: `OriginalName`, `SourceRelativePath`, `Disabled`, `Source`, `SourceType`, `InstalledVersion`, `InstalledAt`, `RepoSkillPath`, `LastUpdatedAt`. |
| `skills.Document` | `internal/skills/frontmatter.go` | 27-30 | A `SKILL.md` parsed into a YAML `MappingNode` + body. Methods: `Name()`, `Description()`, `Body()`, `ManagedMetadata()`, `SetManagedFields`, `WriteTo`. |
| `sync.Result` | `internal/sync/sync.go` | 13-21 | Outcome of a sync: source skills, enabled, disabled, and `Created/Updated/Removed` lists. |
| `sync.Manifest` | `internal/sync/manifest.go` | 15-17 | The `.skill-organizer.manifest.yml` content: `map[flattenedName]relativePath`. |
| `status.SkillStatus` | `internal/status/status.go` | 24-33 | Per-skill state for the `status` command. State values: `synced`, `disabled`, `missing-target`, `broken-link`, `drifted` (`status.go:14-22`). |
| `status.Report` / `Summary` | `internal/status/status.go` | 35-49 | Status output: `Skills`, `Unmanaged`, plus a computed `Summary`. |
| `agenttools.Tool` | `internal/agenttools/agenttools.go` | 10-17 | Supported agent tool descriptor: `ID`, `Name`, `Binaries []string`, `Description`, `Args(prompt) []string`, `PlanArgs(prompt) []string`. |
| `remote.SkillSummary` | `internal/remote/skillscli.go` | 53-64 | A skill's upstream identity and metadata: `Provider`, `ID`, `Name`, `Source`, `SourceURL`, `SourceType`, `Version`, `VersionDate`, `Hash`, `RepoSkillPath`. |
| `remote.SkillBundle` | `internal/remote/skillscli.go` | 66-70 | A skill's metadata + a list of `File{Path, Contents}`. Returned from `FetchSkillBundle` and `LoadInstalledBundle`. |
| `remote.SkillsUpdate` | `internal/remote/skillscli.go` | 77-82 | A detected update: installed vs latest bundle side by side. |
| `remote.Sandbox` | `internal/remote/skillscli.go` | 25-30 | The isolated temp directory used to run the external `skills` CLI. |
| `overlap.Report` | `internal/overlap/overlap.go` | 28-32 | Overlap analysis result: `Summary`, `Groups` (with `Score` 0-100 and `OverlapType` = `duplicate|partial|adjacent`), `Recommendations`. |
| `overlap.SkillInfo` | `internal/overlap/overlap.go` | 20-26 | The subset of a `skills.Skill` that the overlap prompt needs. |
| `backup.Metadata` | `internal/backup/backup.go` | 15-21 | YAML sidecar written to `.old/<stamp>-<name>/.skill-organizer-backup.yml` on delete or update. |
| `logging.Logger` | `internal/logging/logging.go` | 23-28 | The only Go interface in the runtime: `Errorf`/`Warnf`/`Infof`/`Debugf`. |

Web domain types are limited and live in `src/data/`:

| Type | File | Lines | Purpose |
| --- | --- | --- | --- |
| `CommandGroup` | `src/data/commands.ts` | 1-9 | A single command's marketing surface: `slug`, `title`, `command`, `summary`, `whyItMatters`, optional `flags`, `examples`. The `slug` matches the cobra command name and is what the docs `commandSlug` frontmatter points to. |
| `InstallMethod` | `src/data/home.ts` | 1-10 | A single install channel (npm/pnpm/brew/binary). |
| `TerminalScenario` | `src/data/home.ts` | 66-72 | A scripted terminal recording shown on the home page (typed `TerminalEvent[]`). |
| `FeatureCard` / `OverlapGroupDemo` | `src/data/home.ts` | 74-89 | Static content for the home page sections. |

## Dependency Injection

The Go code uses a deliberately small, consistent DI strategy: **package-level function variables**, swapped in tests. There is no DI container, no struct constructor injection outside of the Cobra commands, and no interface-based mocking library.

- **Test-injectable function variables** — each `cmd/` file that does I/O declares a block at the top:
  ```go
  var (
      skillDeleteLoadResolvedLocation = loadResolvedLocation
      skillDeleteConfirm              = confirm
      skillDeleteMoveToBackup         = backuppkg.MoveSkill
      skillDeleteRegistryPath         = configpkg.RegistryPath
      skillDeleteLoadBackupConfig     = configpkg.LoadBackupConfigOrDefault
      runSkillDeleteSync              = syncpkg.Run
  )
  ```
  See `cmd/skill_delete.go:20-27`. The pattern is identical in `cmd/skill_add.go:27-38`, `cmd/skill_check_updates.go:54-61`, `cmd/skill_overlap.go:21-76`, `cmd/skill_try_find_metadata.go:26-34`, and others. Tests reassign these in `t.Cleanup` blocks (e.g. `cmd/skill_delete_test.go:59-90`).
- **Sandbox interface for `skill add`** — `cmd/skill_add.go:20-25` defines an inline `skillAddSandbox interface` with `Close`, `InstalledSkills`, `Run`, `LoadInstalledBundle`, satisfied by `*remote.Sandbox`. The package-level `newSkillAddSandbox` (`skill_add.go:30`) is swapped in tests.
- **Command runner for `overlap`** — `cmd/skill_overlap.go:43-45` uses `var commandRunner = runCommand` (set inside `overlap` package; the `overlap.Run` call goes through it). `cmd/skill_overlap.go:50-60` then has its own `launchPlanSession` and prompt-construction indirection so tests can stub the launch and the spinner separately.
- **Pluggable process control** — `internal/overlap/process_unix.go:10-22` defines `configureInterruptHandling` and `interruptProcessTree`; `internal/overlap/process_windows.go` provides the Windows version. Build-tag gated.
- **Logger interface** — `internal/logging/logging.go:23-28` defines `Logger` (4 methods). `cmd/watch.go:26` constructs a `loggingpkg.NewStd` or `LoadForRegistry` and passes it to `watchpkg.New(registryPath, logger)` (`internal/watch/watch.go:34-52`). The service passes `servicepkg.SystemLogger` through `NewForService` (`logging.go:56-67`).
- **Service abstraction** — `internal/service/service.go:38-56` wraps `kardianos/service` and exposes `Program` (`service.go:24-28`) with `Start`/`Stop`. The CLI hooks into it via `servicepkg.Control` (`service.go:93-138`) and `IsRunning` (`service.go:295-301`).
- **No struct constructor injection** — internal packages accept their inputs as function arguments, not via constructors. `syncpkg.Run(location)` is the canonical example. This keeps internal packages stateless and side-effect-free except for the file system.

Web DI is the normal Astro/MDX pattern: components receive props, `getStaticPaths` derives per-route props (`src/pages/docs/[section]/[slug].astro:8-18`), and `src/lib/docs.ts:30-32 getCommandBySlug(slug)` resolves content from `src/data/commands.ts` at build time.

## Cross-Package Communication

The CLI and the web share exactly **one** artifact: the `VERSION` file at the repo root.

- **Read by the CLI** — baked in at build time via `-ldflags`. The GoReleaser config (`packages/cli/.goreleaser.yaml:33-35`) injects `-X 'github.com/sergiocarracedo/skill-organizer/cli/cmd.version=$(VERSION)'` etc. The npm package version is pinned in `packages/cli/packages/npm/package.json:3` and bumped by `release-please` (`release-please-config.stable.json`'s `$.version` jsonpath).
- **Read by the web** — at build time. `packages/web/astro.config.mjs:8` does `readFileSync(resolve(import.meta.dirname, "../../VERSION"), "utf8").trim()` and exposes it via `vite.define: { "import.meta.env.PUBLIC_CLI_VERSION": JSON.stringify(cliVersion) }` (`astro.config.mjs:14-16`). Consumed by `src/lib/cli-version.ts:1` and rendered in `src/views/home/hero/HeroSection.astro:21` as `CLI {cliVersion}`. The test in `test/docs.test.ts:19-21` asserts the in-page version equals the file's value.

There is **no generated shared code**, no OpenAPI client, and no schema synchronization. The web's `CommandGroup[]` (`src/data/commands.ts:16-178`) is hand-curated from the same UBUNTIOUS_LANGUAGE that drives the Go code, but the two are kept in sync by hand and the test in `test/docs.test.ts:23-27` only asserts the `skill` command string.

The web **does not depend on the CLI** at runtime. The site is a pure static build (`pnpm --filter web build` produces `packages/web/dist`, uploaded by `deploy-web.yml:55-58`). No CLI process is invoked during the web build. The CLI **does not depend on the web** at runtime either; the only outbound HTTP the CLI makes is to the GitHub Releases API (`internal/selfupdate/selfupdate.go:244-284`) and to the `skills` CLI's GitHub-backed upstream.
