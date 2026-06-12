---
wave: 2
depends_on: ["05-01"]
files_modified:
  - packages/cli/cmd/telemetry.go
  - packages/cli/cmd/telemetry_test.go
  - packages/cli/cmd/root.go
  - packages/cli/cmd/root_test.go
  - packages/cli/internal/telemetry/prompt.go
  - packages/cli/internal/telemetry/prompt_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-10
objective: "telemetry subcommand surface: add wipe, collapse status to 2 lines, drop rotate-host-id and --telemetry-endpoint"
must_haves:
  - "telemetry wipe subcommand exists; deletes <appDir>/telemetry-buffer.jsonl; idempotent (running on a clean app dir is a no-op)"
  - "telemetry status prints exactly 2 lines: `Enabled: yes|no` and `Recorder: newrelic|noop`"
  - "telemetry rotate-host-id subcommand is removed"
  - "telemetryIdentity, telemetryRotate, telemetryNewRelicAccountID, telemetryNewRelicInsertKey package-level funcs in cmd are removed"
  - "shortAccountID, keyPresence, shortID, emptyAsNone helpers in cmd/telemetry.go are removed"
  - "recorderTypeName is 2-way (noop, newrelic); the HTTPRecorder case is removed"
  - "--telemetry-endpoint flag is removed from rootCmd"
  - "SKILL_ORGANIZER_TELEMETRY_ENDPOINT env var is no longer read"
  - "SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID and SKILL_ORGANIZER_NEWRELIC_INSERT_KEY env vars are no longer read (already removed in 05-01, but root.go is fully cleaned in this plan)"
  - "SetDefaultFactory is called with RecorderConfig{Enabled: cfg.Enabled} (1-field)"
  - "Service is constructed with TelemetryConfig{Enabled: cfg.Enabled} (no Endpoint)"
  - "FirstRunPrompt copy appended with `(use 'telemetry disable' to turn off at any time)`"
  - "FirstRunPrompt copy in prompt_test.go is updated to match"
  - "All cmd tests pass: telemetry_test.go, root_test.go, prompt_test.go"
  - "go build ./... succeeds, go vet ./... is clean, go test ./... passes"
  - "lefthook pre-commit passes"
---

# Plan 05-02: CLI surface (telemetry subcommands, root flags, first-run prompt)

## Objective

The `telemetry` cobra subcommand surface reflects the
5-field / 2-way / build-time design: `wipe` is added, `status`
is collapsed to 2 lines, `rotate-host-id` is removed, and the
`--telemetry-endpoint` flag and the `SKILL_ORGANIZER_TELEMETRY_*`
env vars are gone. The first-run prompt copy gets a one-line
off-ramp hint.

The dependency on 05-01 is structural: `Service` no longer
has an `Identity` field, the factory takes a 1-field
`RecorderConfig`, and the env vars are gone. This plan
cleans up `cmd/` to match.

## Context

After plan 05-01 lands, the internal telemetry package is
ready. The `cmd/` package still has stale references to the
old surface: `telemetryIdentity`, `telemetryRotate`,
`telemetryNewRelicAccountID`, `telemetryNewRelicInsertKey`,
`shortAccountID`, `keyPresence`, `shortID`, `emptyAsNone`,
`newTelemetryRotateHostIDCommand`, `--telemetry-endpoint`,
and the `SKILL_ORGANIZER_TELEMETRY_*` env var reads.

Plan 05-02 is the visible-to-the-user part: a single commit
that touches `cmd/telemetry.go`, `cmd/root.go`, and
`prompt.go` so the binary's behavior matches the new
internal API.

The first-run prompt copy is changed at `prompt.go:51` to
append the off-ramp hint. The TTY guard, default, and
sticky persistence all stay.

## Tasks

<task id="05-02-01">
<name>Drop the 4 package-level func vars and the 4 unused helpers from cmd/telemetry.go</name>
<files>packages/cli/cmd/telemetry.go</files>
<action>
Edit `cmd/telemetry.go`:
- Remove the 4 package-level func vars at lines 19-32:
  - `telemetryIdentity`
  - `telemetryRotate`
  - `telemetryNewRelicAccountID`
  - `telemetryNewRelicInsertKey`
- Remove the 4 helper functions that become unused:
  - `shortAccountID`
  - `keyPresence`
  - `shortID`
  - `emptyAsNone`
- Update the import block to drop `os` if no longer used (it was only there for the env-var reads).
- The `recorderTypeName` helper at line 203 (current file) becomes 2-way: only `NoopRecorder` and `*NewRelicRecorder` cases. Remove the `HTTPRecorder` case.
</action>
<verify>
- `go build ./packages/cli/cmd/` succeeds.
- `go vet ./packages/cli/cmd/` is clean.
- `grep -n "telemetryIdentity\|telemetryRotate\|telemetryNewRelicAccountID\|telemetryNewRelicInsertKey\|shortAccountID\|keyPresence\|shortID\|emptyAsNone" packages/cli/cmd/telemetry.go` returns 0 matches.
- `grep -n "HTTPRecorder" packages/cli/cmd/telemetry.go` returns 0 matches.
</verify>
<done>[ ]</done>
</task>

<task id="05-02-02">
<name>Add telemetry wipe subcommand; collapse telemetry status to 2 lines</name>
<files>packages/cli/cmd/telemetry.go</files>
<action>
Edit `cmd/telemetry.go`:
- Add a new `newTelemetryWipeCommand()` function modeled on the existing `newTelemetryDisableCommand`. The body resolves the app dir (reuse the same `appDir` lookup the other subcommands use), then:
  ```go
  path := filepath.Join(appDir, telemetrypkg.BufferFileName)
  info, statErr := os.Stat(path)
  if os.IsNotExist(statErr) {
      telemetryInfo("Nothing to wipe.")
      return nil
  }
  if statErr != nil {
      return fmt.Errorf("stat buffer: %w", statErr)
  }
  size := info.Size()
  if err := os.Remove(path); err != nil {
      return fmt.Errorf("wipe buffer: %w", err)
  }
  telemetrySuccess("Wiped %d bytes from %s", size, telemetrypkg.BufferFileName)
  return nil
  ```
- Register the new `wipe` subcommand in `newTelemetryCommand()` (add a `wipeCmd := newTelemetryWipeCommand()` and `cmd.AddCommand(wipeCmd)` line).
- Update `Long` field in `newTelemetryCommand` to: `telemetry enable|disable|status|wipe — see OBSERVABILITY.md for the full schema and opt-in flow.`
- Rewrite `newTelemetryStatusCommand()` to print exactly 2 lines. The body:
  ```go
  cfg, _ := configpkg.LoadTelemetryConfig(registryPath)
  enabled := cfg.Enabled
  recType := "noop"
  if enabled && telemetrypkg.NewRelicEndpoint != "" && telemetrypkg.NewRelicAPIKey != "" {
      recType = "newrelic"
  }
  telemetryInfo("Enabled:    %s", boolToYesNo(enabled))
  telemetryInfo("Recorder:   %s", recType)
  return nil
  ```
  Add a `boolToYesNo(b bool) string` helper at the bottom of the file (replaces `emptyAsNone`).
- Remove the old `newTelemetryRotateHostIDCommand()` function entirely (lines 162-180 of the current file).
- Update `newTelemetryEnableCommand` to drop the `telemetry.endpoint` reference if present; the enable command is now just "flip the bool" (the factory in 05-01 handles the rest).
</action>
<verify>
- `go build ./packages/cli/cmd/` succeeds.
- `go vet ./packages/cli/cmd/` is clean.
- `grep -n "rotate-host-id\|RotateHostID" packages/cli/cmd/telemetry.go` returns 0 matches.
- `grep -n "wipe\|Wipe" packages/cli/cmd/telemetry.go` returns matches only in the new subcommand function name and the registered subcommand.
- The new `status` body has exactly 2 `telemetryInfo` calls.
</verify>
<done>[ ]</done>
</task>

<task id="05-02-03">
<name>Remove --telemetry-endpoint flag and env var reads from cmd/root.go</name>
<files>packages/cli/cmd/root.go, packages/cli/cmd/root_test.go</files>
<action>
Edit `cmd/root.go`:
- Remove the `telemetryEndpoint` package var and the
  `--telemetry-endpoint` flag binding (lines 25 and 75 of the
  current file).
- In the `PersistentPreRunE` body, remove the
  `telemetrypkg.ResolveEndpoint(...)` call and the related
  `telemetryEndpoint` reads (lines 98-103 of the current file).
- Replace the `SetDefaultFactory` call with the 1-field config:
  ```go
  telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{
      Enabled: cfg.Enabled,
  })
  ```
- Construct the telemetry Service with the 1-field
  `TelemetryConfig`:
  ```go
  svc, svcErr := telemetrypkg.New(appDir, version, telemetrypkg.TelemetryConfig{Enabled: cfg.Enabled})
  ```
- Remove the `os.Getenv` calls for
  `SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` and
  `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY` (lines 109-110 of the
  current file). The factory reads the build-time `var`s
  directly.

Edit `cmd/root_test.go`:
- Remove the test cases that set or assert on
  `--telemetry-endpoint` and the env vars.
- The test for the PersistentPreRun E2E flow updates its
  expected `SetDefaultFactory` call to take the 1-field
  config.
</action>
<verify>
- `go build ./...` succeeds.
- `go vet ./...` is clean.
- `grep -n "telemetry-endpoint\|SKILL_ORGANIZER_TELEMETRY_ENDPOINT\|SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID\|SKILL_ORGANIZER_NEWRELIC_INSERT_KEY\|telemetryEndpoint" packages/cli/cmd/root.go` returns 0 matches.
- `go test -count=1 ./packages/cli/cmd/` passes.
</verify>
<done>[ ]</done>
</task>

<task id="05-02-04">
<name>Append the off-ramp hint to the first-run prompt copy</name>
<files>packages/cli/internal/telemetry/prompt.go, packages/cli/internal/telemetry/prompt_test.go</files>
<action>
Edit `prompt.go`:
- Update the `FirstRunPrompt` body (line 51 of the current file) to append the off-ramp hint:
  ```go
  return ConfirmFunc("Enable anonymous telemetry? (only command names, no args/paths/PII) — use 'telemetry disable' to turn off at any time", false)
  ```
- Add a comment above the function: "Phase 5 (REQ-10): the prompt mentions the off-ramp so the user knows how to opt out after opting in. The schema and privacy details are not surfaced in the prompt; see OBSERVABILITY.md and PRIVACY.md."

Edit `prompt_test.go`:
- Update any test that asserts on the prompt string. The new
  string is: `"Enable anonymous telemetry? (only command names, no args/paths/PII) — use 'telemetry disable' to turn off at any time"`.
- If a test uses a substring match (e.g.
  `strings.Contains(got, "Enable anonymous telemetry?")`),
  it stays valid; if it uses exact equality, update it.
</action>
<verify>
- `go test -count=1 ./packages/cli/internal/telemetry/` passes.
- `go vet ./packages/cli/internal/telemetry/` is clean.
- The prompt string in `prompt.go:51` contains the literal
  substring "use 'telemetry disable' to turn off at any time".
</verify>
<done>[ ]</done>
</task>

<task id="05-02-05">
<name>Update cmd tests to match the new surface + verify wipe end-to-end</name>
<files>packages/cli/cmd/telemetry_test.go, packages/cli/cmd/root_test.go</files>
<action>
Edit `cmd/telemetry_test.go`:
- Delete the 4 test cases that reference `telemetryIdentity` and `telemetryRotate` (the lines around 132, 204, 263, 296 in the current file).
- Add a new test `TestTelemetryWipeRemovesBuffer`:
  - Set up a temp app dir.
  - Create `<appDir>/telemetry-buffer.jsonl` with a small payload.
  - Call `newTelemetryWipeCommand().RunE(cmd, nil)`.
  - Assert the file is gone.
  - Assert the success message matches `"Wiped N bytes from telemetry-buffer.jsonl"`.
- Add a new test `TestTelemetryWipeIdempotent`:
  - Set up a temp app dir with no buffer file.
  - Call `newTelemetryWipeCommand().RunE(cmd, nil)`.
  - Assert no error and the message matches `"Nothing to wipe."`.
- Update `TestTelemetryStatus*` tests to assert exactly 2 lines of output (Enabled + Recorder). The old 8-line test is replaced or simplified.

Edit `cmd/root_test.go`:
- Confirm the test suite passes with the new 1-field
  `RecorderConfig`. If any test asserts on the old 4-field
  config, update it.

Edit `cmd/telemetry.go`:
- Add a `telemetryWipe` package-level func var (similar to
  the existing func-var pattern in this file) so tests can
  stub the buffer path resolution. The default closure
  resolves the app dir the same way the other subcommands
  do.
</action>
<verify>
- `go test -count=1 ./packages/cli/cmd/` passes.
- `go vet ./packages/cli/cmd/` is clean.
- The new `TestTelemetryWipeRemovesBuffer` and
  `TestTelemetryWipeIdempotent` tests pass.
- `grep -n "telemetryIdentity\|telemetryRotate" packages/cli/cmd/` returns 0 matches.
</verify>
<done>[ ]</done>
</task>

<task id="05-02-06">
<name>Run full pre-commit + verify all tests + lefthook</name>
<files>none (verification only)</files>
<action>
- Run `go build ./...` (must succeed).
- Run `go vet ./...` (must be clean).
- Run `go test -count=1 ./...` (must pass with 0 failures; new tests for `wipe`, deleted tests for `rotate-host-id` and the HTTPRecorder factory tests).
- Run `pnpm run test:cli:e2e` (lefthook pre-commit hook must pass; cli-e2e may skip, which is fine).
- Confirm `skill-organizer telemetry --help` (in a test) shows 4 subcommands: enable, disable, status, wipe. No rotate-host-id.
- Confirm `skill-organizer telemetry status --help` works and the status output is 2 lines.
- Commit with message: `refactor(05-02): telemetry CLI surface — add wipe, collapse status, drop endpoint flag`.
</action>
<verify>
- All 4 commands above succeed.
- 1 atomic commit created.
- The commit message follows conventional commits.
</verify>
<done>[ ]</done>
</task>

---

*Plan: 05-02-cli-surface*
