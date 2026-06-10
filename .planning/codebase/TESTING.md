# Testing

Last mapped: from a fresh codebase read on the current `main` working tree.
Citations use `path:line` so they can be opened directly.

## Test Frameworks

### Go (`packages/cli/`)

- **Stdlib `testing` only.** No `testify`, no `gomock` / `mockery`, no
  `gotest.tools`. Confirmed by reading `packages/cli/go.mod` end-to-end:
  the only non-cobra / non-pterm / non-yaml / non-kardianos dependency
  in `go.mod` is `fsnotify`. No assertions library is imported in any
  test file.
- **`creack/pty`** is imported in `e2e_test.go:18` to drive the
  interactive TUI tests against a real pseudo-terminal.
- **`os/exec`** is used to build and execute the compiled binary in
  the e2e tests (`packages/cli/e2e_test.go:312-329`).
- **Module: `go 1.24.0`** (`packages/cli/go.mod:3`).

### Web (`packages/web/`)

- **Vitest 4.1.5** for unit tests — `packages/web/package.json:38`.
  Configured with `globals: true`, `environment: "node"`, and includes
  `test/**/*.test.ts` (`packages/web/vitest.config.ts:11-14`).
- **`@vitest/coverage-v8`** for coverage, with **95% thresholds** on
  `lines`, `statements`, `functions`, and `branches`
  (`packages/web/vitest.config.ts:15-24`).
- **Playwright 1.59.1** for e2e tests
  (`packages/web/package.json:28`, `:35`). Single chromium project,
  baseURL `http://127.0.0.1:4321`, webServer is `pnpm preview --host
  127.0.0.1 --port 4321` (`packages/web/playwright.config.ts:3-21`).
- **No Jest.** No Storybook test runner. No `nyc` / `istanbul`.

## Test File Layout

### Go

- `*_test.go` **co-located** with the source file, same package
  (e.g. `package cmd` for `cmd/*.go`, `package skills` for
  `internal/skills/*.go`).
- **No separate `tests/` or `internal_test/` package.** Every test
  file is in the same directory and uses the same package name as the
  source it tests.
- **End-to-end tests** for the binary live in a single file
  `packages/cli/e2e_test.go` (package `main_test`).
- **Test fixtures** (fake external CLI, fake lockfiles, fake fixtures)
  live in `packages/cli/testdata/`. Specifically,
  `testdata/fake-skills-cli.go` is invoked through a PATH shim by the
  e2e tests (`packages/cli/testdata/fake-skills-cli.go`,
  `packages/cli/e2e_test.go:412-420`).

Full inventory of `*_test.go` files observed (27 files):

```
packages/cli/e2e_test.go
packages/cli/cmd/context_test.go
packages/cli/cmd/editable_path_selector_test.go
packages/cli/cmd/prompt_test.go
packages/cli/cmd/skill_add_test.go
packages/cli/cmd/skill_check_updates_test.go
packages/cli/cmd/skill_delete_test.go
packages/cli/cmd/skill_overlap_test.go
packages/cli/cmd/skill_try_find_metadata_test.go
packages/cli/cmd/status_render_test.go
packages/cli/internal/agenttools/agenttools_test.go
packages/cli/internal/backup/backup_test.go
packages/cli/internal/config/discovery_test.go
packages/cli/internal/config/overlap_test.go
packages/cli/internal/config/path_test.go
packages/cli/internal/config/registry_roundtrip_test.go
packages/cli/internal/config/registry_test.go
packages/cli/internal/maintenance/maintenance_test.go
packages/cli/internal/mover/mover_test.go
packages/cli/internal/overlap/overlap_test.go
packages/cli/internal/remote/skillscli_test.go
packages/cli/internal/selfupdate/selfupdate_test.go
packages/cli/internal/service/service_test.go
packages/cli/internal/skills/frontmatter_test.go
packages/cli/internal/skills/scanner_test.go
packages/cli/internal/status/status_test.go
packages/cli/internal/sync/sync_test.go
packages/cli/internal/watch/watch_reload_test.go
packages/cli/internal/watch/watch_test.go
```

### Web

- `packages/web/test/**/*.test.ts` — Vitest unit tests
  (`packages/web/vitest.config.ts:14`).
- `packages/web/test/e2e/**/*.spec.ts` — Playwright e2e tests
  (`packages/web/playwright.config.ts:4`).
- Current concrete files:
  - `packages/web/test/docs.test.ts` (Vitest, 32 lines)
  - `packages/web/test/e2e/smoke.spec.ts` (Playwright, 35 lines)
- The `coverage/` and `dist/` directories are git-ignored
  (`packages/web/.gitignore:5-12`).

## Test Structure

### Go: conventions

- **Function naming**: `TestXxxYyy` in PascalCase, no underscores
  between the words, descriptive of the behavior under test.
  Examples:
  - `TestLoadResolvedProjectFallsBackToAgentsHome`
    (`packages/cli/cmd/context_test.go:11`).
  - `TestScanSourceStopsAtTerminalSkills`
    (`packages/cli/internal/skills/scanner_test.go:9`).
  - `TestDocumentManagedFieldsPreserveExtraFrontmatter`
    (`packages/cli/internal/skills/frontmatter_test.go:10`).
  - `TestResolveSkillsForDeleteSupportsWildcards`
    (`packages/cli/cmd/skill_delete_test.go:15`).
- **Table-driven tests are NOT the dominant pattern.** Most tests
  build the inputs inline and assert directly with `t.Fatalf`. Where
  multiple cases are tested in a single test, a `for _, want := range
  []string{…}` loop is preferred over a `tests := []struct{…}{…}`
  table (`packages/cli/internal/skills/frontmatter_test.go:36-54`,
  `packages/cli/cmd/status_render_test.go:19-23`).
- **Setup / teardown** uses `t.TempDir()` (auto-cleaned), `t.Setenv()`
  (auto-restored), and `t.Cleanup(func() { … })` for explicit
  restoration. No `TestMain`, no shared fixtures module.
- **No `t.Parallel()`** in unit tests, but it is used in the binary
  e2e tests:
  ```go
  func TestMoveUnmanagedBinary(t *testing.T) {
      t.Parallel()
      env := newCLIEnv(t)
      …
  }
  ```
  `packages/cli/e2e_test.go:74-76`. Each test gets its own `cliEnv`
  rooted in its own `t.TempDir()`, so parallel-safe isolation is
  achieved.
- **Failures use `t.Fatalf`** with format strings, not
  `t.Errorf`/`t.Errorf` chains. `t.Helper()` is called on every
  helper that wraps `t.Fatalf` so the failing line points at the
  caller — see `assertContains` in
  `packages/cli/e2e_test.go:661-666` and `loadUpdatesState` in
  `:564-572`.
- **Assertion helpers live in the test file itself** rather than in
  a shared helpers package. The four helpers in `e2e_test.go:661-691`
  are `assertContains`, `assertExists`, `assertNotExists`,
  `assertSymlinkTargetContains`. Other test files redefine
  `assertContains`-style helpers as needed (e.g.
  `internal/backup/backup_test.go`).

### Web: conventions

- **Vitest uses `describe` / `it` / `expect` / `vi`** explicitly
  imported, even though `globals: true` is on
  (`packages/web/vitest.config.ts:12`,
  `packages/web/test/docs.test.ts:4`).
- **Env stubbing with `vi.stubEnv`** — see
  `packages/web/test/docs.test.ts:6-9`:
  ```ts
  vi.stubEnv(
    "PUBLIC_CLI_VERSION",
    readFileSync(fileURLToPath(new URL("../../../VERSION", import.meta.url)), "utf8").trim(),
  );
  ```
  This is required because the Astro build bakes
  `import.meta.env.PUBLIC_CLI_VERSION` from the repo `VERSION` file
  (`packages/web/astro.config.mjs:14-16`), and the test reads it back
  to assert sync between the version pin and the helper output.
- **Path alias** `@/*` is used in imports
  (`packages/web/test/docs.test.ts:11-12`).
- **Playwright uses `test` / `expect`** with `async ({ page }) => …`
  callbacks. One test per file, grouped by file per concern. The
  current `test/e2e/smoke.spec.ts` has a single `test("homepage and
  docs smoke flow", …)` (35 lines, exercises the home page, hero
  CTAs, docs index, and one docs reference route).

## Mocking Approach

The Go side uses **package-level function variables** to inject
fakes, not interfaces or generated mocks. The pattern is:

1. Declare package-level vars that point at the real implementations
   next to the command function. Example
   (`packages/cli/cmd/skill_delete.go:20-27`):
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
2. In tests, reassign these vars to fakes that return hard-coded
   values. Example
   (`packages/cli/cmd/skill_delete_test.go:65-82`):
   ```go
   skillDeleteLoadResolvedLocation = func() (string, configpkg.Location, error) {
       return filepath.Join(root, ".skill-organizer.yml"), configpkg.Location{Source: source, Target: target}, nil
   }
   skillDeleteConfirm = func(prompt string, defaultValue bool) (bool, error) {
       return true, nil
   }
   ```
3. Restore via `t.Cleanup` (`packages/cli/cmd/skill_delete_test.go:83-90`).

Other examples of the same pattern:

- `skill_check_updates.go` exposes `collectUpdateCandidatesDeps` for
  the same kind of injection — see the test in
  `packages/cli/cmd/skill_check_updates_test.go:285-320` where a
  fake `fetchBundle` is supplied.
- `skill_overlap.go` similarly fakes `chooseToolFunc`,
  `launchPlanSession`, `printOverlapPromptFunc` in
  `packages/cli/cmd/skill_overlap_test.go`.

**Why this and not interfaces?** It avoids forcing a structural
interface on every collaborator, keeps the public API of the `cmd`
package simple, and means tests don't need to import
interface-implementing fakes. The downside is that the variables are
package-global, so they must always be restored in `t.Cleanup`.

For external processes, the e2e test suite uses a real fake
binary: `packages/cli/testdata/fake-skills-cli.go` is a small Go
program that implements the subset of the `skills` CLI the CLI
under test actually invokes. Tests add it to `PATH` via a shim
script written to a temp `binDir`
(`packages/cli/e2e_test.go:412-420`).

For interactive TUI tests, a real pseudo-terminal is opened with
`creack/pty`, the command is run, the test goroutine reads from the
PTY until a needle appears in the output, then writes a keystroke.
See `runInteractive` and `waitForOutput` in
`packages/cli/e2e_test.go:475-558`.

On the web side there is no mocking layer yet — both existing test
files are pure-function assertions over `src/lib/`.

## E2E Tests

### CLI binary e2e (`packages/cli/e2e_test.go`)

A single `e2e_test.go` file drives the full CLI as a compiled
binary. The `cliEnv` struct (`packages/cli/e2e_test.go:24-43`) is the
test harness:

- Builds the binary once per test with `go build -o binaryPath
  ./main.go` (`packages/cli/e2e_test.go:312-329`).
- Sets `HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, and a `PATH` that
  puts the fake `skills` shim first
  (`packages/cli/e2e_test.go:300-305`).
- Seeds an update-cache file dated `2100-01-01` so update checks do
  not self-update during the test.
- Provides `run`, `runRaw`, `runInteractive` helpers plus
  `assertContains`, `assertExists`, `assertNotExists`,
  `assertSymlinkTargetContains`.

The four currently-defined e2e tests cover the binary path:

- `TestMoveUnmanagedBinary` (`e2e_test.go:74`)
- `TestSkillDeleteWildcardBinary` (`e2e_test.go:90`)
- `TestSkillAddAndCheckUpdatesBinary` (`e2e_test.go:106`)
- `TestSkillTryFindMetadataBinary` / `…SkipsUnresolvedBinary`
  (`e2e_test.go:185`, `:231`)
- `TestSkillAddWithRealNpxSkillsSmoke` (`e2e_test.go:158`) — gated
  on `SKILL_ORGANIZER_E2E_REAL_NPX=1`, skips in `testing.Short()`,
  runs against `https://github.com/vercel-labs/agent-browser`. The
  fake `skills` shim is removed (`removeFakeSkillsShim`,
  `e2e_test.go:431-435`) so the real `npx skills` is used.

### Web e2e (`packages/web/test/e2e/smoke.spec.ts`)

A single Playwright test (`smoke.spec.ts:3`) drives the home page
and the docs index/reference flows. The `playwright.config.ts`
starts a `pnpm preview` server on `127.0.0.1:4321` automatically
(`packages/web/playwright.config.ts:10-15`).

There is no CLI e2e for the web side (the web build is just a
static site).

## Coverage State

- **CLI (Go)**: coverage is **not** measured. `go test ./...` in
  `ci.yml` has no `-coverprofile` flag and no codecov / coveralls
  upload.
- **Web (Vitest + v8)**: configured but not enforced in CI
  (`packages/web/vitest.config.ts:15-24`). The thresholds
  `lines/statements/functions/branches: 0.95` are defined, and
  `pnpm --filter web test` runs `vitest run --coverage`
  (`packages/web/package.json:14`), but
  `.github/workflows/ci.yml` does not run the web test or coverage
  step at all — only the web build is exercised in CI
  (`ci.yml:54-55`).
- **Web coverage artifact**: `packages/web/coverage/` exists on
  disk locally (contains `coverage-final.json`, `base.css`,
  `index.html`, plus `prettify.*` and `block-navigation.js` from
  the source-map preview). It is git-ignored
  (`packages/web/.gitignore:9-12`).
- **No `codecov.yml` or `.nycrc`** exists anywhere in the repo.
- **Honest uncertainty**: I did not run `go test -cover` or
  `vitest run --coverage` to capture current numeric coverage
  values; the numbers above are what the configured thresholds and
  CI surface would say if they were enabled.

## How to Run

All commands are run from the repo root unless noted.

### All tests (monorepo)

```bash
pnpm test
```

Resolves to `pnpm run test:cli && pnpm run test:web` — runs the
Go `go test ./...` for the CLI and Vitest (with coverage) for the
web in sequence (`package.json:13`).

### CLI — unit tests only

```bash
pnpm run test:cli
```

Equivalent to `sh -c 'cd packages/cli && go test ./...'`
(`package.json:14`). Runs **all** `*_test.go` files in
`packages/cli/`, including the binary e2e tests (because
`TestMoveUnmanagedBinary` etc. are matched by the default `go test`
pattern). To skip e2e explicitly, no script is wired — pass
`-short` and look for `t.Skip("…", testing.Short())` in
`e2e_test.go:159-164`.

### CLI — binary e2e only (subset, faster)

```bash
pnpm run test:cli:e2e
```

Runs only
`TestMoveUnmanagedBinary|TestSkillDeleteWildcardBinary|TestSkillAddAndCheckUpdatesBinary`
in `packages/cli/`. Defined in `package.json:15`. This is the
command that runs in the root `lefthook` pre-commit hook for CLI
globs (`lefthook.yml:1-10`).

### CLI — real `npx skills` smoke (network)

```bash
pnpm run test:cli:e2e:real-npx
```

Equivalent to
`SKILL_ORGANIZER_E2E_REAL_NPX=1 go test -run TestSkillAddWithRealNpxSkillsSmoke ./...`
in `packages/cli/` (`package.json:16`). Needs Node 24+ on PATH
because it shells out to `npx skills`. CI is opt-in via the
`run_real_npx_smoke` input on `.github/workflows/cli-e2e.yml:11-15`.

### CLI — both e2e lanes

```bash
pnpm run test:cli:e2e:all
```

Runs `test:cli:e2e` then `test:cli:e2e:real-npx` in sequence
(`package.json:17`).

### Web — unit tests with coverage

```bash
pnpm --filter web test
```

Runs `vitest run --coverage` from `packages/web/`
(`packages/web/package.json:14`). Output: V8 coverage in
`packages/web/coverage/`, with text/json/html reporters.

### Web — Vitest watch UI

```bash
pnpm --filter web test:ui
```

`packages/web/package.json:15`. Requires the `@vitest/ui` dev dep
listed at `package.json:31`.

### Web — Playwright e2e

```bash
pnpm --filter web test:e2e
```

Runs `playwright test` from `packages/web/`
(`packages/web/package.json:16`,
`packages/web/playwright.config.ts`). Playwright auto-starts
`pnpm preview` on port 4321 (`playwright.config.ts:10-15`).

### Lint and type-check

```bash
pnpm run lint          # oxlint for the web only
pnpm run type-check    # astro check for the web only
```

Both delegate to `pnpm --filter web …` (`package.json:9-12`). The
CLI has no lint script. CI runs `go build` and `go test` only
(`.github/workflows/ci.yml:33-39`).

### Hooks (run lefthook manually)

```bash
pnpm run hooks:install             # pnpm exec lefthook install
pnpm run hooks:run:pre-commit      # run pre-commit on staged files
pnpm run hooks:run:pre-commit:all  # run pre-commit on all files
```

`package.json:20-22`. Note the root `lefthook.yml` only triggers
on CLI-related globs; `packages/web/lefthook.yml` triggers on
every web-package change.
