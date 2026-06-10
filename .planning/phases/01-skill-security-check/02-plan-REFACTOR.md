---
wave: 1
depends_on: []
files_modified:
  - packages/cli/internal/agenttools/agenttools.go
  - packages/cli/internal/agenttools/agenttools_test.go
  - packages/cli/internal/config/config.go
  - packages/cli/internal/config/registry.go
  - packages/cli/internal/config/overlap_test.go
  - packages/cli/internal/config/agent_selection_test.go
  - packages/cli/cmd/skill_overlap.go
  - packages/cli/cmd/skill_overlap_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-4
objective: "Extract agent-selection logic (choose/select/prompt/spinner/launch) from skill_overlap.go into internal/agenttools, and rename OverlapConfig → AgentSelectionConfig with YAML migration fallback — no user-facing behavior change."
must_haves:
  - "go test ./internal/agenttools/... passes with at least 3 new tests"
  - "go test ./internal/config/... passes (renamed types, YAML migration test)"
  - "go test ./cmd/... passes (overlap command still works with new helpers)"
  - "packages/cli/internal/agenttools/agenttools.go exports ChooseAgentTool and SelectInstalledTool functions"
  - "packages/cli/internal/config/config.go has AgentSelectionConfig type and AppConfig.AgentSelection field (no more AppConfig.Overlap)"
  - "A YAML config file with only overlap:* is still readable (migration fallback)"
---

# Plan 02: Refactor agent-selection helper + config rename

## Objective

Extract `chooseOverlapTool` and `selectInstalledTool` from `cmd/skill_overlap.go` into the `internal/agenttools` package as exported functions `ChooseAgentTool` and `SelectInstalledTool`, backed by swappable function variables for test injection. Rename `OverlapConfig` → `AgentSelectionConfig` and move it from `AppConfig.Overlap` to `AppConfig.AgentSelection` with YAML migration fallback. Move spinner/launch helpers (cursor hide/show, spinner start, launch session) into `internal/agenttools` as swappable function variables as well. The overlap command itself is updated to call through the new shared helpers. Existing tests still pass.

## Context

This is a pure refactoring plan — no new behavior, no new user-facing commands. It creates the shared infrastructure that Plans 03 (metadata), 04 (check-security command), and 05 (hooks) depend on.

Key conventions to follow:
- Package-level function variables for test injection (established pattern in `cmd/skill_delete.go`, `cmd/skill_overlap.go`)
- Aliased internal imports: `configpkg` for `internal/config`
- No testify, no mock libraries — `t.Cleanup` swap for fakes
- Stdlib `testing` only
- YAML migration is read-only: never auto-rewrite the file

## Tasks

<task id="refactor-01">
<name>Add ChooseAgentTool + SelectInstalledTool to internal/agenttools</name>
<files>
  - packages/cli/internal/agenttools/agenttools.go
  - packages/cli/internal/agenttools/agenttools_test.go
</files>
<action>
Add to `agenttools.go`:

1. Import `configpkg` (aliased import of `github.com/sergiocarracedo/skill-organizer/cli/internal/config`).

2. Add two exported function variables at package level:
   - `var ChooseAgentToolFunc = chooseAgentToolImpl` — signature: `func(installed []InstalledTool, cfg configpkg.AgentSelectionConfig, explicitID string, choose bool) (InstalledTool, configpkg.AgentSelectionConfig, error)`
   - `var SelectInstalledToolFunc = selectInstalledToolImpl` — signature: `func(installed []InstalledTool, selector ToolSelector) (InstalledTool, error)`
   - Add a `ToolSelector` type: `type ToolSelector func(labels []string, defaultOption string) (string, error)`

3. Add `chooseAgentToolImpl` — identical logic to `cmd/skill_overlap.go`'s `chooseOverlapTool` but uses `configpkg.AgentSelectionConfig` instead of `configpkg.OverlapConfig`. Logic:
   - If `explicitID != ""`, find and return that tool (updating `DefaultAgentTool`), or error
   - If `!choose && cfg.DefaultAgentTool != ""`, find the cached tool or fall through
   - Otherwise, call `selectInstalledToolImpl(installed, selector)` and use result
   - Return the final `AgentSelectionConfig` with `DefaultAgentTool` set

4. Add `selectInstalledToolImpl` — identical logic to `cmd/skill_overlap.go`'s `selectInstalledTool`:
   - Sort installed by name
   - Build labels via `Labels(installed)`
   - Call `selector(labels, labels[0])` to get user choice
   - Resolve back to `InstalledTool` and return

5. Add convenience wrapper:
   - `func ChooseAgentTool(installed []InstalledTool, cfg configpkg.AgentSelectionConfig, explicitID string, choose bool, selector ToolSelector) (InstalledTool, configpkg.AgentSelectionConfig, error)` — delegates to `ChooseAgentToolFunc`
   - `func SelectInstalledTool(installed []InstalledTool, selector ToolSelector) (InstalledTool, error)` — delegates to `SelectInstalledToolFunc`

6. Add spinner/launch helpers:
   - `type SpinnerHandle interface { UpdateText(text string); Success(text ...any); Fail(text ...any) }`
   - `var StartSpinnerFunc func(text string) (SpinnerHandle, error)` — default `defaultStartSpinner`
   - `var HideCursorFunc func()` — default `func() { fmt.Fprint(os.Stdout, "\033[?25l") }`
   - `var ShowCursorFunc func()` — default `func() { fmt.Fprint(os.Stdout, "\033[?25h") }`
   - `var LaunchSessionFunc func(tool InstalledTool, prompt string) error` — default `defaultLaunchSession`
   - `defaultStartSpinner`: hide cursor, call `pterm.DefaultSpinner.Start(text)`, return spinner or show cursor on error
   - `defaultLaunchSession`: if `tool.Tool.PlanArgs == nil`, return error; exec `tool.Binary` with `PlanArgs(prompt)`, wire stdin/stdout/stderr, run
   - `func StartSpinner(text string) (SpinnerHandle, error)` — delegates to `StartSpinnerFunc`
   - `func LaunchSession(tool InstalledTool, prompt string) error` — delegates to `LaunchSessionFunc`
   - `func HideCursor()` / `func ShowCursor()` — delegate to func vars

Add to `agenttools_test.go`:

7. Test `TestChooseAgentToolUsesSavedDefault`: create `InstalledTool` list, set `AgentSelectionConfig{DefaultAgentTool: "codex"}`, call `ChooseAgentTool` with `choose=false` and a fake selector that errors, verify codex is returned.

8. Test `TestChooseAgentToolUsesExplicitID`: create list, pass `explicitID="claude"`, verify claude is returned regardless of config default.

9. Test `TestChooseAgentToolErrorsOnMissingExplicitID`: create list with only claude, pass `explicitID="codex"`, verify error.

10. Test `TestSelectInstalledToolPrompts`: create list, pass a fake selector that returns the second option, verify the second tool is returned.

11. Test `TestStartSpinnerAndLaunchSessionCompile`: just call the constructors to verify they compile.

12. Restore all func vars in `t.Cleanup`.
</action>
<verify>
- `go test ./internal/agenttools/...` passes
- `TestChooseAgentToolUsesSavedDefault` passes
- `TestSelectInstalledToolPrompts` passes
</verify>
<done>[ ]</done>
</task>

<task id="refactor-02">
<name>Rename OverlapConfig → AgentSelectionConfig with YAML migration</name>
<files>
  - packages/cli/internal/config/config.go
  - packages/cli/internal/config/registry.go
  - packages/cli/internal/config/overlap_test.go
</files>
<action>
In `config.go`:

1. Rename `type OverlapConfig struct` → `type AgentSelectionConfig struct` (same fields, same yaml tags: `yaml:"default-agent-tool,omitempty"` and `yaml:"acknowledged-external-tool-costs,omitempty"`).

2. In `AppConfig`:
   - Change `Overlap OverlapConfig \`yaml:"overlap,omitempty"\`` → `AgentSelection AgentSelectionConfig \`yaml:"agent-selection,omitempty"\``

3. Update `Normalize()`:
   - Rename `c.Overlap.Normalize()` call site to `c.AgentSelection.Normalize()`
   - Rename `(c *OverlapConfig) Normalize()` to `(c *AgentSelectionConfig) Normalize()`

4. Keep `OverlapConfig` as a deprecated type alias: `type OverlapConfig = AgentSelectionConfig` (so existing references still compile until fully migrated).

In `registry.go`:

5. Add new public functions:
   - `func LoadAgentSelectionConfig(path string) (AgentSelectionConfig, error)` — reads `AppConfig`, returns `cfg.AgentSelection`
   - `func LoadAgentSelectionConfigOrDefault(path string) (AgentSelectionConfig, error)` — same with or-default
   - `func SaveAgentSelectionConfig(path string, as AgentSelectionConfig) error` — loads config, sets `cfg.AgentSelection = as`, saves

6. YAML migration: in `LoadAppConfig`:
   - After initial unmarshal, add logic: if `cfg.AgentSelection.DefaultAgentTool == ""`, try to decode the raw `overlap.*` keys from the YAML content. Parse into an anonymous struct `oldOverlap struct { DefaultAgentTool string \`yaml:"default-agent-tool"\`; AcknowledgedExternalToolCosts bool \`yaml:"acknowledged-external-tool-costs"\` }` with `yaml:"overlap"` wrapper. If values are found, copy them into `cfg.AgentSelection`. (The old `overlap` yaml tag on the removed field won't be processed by the standard unmarshal, so explicit fallback-reading the raw bytes is needed.)

In `overlap_test.go` (rename to `agent_selection_test.go` in the same directory):

7. Rename `TestSaveAndLoadOverlapConfig` → `TestSaveAndLoadAgentSelectionConfig`. Use `AgentSelectionConfig` instead of `OverlapConfig`, `LoadAgentSelectionConfig`/`SaveAgentSelectionConfig` instead of old names.

8. Rename `TestLoadAppConfigSupportsOverlapSection` → `TestLoadAppConfigWithAgentSelectionMigration`. Create a YAML file with `overlap: default-agent-tool: codex` (old format), call `LoadAppConfig`, assert `cfg.AgentSelection.DefaultAgentTool == "codex"`.

9. Add `TestAgentSelectionRoundTrip`: create config with both fields set, save via `SaveAgentSelectionConfig`, load via `LoadAgentSelectionConfig`, assert both fields match.
</action>
<verify>
- `go test ./internal/config/...` passes
- `TestLoadAppConfigWithAgentSelectionMigration` passes (old YAML format still loads)
- `TestSaveAndLoadAgentSelectionConfig` passes
</verify>
<done>[ ]</done>
</task>

<task id="refactor-03">
<name>Update overlap command to use shared helpers</name>
<files>
  - packages/cli/cmd/skill_overlap.go
  - packages/cli/cmd/skill_overlap_test.go
</files>
<action>
In `skill_overlap.go`:

1. Replace package-level func vars:
   - `loadOverlapConfigFunc = configpkg.LoadOverlapConfigOrDefault` → `loadAgentSelectionConfigFunc = configpkg.LoadAgentSelectionConfigOrDefault`
   - `saveOverlapConfigFunc = configpkg.SaveOverlapConfig` → `saveAgentSelectionConfigFunc = configpkg.SaveAgentSelectionConfig`

2. Remove local `chooseOverlapTool` and `selectInstalledTool` functions entirely (they now live in `agenttools`).

3. Remove the local `spinnerHandle` interface (now `agenttools.SpinnerHandle` is used).

4. Remove local `launchPlanSession` func var, `hideCursor`/`showCursor` func vars (now in `agenttools`).

5. Remove `startOverlapSpinner` func var — use `agenttools.StartSpinnerFunc` instead (rename variable to `startSecuritySpinner` or just `startSpinner`).

6. Update `newCheckOverlapCommand`:
   - `loadOverlapConfigFunc(registryPath)` → `loadAgentSelectionConfigFunc(registryPath)`
   - `saveOverlapConfigFunc(registryPath, overlapCfg)` → `saveAgentSelectionConfigFunc(registryPath, overlapCfg)`
   - `chooseOverlapTool(installed, overlapCfg, ...)` → `agenttools.ChooseAgentTool(installed, overlapCfg, ..., selectToolOption)`
   - Where `selectToolOption` is the existing `selectOption` function from `prompt.go`
   - `startOverlapSpinner("Analyzing skills")` → keep local or use agenttools StartSpinner
   - Keep the rest of the overlap flow unchanged (prompt building, cost ack, analysis, report, apply plan)

7. Remove `chooseOverlapTool`, `selectInstalledTool`, `startDefaultSpinner`, `hideCursor`, `showCursor`, `spinnerHandle` from the file. Keep `printInfoMessage`, `printDebugMessage`, `printWarningMessage` for now (they'll be extracted in a later phase if needed).

In `skill_overlap_test.go`:

8. Update all references:
   - `configpkg.OverlapConfig{}` → `configpkg.AgentSelectionConfig{}`
   - `loadOverlapConfigFunc` → `loadAgentSelectionConfigFunc`
   - `saveOverlapConfigFunc` → `saveAgentSelectionConfigFunc`
   - Update test helpers accordingly

9. Verify `TestChooseOverlapToolUsesSavedInstalledDefault` etc. now import and test the functions in `agenttools` (they moved). The existing tests that test those functions through the command's RunE should still work.
</action>
<verify>
- `go test ./cmd/...` passes
- Overlap command compiles and runs: `go build ./...` succeeds
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./internal/agenttools/...` passes with the 3+ new tests
- [ ] `go test ./internal/config/...` passes with renamed types
- [ ] `go test ./cmd/...` passes (all existing overlap tests)
- [ ] `go build ./...` succeeds
- [ ] `agenttools.ChooseAgentTool` and `agenttools.SelectInstalledTool` are exported and callable
- [ ] `configpkg.AgentSelectionConfig` exists; `configpkg.OverlapConfig` is a deprecated alias
- [ ] YAML file with `overlap: default-agent-tool: claude` loads and populates `cfg.AgentSelection.DefaultAgentTool == "claude"`
- [ ] Writing via `SaveAgentSelectionConfig` writes `agent-selection:` key (not `overlap:`)

## Rollback Guide

If this plan fails (tests don't pass or build breaks):

1. Revert all changes: `git checkout -- packages/cli/internal/agenttools/ packages/cli/internal/config/ packages/cli/cmd/skill_overlap.go packages/cli/cmd/skill_overlap_test.go`
2. If new files were created: `git clean -f packages/cli/internal/agenttools/ packages/cli/internal/config/`
3. Verify: `go test ./...` passes from clean state
4. Re-try with smaller scope (e.g., just rename config first, then move functions independently)

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | YAML migration creates data loss — overlap.* key values silently dropped | Low | Medium | Migration reads raw bytes before standard unmarshal; test `TestLoadAppConfigWithAgentSelectionMigration` validates fallback. No auto-rewrite means old data stays in the file. |
| 2 | Function-variable migration misses a call site, leaving a dangling reference to `OverlapConfig` | Medium | High | All references are traced with `grep -r "OverlapConfig\|overlap\.Overlap\|LoadOverlapConfig\|SaveOverlapConfig"` before commit; `go build` will catch unresolved references. |
| 3 | Exported function vars in `agenttools` cause test pollution between parallel tests | Low | Medium | All tests use `t.Cleanup` for restoration; no `t.Parallel()` in unit tests (established pattern). |

## Commit Message

```
refactor(cli): extract agent-selection helper into internal/agenttools, rename OverlapConfig

- Move chooseOverlapTool → agenttools.ChooseAgentTool with swappable func var
- Move selectInstalledTool → agenttools.SelectInstalledTool with swappable func var  
- Add spinner/launch/cursor helpers to agenttools as func vars
- Rename OverlapConfig → AgentSelectionConfig in AppConfig.AgentSelection
- Add YAML migration fallback (reads overlap.* if agent-selection.* is absent)
- Update skill_overlap.go and tests to use new shared helpers
```
