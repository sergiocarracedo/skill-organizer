# Plan 03-02 Summary

**Completed:** 2026-06-12
**Phase:** 3 — Observability (REQ-8)

## What was built

Wired the cobra surface and shipped a working end-to-end telemetry
path. The new code adds: a `HTTPRecorder` that POSTs JSON and treats
4xx/5xx as failure, a `Buffer` JSONL spool with `O_APPEND` writes
and a 1 MB FIFO eviction post-condition, a TTY-gated
`FirstRunPrompt` (Pitfall P10 compliant), a `Service` umbrella with
`RecordEvent` (single write path; falls back to buffer on
recorder failure) and `DrainBuffer`, a `NormalizeCommandName` alias
table (e.g. `on` → `enable`), a `ResolveEndpoint` flag > env > YAML
precedence helper, the `TelemetryConfig` YAML struct (and its
`LoadTelemetryConfigOrDefault` / `SaveTelemetryConfig` helpers),
the `cmd/telemetry.go` subcommand (enable | disable | status |
rotate-host-id), the `--telemetry-endpoint` persistent flag, and
the `PersistentPreRun` / new `PersistentPostRun` hooks in
`cmd/root.go` that emit one event per non-skipped command. 38 new
tests exercise the buffer FIFO + drain idempotency, HTTPRecorder
smoke + 4xx/5xx failure, TTY-gated first-run prompt, alias
normalization, endpoint precedence, Service record/drain, the four
telemetry subcommands, and the root.go integration (PreRun skip,
PostRun emit, first-run fires on first run).

## Key files

- `packages/cli/internal/config/config.go` — new
  `TelemetryConfig` (Enabled bool, Endpoint string with `omitempty`),
  added to `AppConfig`; new `Normalize()` trims Endpoint whitespace.
  ~25 new lines.
- `packages/cli/internal/config/registry.go` — new
  `LoadTelemetryConfig` / `LoadTelemetryConfigOrDefault` /
  `SaveTelemetryConfig` mirroring the `BackupConfig` pattern.
  ~30 new lines.
- `packages/cli/internal/telemetry/recorder.go` — `HTTPRecorder`
  struct, `NewHTTPRecorder` constructor, `RecorderConfig` struct,
  `SetDefaultFactory` closure. 4xx/5xx is failure (the caller
  appends to the buffer for a later drain). ~70 new lines.
- `packages/cli/internal/telemetry/buffer.go` (new) — `Buffer`
  JSONL spool with `O_APPEND` writes, `Append`, `Drain`,
  `evictLocked` (FIFO), `splitLinesKeepNewline` /
  `joinBytes` helpers. 1 MB cap is a post-condition: Append
  auto-evicts when the file exceeds the cap. ~190 lines.
- `packages/cli/internal/telemetry/prompt.go` (new) — TTY-gated
  `FirstRunPrompt` + `MaybeRunFirstRunPrompt` (fire-and-forget
  wrapper matching `maintenance.MaybeNotify*`); `IsStdInTTYFunc`
  and `ConfirmFunc` func-vars for test injection. ~80 lines.
- `packages/cli/internal/telemetry/telemetry.go` (new) — `Service`
  umbrella (`AppDir` / `Identity` / `Version` / `Cfg` /
  `Recorder` / `Buffer`), `New` constructor, `RecordEvent`,
  `DrainBuffer`, `MaybeRunFirstRunPrompt` (sentinel-based
  short-circuit), `NormalizeCommandName`, `ResolveEndpoint`,
  `commandNameAliases` table. ~170 lines. `TelemetryConfig` is a
  type alias of `configpkg.TelemetryConfig` so the cmd package
  can construct it from the same struct that holds the YAML
  tags.
- `packages/cli/cmd/root.go` — new `--telemetry-endpoint`
  persistent flag with help text naming the env + YAML layers;
  the `PersistentPreRun` guard now also skips
  `cmd.Name() == "telemetry"` (in addition to `completion` and
  `help`); the new `PersistentPostRun` emits a per-command event
  for any non-skipped command. Two unexported context-key
  helpers (`withTelemetryService` / `telemetryServiceFromContext`)
  plumb the Service from PreRun to PostRun. ~80 new lines.
- `packages/cli/cmd/telemetry.go` (new) — parent `telemetry`
  cobra subcommand with four leaf subcommands
  (`enable` | `disable` | `status` | `rotate-host-id`). All
  four stub their config / telemetry dependencies through
  package-level func vars for test injection. ~180 lines.
- `packages/cli/cmd/telemetry_test.go` (new) — 4 subcommand
  tests with stubbed `telemetryLoadConfig`,
  `telemetrySaveConfig`, `telemetryAppDir`, `telemetryRotate`,
  `telemetryIdentity`, `telemetryInfo`, `telemetrySuccess`.
  ~190 lines.
- `packages/cli/cmd/root_test.go` (new) — 3 root integration
  tests: PreRun skips telemetry, PostRun emits an event, the
  first-run prompt fires (and writes the sentinel + the YAML)
  on first run. ~185 lines.
- `packages/cli/e2e_test.go` — pre-create the
  `telemetry-prompted` sentinel in `newCLIEnv` so the
  binary's first-run prompt does not block the e2e PTY tests.
- `packages/cli/internal/telemetry/buffer_test.go` (new) — 5
  buffer tests (append + read, FIFO eviction at 1 MB, drain
  idempotency, drain preserves on send failure, append creates
  file) plus 2 HTTPRecorder tests (smoke OK with 7-key JSON
  shape, 500 counts as failure). ~250 lines.
- `packages/cli/internal/telemetry/prompt_test.go` (new) — 3
  first-run prompt tests (sticky yes, sticky no, non-TTY
  skipped-and-not-persisted). ~135 lines.
- `packages/cli/internal/telemetry/telemetry_test.go` (new) —
  2 normalization/precedence tables plus 4 Service tests
  (record writes to buffer on failure, record has no egress
  when disabled, drain sends and truncates, new creates the
  app dir). ~210 lines.
- `packages/cli/internal/config/registry_test.go` — new
  `TestTelemetryConfigRoundtrip` (zero-value on fresh AppDir;
  round-trip of the `{Enabled: true, Endpoint: ...}` case).

## Decisions made

- **TelemetryConfig lives in BOTH packages, with a type alias.**
  The plan calls `telemetrypkg.TelemetryConfig{...}` from the
  cmd package, but the YAML persistence layer lives in
  `config.TelemetryConfig`. The cleanest is to make
  `telemetry.TelemetryConfig` a type alias of
  `configpkg.TelemetryConfig` — the cmd package can construct
  it from the same struct that has the `yaml:` tags. The plan
  has slight ambiguity here; the type alias resolves it
  without duplication.
- **Buffer auto-evicts inside Append.** The plan's
  `TestBufferFIFOEvictionAt1MB` expected the file to grow past
  1 MB and then call `evictLocked` explicitly. My implementation
  auto-evicts inside `Append` when the file exceeds the cap
  (the post-condition is "the file is never observed to
  exceed 1 MB"). The test was rewritten to assert the
  post-condition: after 12000 events (~2.6 MB) the file is <=
  1 MB AND the oldest events are gone AND the newest is
  preserved. This is a stronger property than the original
  test (which would have been racy on the size > cap check
  since the auto-evict kicks in as soon as the cap is
  exceeded).
- **e2e test pre-creates the sentinel.** The new first-run
  prompt would block the e2e PTY tests (they don't drive that
  prompt). The fix is one line in `newCLIEnv` that writes
  `telemetry-prompted` with content `no` so the prompt
  short-circuits and telemetry is opt-out in the e2e
  environment. The alternative was an env-var skip, but
  pre-creating the sentinel also exercises the actual
  "user has already answered" path.
- **`firstRunCall` helper struct in `prompt_test.go`.** Go
  does not allow method declarations inside function bodies;
  the helper struct holds the answer and the call count and
  exposes a `callback(sentinelPath)` method to build the
  onAnswer closure.
- **`eventCapture` is a package-scope test Recorder** in
  `root_test.go` for the same reason — Go forbids method
  declarations inside function bodies.
- **`bug #1` fix: env var is `SKILL_ORGANIZER_TELEMETRY_ENDPOINT`.**
  The plan text in task 03-02-06 said `_ENABLED` (which would
  be a different env var for a different purpose: enabling /
  disabling). The CONTEXT, the RESEARCH, the flag's own help
  text, and the OBSERVABILITY.md doc all specify `_ENDPOINT`
  for the URL. The plan-checker confirmed this. I used
  `_ENDPOINT` in `root.go`.
- **`bug #2` fix: `TestService_RecordEvent_NoEgressWhenDisabled`
  swaps the factory BEFORE `telemetry.New(...)`.** The
  `Service.Recorder` is set inside `New` by calling
  `NewRecorder()`, which calls `RecorderFactoryFunc()` at
  construction time. Swapping the factory after `New` returns
  is a no-op for that `Service`. The test must call
  `SetDefaultFactory` (or assign `RecorderFactoryFunc`)
  BEFORE `New`. I documented this prominently in the test
  with a `// CRITICAL (BUG #2)` comment.

## Deviations from plan

- **`TestBufferFIFOEvictionAt1MB` rewritten.** The plan's
  version assumed explicit `evictLocked()` calls. My
  implementation auto-evicts inside `Append`, which is the
  "post-condition" pattern the RESEARCH recommended (P7). The
  test was rewritten to assert the post-condition (file <= 1
  MB, oldest events gone, newest preserved). Net result: same
  must-have coverage, but a stronger property.
- **`TelemetryConfig` is a type alias** in the telemetry
  package (see "Decisions made" above).
- **e2e test gets one new line** in `newCLIEnv` to
  pre-create the telemetry-prompted sentinel so the e2e PTY
  tests don't block on the first-run prompt.
- **`TestRootPersistentPostRun_EmitsEvent` orders the
  factory setup carefully.** `SetDefaultFactory` replaces
  `RecorderFactoryFunc`, so the test calls `SetDefaultFactory`
  FIRST and then assigns the capturing closure SECOND. The
  test has a comment explaining this. (This is the same
  concern as BUG #2: the factory must be set up before any
  `New` call that captures it.)

## Notes for downstream

- The default `RecorderFactoryFunc` is unchanged from plan 01
  (returns `NoopRecorder{}`). The new `SetDefaultFactory` is
  the production entry point — the cmd package's
  `PersistentPreRun` will call it before constructing the
  Service so the HTTPRecorder is selected when telemetry is
  enabled and an endpoint is configured.
- The `Buffer` cap (1 MB) is enforced as a post-condition.
  Plan 03's byte-for-byte schema test can use a sentinel or
  a temp dir without worrying about the cap.
- The first-run prompt's `ConfirmFunc` is wired to
  `cmd.confirm` in the cmd package's `init()`. The
  `defaultConfirm` in the telemetry package is a safe no-op
  for tests that don't import the cmd package.
- The `Service` holds a single `Recorder`; tests that need
  to inspect events should assign `svc.Recorder = &myRecorder`
  directly after `New` returns, or swap
  `RecorderFactoryFunc` BEFORE `New` is called.
- `cmd/telemetry status` prints the buffer file size in
  bytes. The `telemetry status` e2e test (if added in plan
  03) should expect an integer, not a human-readable size.
- The `PersistentPostRun` always emits `exit_status: 0`
  because cobra short-circuits the post-run path when `RunE`
  returns an error. A future enhancement could wrap every
  command's `RunE` to set the exit status on the context;
  the schema already includes `exit_status` so the value is
  just always 0 today.

## Verification summary

- `go build ./...` — success
- `go test ./internal/telemetry/...` — 53 passed
- `go test ./cmd/...` — 75 passed
- `go test ./...` — 223 passed in 19 packages (185 baseline +
  38 new tests)
- `go vet ./...` — no issues
- `gofmt -d` on changed files — clean
- `pnpm run test:cli:e2e` (the lefthook pre-commit command
  for staged CLI files) — passes
- Binary: `skill-organizer --help` lists
  `--telemetry-endpoint`; `skill-organizer telemetry --help`
  lists the four subcommands.
- `git diff --stat HEAD~12 HEAD` — 12 new files, 7 modified
  files, ~1,800 insertions, ~10 deletions.
