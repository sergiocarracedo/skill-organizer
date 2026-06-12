# Plan 03-01 Summary

**Completed:** 2026-06-12
**Phase:** 3 — Observability (REQ-8)

## What was built

The new `packages/cli/internal/telemetry/` package, the data and
dependency-injection foundation for the opt-in anonymous telemetry
layer. The package exposes the 7-field `Event` struct with schema
validation, a `Recorder` interface with a no-op default, the
`Identity` type that produces 32-hex-char `install_id`/`host_id`
files, and a swappable `RecorderFactoryFunc` for test injection.
22 unit tests cover Event validation (5 sub-cases + 2 host/version
error paths), JSON shape, ULID format, timestamp format, the
`NoopRecorder` drop path, the factory swap round-trip, and the
identity round-trip, corruption recovery, app-dir creation, and
host-ID rotation. No callers yet — the cobra wire-up is plan 02.

## Key files

- `packages/cli/internal/telemetry/event.go` — `Event` struct with
  `json:"snake_case"` tags in declaration order, regex-compiled
  validators, `NewEventID()` (ULID via `oklog/ulid/v2`), and
  `NewTimestamp()` (RFC3339 UTC). 79 lines.
- `packages/cli/internal/telemetry/recorder.go` — `Recorder`
  interface, `NoopRecorder` (zero-egress default), the
  `RecorderFactoryFunc` package var + `NewRecorder()` wrapper, and
  the `NewHTTPClientFunc` placeholder for plan 02. 51 lines.
- `packages/cli/internal/telemetry/identity.go` — `Identity`,
  `LoadOrCreate(appDir)` (creates dir if missing, generates
  missing IDs from `crypto/rand`), `RotateHostID(appDir)`, and the
  unexported `generateID(io.Reader)` test seam that powers
  `TestIdentityIs32HexChars`. 141 lines.
- `packages/cli/internal/telemetry/event_test.go` — 7 top-level
  tests + 5 sub-cases.
- `packages/cli/internal/telemetry/recorder_test.go` — 3 tests
  (`NoopRecorder` drops, factory-returns-noop, factory-swap with
  captured `[]Event`).
- `packages/cli/internal/telemetry/identity_test.go` — 7 tests
  (hex format, create-if-missing, reuse-if-present, rotation,
  corruption recovery, app-dir creation, regenerate-on-call).
- `packages/cli/go.mod` / `go.sum` — `oklog/ulid/v2 v2.1.1`
  added via `go get`. After event.go was created, `go mod tidy`
  promoted it from indirect to direct, and also tidied three
  pre-existing indirect→direct promotions (creack/pty,
  pmezard/go-difflib, golang.org/x/term) that were already
  directly imported by other code in the module.

## Decisions made

- **Single call to `go mod tidy` after `event.go` exists.** The
  plan's task 1 step says "run `go get` then `go mod tidy`", but
  with no caller, `go mod tidy` removes the unused dep. The
  correction was: `go get` first (deposits the dep in the
  indirect block of `go.mod` and updates `go.sum`), then create
  `event.go` (which imports it), then `go mod tidy` (which
  promotes the dep from indirect to direct). Net result is the
  same — the dep is in `go.mod` — and `go list -m
  github.com/oklog/ulid/v2` works from the very first commit.
- **Added two extra Validate tests** (`TestEventValidateRejectsBadHostID`
  and `TestEventValidateRejectsEmptyVersion`) beyond the
  5-sub-case table. The plan's table-driven
  `TestEventValidateFields` covers 5 of the 7 error paths; the
  extra tests cover the remaining two (`host_id` and `version`).
  Cost: 2 extra lines of code, full coverage of the Validate
  function.
- **`fakeRecorder` declared at package scope.** A first attempt
  declared the type and its `Record` method inside
  `TestRecorderFactorySwapRoundtrip` — but Go does not allow
  method declarations inside function bodies. The type is now a
  package-level test double used by the factory-swap test.

## Deviations from plan

- **Step 1 of task 03-01-01 was split.** The plan said "go get
  and then go mod tidy" in the same task, with verify steps that
  expect the dep to be findable by `go list -m`. Running
  `go mod tidy` with no caller silently removed the dep, which
  would have failed the verify. The fix: run `go get` only in
  task 1, then run `go mod tidy` after `event.go` exists in
  task 2. This split leaves the `go get` in the task 1 commit
  and the `go mod tidy` (and direct-promotion) in the task 2
  commit alongside `event.go`. The plan's threat #7 explicitly
  anticipated "demoted to indirect" but the actual behaviour
  was "removed entirely with no callers". Net effect on the
  repo: the dep is in `go.mod` and `go build` succeeds.

## Notes for downstream

- The default `RecorderFactoryFunc` returns `NoopRecorder{}`. Plan
  02 will replace the default factory with an `HTTPRecorder`
  when telemetry is enabled and an endpoint is configured; the
  test in `TestRecorderFactoryReturnsNoopOnEmptyConfig` will
  need to be relaxed (or moved) when plan 02's factory lands.
- `NewHTTPClientFunc` is wired with a 10-second timeout. Plan 02
  will use it from the `HTTPRecorder`; plan 02's zero-egress test
  will swap it to return a client with a counting transport.
- `bytes.NewReader` is the test seam for `generateID` (it is
  unexported). The next plan's tests can call `generateID`
  directly with a deterministic reader.
- `LoadOrCreate` regenerates IDs on the rare event of a corrupted
  on-disk file (hand-edit, partial write, etc.). The new ID is
  written back to disk before being returned. There is no
  warning log because the package is logger-free by design; the
  caller can detect regeneration by comparing the returned ID to
  the on-disk content (the package does not expose this).
- The `cmd/root.go` and `cmd/skill.go` files are untouched. Plan
  02 wires the first-run prompt, the per-command event emit, and
  the buffer drain into the existing `PersistentPreRun` /
  `PersistentPostRun` hooks.

## Verification summary

- `go build ./...` — success
- `go test ./...` — 184 passed, 19 packages (162 baseline + 22 new)
- `go vet ./...` — no issues
- `gofmt -d packages/cli/internal/telemetry/*.go` — clean
- `pnpm run test:cli:e2e` (the lefthook pre-commit command) —
  passes
- `git diff --stat HEAD~7 HEAD` — 8 files changed, 761 insertions,
  3 deletions. No `cmd/` or `config/` file modified.
