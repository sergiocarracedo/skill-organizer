# Code Conventions

Last mapped: from a fresh codebase read on the current `main` working tree. The
conventions below are observed in `packages/cli/` and `packages/web/`. Citations
use `path:line` so they can be opened directly.

## Code Style

### Linters and formatters in use today

- **Go (`packages/cli/`)**: no project-level `golangci.yml`, no `.golangci-lint`,
  and no `testify` dependency in `packages/cli/go.mod`. The CI pipeline runs
  only `go test ./...` and `go build ./...` against the Go module —
  see `.github/workflows/ci.yml:33-39`. The linter story for the CLI is
  therefore "stdlib vet + go build + go test", not golangci.
- **Web (`packages/web/`)**: `oxlint` is the linter and Prettier is the
  formatter in active use. The repo contains both `.oxlintrc.json` and
  `.oxfmt.json`, but only `oxlint` and Prettier are wired into scripts and
  hooks. See `packages/web/package.json:10-12` and
  `packages/web/lefthook.yml:3-7`.
  - `.oxlintrc.json` enables three plugins and no rules:
    ```json
    {
      "$schema": "https://raw.githubusercontent.com/oxc-project/oxc/main/npm/oxlint/configuration_schema.json",
      "plugins": ["typescript", "unicorn", "jsx-a11y"],
      "rules": {}
    }
    ```
    The plugins are loaded but no individual rules are configured — the
    default rule set is in effect. `packages/web/.oxlintrc.json:1-5`.
  - `.oxfmt.json` exists but is **not** wired into any script. The
    `format`/`format:check` scripts call `prettier` directly, not `oxfmt`
    (`packages/web/package.json:11-12`). Treat the `.oxfmt.json` file as
    aspirational until the scripts change.
  - Prettier config: `semi: false`, `singleQuote: true`,
    `jsxSingleQuote: false`, `trailingComma: "all"`, `printWidth: 100`,
    `arrowParens: "always"`. `packages/web/.oxfmt.json:2-7` matches the
    Prettier equivalents that would be used. No `.prettierrc` file exists,
    so these are the effective values when the script reads them.
- **Commit messages**: `commitlint` with the conventional config
  (`packages/web/commitlint.config.js:1-3`). CI enforces
  `^(feat|fix|docs|refactor|test|build|ci|chore)(\([^)]+\))?(!)?: .+` for PR
  titles — `.github/workflows/ci.yml:20`.
- **Lefthook pre-commit hooks**:
  - Root `lefthook.yml` runs **only on CLI-related changes** (glob on
    `packages/cli/**`, `package.json`, `pnpm-lock.yaml`, the workflow file,
    and `lefthook.yml` itself). The single command is
    `pnpm run test:cli:e2e` — `lefthook.yml:1-10`.
  - `packages/web/lefthook.yml` runs lint, format, type-check, and test on
    every web-package change:
    ```yaml
    pre-commit:
      commands:
        lint:        { run: pnpm oxlint . }
        format:      { run: pnpm format:check }
        type-check:  { run: pnpm astro check }
        test:        { run: pnpm vitest run }
    commit-msg:
      commands:
        commitlint:  { run: pnpm commitlint --edit {1} }
    ```
  - The hooks are installed via `pnpm prepare` (which runs
    `pnpm exec lefthook install`) — `package.json:23`.

## Naming Patterns

### Go (`packages/cli/`)

- **Functions and methods**: `camelCase`. Exported constructors for cobra
  commands follow `newXxxCommand()` returning `*cobra.Command` (e.g.
  `newSkillCommand()` in `packages/cli/cmd/skill.go:5`,
  `newSkillDeleteCommand()` in `packages/cli/cmd/skill_delete.go:29`,
  `newCheckUpdatesCommand()` in `packages/cli/cmd/skill_check_updates.go:63`).
- **Types**: `PascalCase` (structs, interfaces, typedefs) — `cliEnv`,
  `Logger`, `Location`, `SkillBundle`, `LevelLogger`, `SkillsCLIRunner`
  (`packages/cli/internal/logging/logging.go:14-47`,
  `packages/cli/internal/remote/skillscli.go:21-89`).
- **Constants**: `PascalCase`. `LevelError`, `LevelWarn`, `LevelInfo`,
  `LevelDebug` (`packages/cli/internal/logging/logging.go:16-21`),
  `FileName` in `internal/config`.
- **Cobra command names**: lowercase, single word where possible
  (`Use: "skill"`, `Use: "sync"`, `Use: "onboard"`). Subcommand paths
  documented in `Use:` are also lowercase with `<>` for required args
  (`Use: "delete <skill-path-or-pattern>"`,
  `packages/cli/cmd/skill_delete.go:35`).
- **Aliases**: lower-case (`Aliases: []string{"remove", "rm"}` —
  `packages/cli/cmd/skill_delete.go:36`).
- **Package-level "swappable" function variables** (used to inject test
  doubles — see Error Handling and TESTING.md for details): lowerCamelCase
  with the command name as a prefix, e.g. `skillDeleteLoadResolvedLocation`,
  `skillDeleteConfirm`, `skillDeleteMoveToBackup`,
  `skillDeleteRegistryPath`, `skillDeleteLoadBackupConfig`,
  `runSkillDeleteSync` (`packages/cli/cmd/skill_delete.go:20-27`).
- **Sentinel errors**: `var ErrXxx = errors.New(...)`. Only one seen in
  the codebase: `ErrConfigNotFound` in
  `packages/cli/internal/config/discovery.go:15`.
- **Imports are grouped stdlib/third-party/internal**, and internal
  packages are aliased to avoid name collisions with `cmd` (e.g.
  `configpkg`, `backuppkg`, `syncpkg`, `maintenancepkg`,
  `selfupdatepkg`, `servicepkg`, `statuspkg`, `remotepkg`). Examples:
  `packages/cli/cmd/skill_delete.go:11-17`,
  `packages/cli/cmd/root.go:11-15`,
  `packages/cli/cmd/context_test.go:8`,
  `packages/cli/cmd/skill_check_updates_test.go` (uses `remotepkg`).

### TypeScript / TSX / Astro (`packages/web/src/`)

- **TypeScript strict mode** via `astro/tsconfigs/strict` —
  `packages/web/tsconfig.json:2`.
- **Path alias** `@/*` → `src/*` for both TS and Vitest
  (`packages/web/tsconfig.json:6-8`,
  `packages/web/vitest.config.ts:7-9`).
- **Astro components** use `PascalCase.astro` filenames
  (`HeroSection.astro`, `Header.astro`, `SectionHeader.astro`,
  `DocsLayout.astro`).
- **Lib helpers** use `kebab-case.ts` filenames (`cli-version.ts`,
  `with-base.ts`, `docs.ts`).
- **Single quotes, no semicolons, trailing commas everywhere, 100 col
  width, always-arrow-parens** (the `.oxfmt.json` values
  `packages/web/.oxfmt.json:2-7`). Imports use explicit named imports, e.g.
  `import { defineConfig } from "vitest/config";`
  (`packages/web/vitest.config.ts:3`),
  `import { describe, expect, it, vi } from "vitest";`
  (`packages/web/test/docs.test.ts:4`).
- **Test files**:
  - `test/**/*.test.ts` for unit (Vitest)
    (`packages/web/vitest.config.ts:14`).
  - `test/e2e/**/*.spec.ts` for Playwright e2e
    (`packages/web/playwright.config.ts:4`).

## Error Handling

- **Construction**: `fmt.Errorf("…: %w", err)` with `%w` for wrapping. The
  wrapped verb goes **first**, the underlying error second. Examples:
  - `return nil, fmt.Errorf("read installed skills directory: %w", err)`
    (`packages/cli/internal/remote/skillscli.go:237`).
  - `return "", fmt.Errorf("resolve working directory: %w", err)`
    (`packages/cli/cmd/skill_overlap.go:484`).
  - `return fmt.Errorf("create backup root: %w", err)`
    (`packages/cli/internal/backup/backup.go:40`).
- **Plain (non-wrapping) errors**: `fmt.Errorf("…")` for ad-hoc errors
  without an underlying cause. Examples:
  - `return fmt.Errorf("aborted")` (user pressed Ctrl+C / declined
    confirm) — appears in `cmd/skill_check_updates.go:329`,
    `cmd/editable_path_selector.go:135`, `cmd/skill_delete.go:72`,
    `cmd/watched.go:141`, `cmd/onboard.go:163`, `cmd/remove.go:28`.
  - `return fmt.Errorf("interrupted")` for context-cancelled paths.
  - `return nil, fmt.Errorf("no skills matched %q", trimmed)`
    (`packages/cli/cmd/skill_delete.go:152`).
  - `return fmt.Errorf("--to requires exactly one unmanaged target entry")`
    (`packages/cli/cmd/move_unmanaged.go:57`).
- **Sentinel errors**: only one in the codebase —
  `var ErrConfigNotFound = errors.New("project config not found")`
  (`packages/cli/internal/config/discovery.go:15`). Tests use
  `errors.Is(err, configpkg.ErrConfigNotFound)` to detect it
  (`packages/cli/internal/config/discovery_test.go:35`,
  `packages/cli/cmd/context.go:32`).
- **Return shape**: errors are returned as the last return value, never
  thrown. Cobra command bodies are `func(_ *cobra.Command, args []string)
  error { … }` (`RunE:`, never `Run:`). When a command errors, `RunE`
  returns the error; `rootCmd.SilenceUsage = true; rootCmd.SilenceErrors
  = true` are set so cobra does not re-print usage on every error
  (`packages/cli/cmd/root.go:91-92`).
- **Top-level printing**: `main.go` prints via `pterm.Error.Printfln`
  and `os.Exit(1)` —
  `packages/cli/main.go:11-14`.
- **Function-variable dependency injection** for swappable side effects
  in `cmd` files. Example pattern from `skill_delete.go:20-27`:
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
  Tests reassign these package-level vars to fakes and restore them via
  `t.Cleanup` (`packages/cli/cmd/skill_delete_test.go:59-90`). This is the
  established DI/mocking pattern — see TESTING.md for details.

## Logging

- **Logger abstraction**: defined in `packages/cli/internal/logging/logging.go`.
  ```go
  type Logger interface {
      Errorf(format string, args ...any)
      Warnf (format string, args ...any)
      Infof (format string, args ...any)
      Debugf(format string, args ...any)
  }
  ```
  `packages/cli/internal/logging/logging.go:23-28`.
- **Constructors**:
  - `logging.NewStd(level string) Logger` — writes to `os.Stdout` via
    `log.New(os.Stdout, "skill-organizer ", log.LstdFlags)`.
    `packages/cli/internal/logging/logging.go:49-54`.
  - `logging.NewForService(level string, svc servicepkg.Service) Logger` —
    wires the logger to the kardianos service's `SystemLogger`/`Logger`
    if available, else falls back to `NewStd`.
    `packages/cli/internal/logging/logging.go:56-67`.
  - `logging.LoadForRegistry(registryPath, svc) Logger` — reads the
    configured `LogLevel` from the registry file (with a fallback
    warning). `packages/cli/internal/logging/logging.go:69-77`.
- **Levels**: `LevelError` < `LevelWarn` < `LevelInfo` < `LevelDebug`,
  validated against `configpkg.IsValidLogLevel`
  (`packages/cli/internal/config/config.go:119`,
  `packages/cli/internal/logging/logging.go:14-21`).
- **Format strings**: lowercase `level` token prefixed to every line —
  `ERROR: …`, `WARN: …`, `INFO: …`, and `DEBUG: …`
  (`packages/cli/internal/logging/logging.go:123-145`).
- **What NOT to log** (inferred from the codebase):
  - Do not log secrets, API keys, or token material — none of the
    service log fields carry credentials, and the config layer exposes
    only paths and counts.
  - Do not log raw frontmatter / SKILL.md content. Tests use
    `t.Fatalf("…\n%s", err, string(output))` only for build/test
    diagnostics, never via the logger.
  - Do not introduce a third-party logger (zap, zerolog, logrus). The
    stdlib `log.Logger` wrapped in the `Logger` interface is the
    intentional choice. The interface is small (4 methods) and easy to
    keep.

## Validation

- **Cobra `Args` validators** on every command that takes positional
  args. Observed values:
  - `Args: cobra.ExactArgs(1)` — `cmd/enable.go:16`,
    `cmd/disable.go:16`, `cmd/skill_add.go:47`,
    `cmd/skill_delete.go:45`, `cmd/service.go:65`.
  - `Args: cobra.MaximumNArgs(1)` — `cmd/watched.go:56`,
    `cmd/watched.go:105`.
  - `Args: target.Args` (custom) for subcommand groups — `cmd/aliases.go:15`.
- **Field-level validation** is colocated with the parsing code. Example:
  - Min-overlap-type flag accepts both names and numbers; rejects with
    `fmt.Errorf("invalid min overlap type %q: use 1, 2, 3 or adjacent,
    partial, duplicate", value)`
    (`packages/cli/cmd/skill_overlap.go:400-402`).
  - Skill path validator returns `fmt.Errorf("skill path cannot be
    empty")` (`packages/cli/cmd/skill_delete.go:127`).
  - Log level validator returns `fmt.Errorf("invalid log level %q; use
    error, warn, info, or debug", level)`
    (`packages/cli/internal/logging/logging.go:82`).
- **Path resolution** goes through `internal/config` helpers
  (`configpkg.RegistryPath`, `configpkg.UpdatesPath`, `configpkg.Location`)
  and surfaces `ErrConfigNotFound` rather than fabricating paths.

## Color / Style Conventions

The AGENTS.md local-notes file is short and load-bearing:

> - Reserve yellow for interactive key hints and navigation help text.
> - Do not use yellow for progress labels or spinner status text.
> - Prefer the existing CLI palette for progress/status text:
>   magenta/cyan/light-magenta before introducing new colors.
> `AGENTS.md:1-5`

Observed usage in code matches the rule:

- **Yellow (key hints only)**: `pterm.NewStyle(pterm.FgYellow,
  pterm.Bold)` is used for help-bar key labels and key counters in
  `cmd/skill_check_updates.go:590`, `:640`, `:673`. Not used for
  spinners.
- **Magenta / LightMagenta (label, then value)**: `styleLabel(...)` →
  `FgMagenta, Bold`; value formatting uses
  `pterm.NewStyle(pterm.FgLightMagenta).Sprint(value)`
  (`packages/cli/cmd/skill_check_updates.go:665-677`,
  `packages/cli/internal/maintenance/maintenance.go:52`).
- **Cyan / LightCyan (headings, tool names)**: `FgCyan, Bold` for the
  selected tool name, `FgLightCyan, Bold` for group headers and skill
  names in the overlap report
  (`packages/cli/cmd/skill_overlap.go:288`, `:344`, `:362`).
- **Green (success)**: `pterm.Success` for "Updated skill: …",
  "Configured …", "Moved … unmanaged target entries"
  (`packages/cli/cmd/skill_check_updates.go:151`,
  `cmd/onboard.go:64`, `cmd/move_unmanaged.go:214`).
- **Red (errors, high scores)**: `FgRed, Bold` for high-overlap scores
  and high-severity overlap types
  (`packages/cli/cmd/skill_overlap.go:370`, `:381`).
- **DarkGray (de-emphasis)**: hint text, footer separators
  (`packages/cli/cmd/prompt.go:281`,
  `packages/cli/cmd/skill_check_updates.go:639`).
- **LightWhite (body)**: prose lines and explanations
  (`packages/cli/cmd/skill_check_updates.go:578`).
- **Interactive spinners**: `pterm.DefaultSpinner.Start(text)` (yellow
  default), but the spinner text is itself short, magenta/cyan framed by
  the surrounding labels — keep yellow off the label itself.
- **Background highlight for the interactive help bar**:
  `pterm.NewStyle(pterm.BgDarkGray, pterm.FgLightWhite,
  pterm.Bold).Sprint(" … ")` for the help-bar pill
  (`packages/cli/cmd/skill_check_updates.go:584`).

## What NOT to Do

Anti-patterns already established in this codebase. Don't introduce any
of the following — they are deliberate choices, not gaps:

- **Don't introduce a new logger library** (zap, zerolog, logrus, etc.).
  The codebase uses the stdlib `log.Logger` behind the
  `logging.Logger` interface. Add a new method on the interface or a
  new constructor in `internal/logging` instead.
- **Don't use yellow for progress labels or spinners.** Yellow is
  reserved for interactive key hints (AGENTS.md and the
  `FgYellow, Bold` usages in `cmd/skill_check_updates.go:590, 640,
  673`). Use magenta/cyan/light-magenta for status text.
- **Don't add a new color outside the existing CLI palette** without a
  corresponding update to AGENTS.md and the palette comment in
  `cmd/skill_check_updates.go:585-678`. Reuse the existing helpers
  (`styleLabel`, `overlapScoreStyle`, `overlapTypeStyle`).
- **Don't add `testify` to the Go module.** Tests use stdlib
  `testing`, plain `t.Fatalf`, and helper assertions defined inside the
  test files themselves (e.g. `assertContains`, `assertExists`,
  `assertNotExists`, `assertSymlinkTargetContains` in
  `packages/cli/e2e_test.go:661-691`). Use those patterns.
- **Don't introduce a mock library** (gomock, mockery). The established
  pattern is module-level function variables swapped in tests
  (e.g. `skillDeleteLoadResolvedLocation`). See TESTING.md.
- **Don't define cobra commands with `Run:`** — use `RunE: func(_ *cobra.Command,
  args []string) error` and return the error. `SilenceUsage` and
  `SilenceErrors` are already true on the root command
  (`packages/cli/cmd/root.go:91-92`).
- **Don't wrap unrelated, non-action context into error messages.** The
  established pattern is `<verb> <object>: %w`. Avoid stack-trace-like
  prefixes or capitalised error text.
- **Don't wire oxfmt into scripts** until the project has decided to
  replace Prettier. Right now the active formatter is Prettier
  (`packages/web/package.json:11-12`); `.oxfmt.json` is present but
  unused. Either remove the file or update scripts together — don't
  leave both running.
- **Don't import internal packages unaliased** in `cmd/` files — the
  `cmd` package name collides with the standard `cmd` import path and
  many internal package names. Always alias:
  `configpkg`, `backuppkg`, `syncpkg`, `statuspkg`, `remotepkg`, etc.
- **Don't use `errors.New` for non-sentinel errors** — use
  `fmt.Errorf("…: %w", err)` (wrapping) or `fmt.Errorf("…")` (plain).
  `errors.New` is reserved for sentinels such as
  `internal/config/discovery.go:15`.

## File / package map (quick reference)

- `packages/cli/main.go` — entry point, prints error via `pterm.Error`,
  exits 1 on error.
- `packages/cli/cmd/` — cobra command tree, one `newXxxCommand()` per
  command, sibling `xxx_test.go` co-located.
- `packages/cli/internal/<area>/` — domain packages: `agenttools`,
  `backup`, `config`, `logging`, `maintenance`, `mover`, `overlap`,
  `remote`, `selfupdate`, `service`, `skills`, `status`, `sync`,
  `versionfmt`, `watch`. Each has its own `*_test.go` next to the
  source.
- `packages/cli/testdata/fake-skills-cli.go` — fake `skills` CLI used
  by the e2e tests; invoked via a PATH shim written to a temp
  `binDir` in `e2e_test.go:412-420`.
- `packages/web/src/pages/`, `src/views/`, `src/components/`, `src/lib/`
  — Astro 6 site, kebab-case lib helpers, PascalCase components.
- `packages/web/test/` — `*.test.ts` for Vitest, `e2e/*.spec.ts` for
  Playwright.
