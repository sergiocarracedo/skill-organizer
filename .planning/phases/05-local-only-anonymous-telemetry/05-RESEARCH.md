# Phase 5 — Local-only anonymous telemetry (REQ-10) — Research

**Gathered:** 2026-06-12
**Status:** Ready for planning

## What Phase 5 is

Phase 5 is a **refactor of an existing module** (telemetry
package + cmd/root.go + cmd/telemetry.go + OBSERVABILITY.md) —
not a greenfield build. The codebase already has:

- A 3-way `Recorder` factory (`NoopRecorder`, `HTTPRecorder`,
  `NewRelicRecorder`) wired to env vars and YAML config.
- A 7-field `Event` struct with `install_id` and `host_id`
  (Phase 3 design).
- An `Identity` module that writes
  `<appDir>/install_id` and `<appDir>/host_id` files.
- A `telemetry` cobra subcommand with
  `enable`/`disable`/`status`/`rotate-host-id` and an
  8-line `status` output.
- 21 test files exercising the 7-field schema, the 3-way
  factory, and the identity module.

Phase 5 collapses all of this to:

- A 5-field `Event` (drop `install_id` + `host_id`).
- A 2-way `Recorder` factory (`NoopRecorder` + `NewRelicRecorder`).
- Build-time `var`s for the New Relic endpoint + token.
- `enable` / `disable` / `status` / `wipe` subcommands, with
  a 2-line `status` output.
- No identity files on disk.

The research effort is therefore concentrated in: (1) exactly
which test files need updating; (2) the build-time var pattern
in Go (`-ldflags -X "package.Var=value"`); (3) the contract
preservation plan so the byte-for-byte schema test stays the
canonical source of truth for the 5-field shape.

## Key existing artifacts (read in full during research)

- `packages/cli/internal/telemetry/recorder.go` (293 lines).
- `packages/cli/internal/telemetry/event.go` (79 lines).
- `packages/cli/internal/telemetry/identity.go` (141 lines).
- `packages/cli/internal/telemetry/telemetry.go` (169 lines).
- `packages/cli/internal/telemetry/buffer.go` (190 lines).
- `packages/cli/internal/telemetry/prompt.go` (78 lines).
- `packages/cli/cmd/root.go` (139 lines, PersistentPreRun region).
- `packages/cli/cmd/telemetry.go` (242 lines).
- `packages/cli/internal/telemetry/recorder_test.go` (718 lines,
  the canonical schema byte-for-byte test).
- `OBSERVABILITY.md` (232 lines, 7 H2 sections).

The Phase 4 decision record
(`.planning/PHASE-4-DECISION.md`) and Phase 4 CONTEXT are
preserved on disk; Phase 5 does not re-derive the New Relic
backend choice — it refines the wiring so the creds are
build-time, not env-var.

## Pillar 1: Build-time `var` injection pattern in Go

The `var NewRelicEndpoint = ""` + `var NewRelicAPIKey = ""`
pattern is the standard Go pattern for build-time injection.
The release script does:

```bash
go build -ldflags "-X 'github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry.NewRelicEndpoint=https://insights-collector.newrelic.com/v1/accounts/1234/events' -X 'github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry.NewRelicAPIKey=NRAK-...'" ./cmd
```

Trade-offs (already accepted by the user — see
`05-DISCUSSION-LOG.md`):

- **Pro:** no env-var dance, no per-machine setup, no user
  documentation burden.
- **Con:** the API key is in the binary. Anyone with the binary
  can spam the maintainer's New Relic account. The user
  accepted this in exchange for "no backend to run" and
  "one-key opt-in."
- **Con:** key rotation requires a binary release. Mitigation:
  the maintainer rebuilds and re-tags.

The dev build path is: leave the `var`s empty. The factory
detects the empty strings and routes to `NoopRecorder` even
when `telemetry.enabled` is true. This is the CI and unit-test
default.

The empty-string guard is the same pattern that Phase 3 used
for `telemetry.endpoint` (the `ResolveEndpoint` helper
returned the empty string when nothing was set, and the
factory treated empty as "no recorder"). The Phase 5 refactor
removes `ResolveEndpoint` (no more user-configurable endpoint)
and replaces it with two package-level `var`s
(`NewRelicEndpoint`, `NewRelicAPIKey`) that have the same
empty-string semantics.

## Pillar 2: Why drop `install_id` and `host_id` (not just make them more random)

Both IDs are 32 hex chars from 16 random bytes
(`encoding/hex.EncodeToString(rand.Read(16))`). They are
pseudonymous, not anonymous. Pseudonymous data is linkable in
principle: if the maintainer ever gets a court order, or
subpoena, or just looks at the raw event stream, two events
from the same machine are trivially grouped.

The user's design intent (per `05-DISCUSSION-LOG.md`,
Area D, question 1): "we want anonimous data, so those ids i
guess makes no sense." The cleanest way to honor that intent
is to not emit any identifier at all. The schema goes from 7
fields to 5: drop `install_id` and `host_id`. `event_id`
stays (it's per-event random; it cannot link two events
together because it's freshly generated for every
`Record` call).

This is a stronger posture than GDPR technically requires
(GDPR permits pseudonymous data with appropriate technical
and organizational measures), but it's the right posture for
a v0.x CLI: minimal data, minimal surface, minimal future
work to re-explain.

## Pillar 3: `HTTPRecorder` removal — full impact

`HTTPRecorder` (Phase 3, lines 64-100 of `recorder.go`) is
the "bring your own endpoint" passthrough: it POSTs the flat
7-field JSON to whatever URL the user configures in
`telemetry.endpoint`. Phase 5 removes it because:

- The new design bakes the endpoint into the binary. The
  "user sets endpoint" model is no longer coherent.
- `HTTPRecorder` has no value as a test helper (the existing
  `countingTransport` test in `recorder_test.go` works just
  as well with `NewRelicRecorder`).
- The user explicitly said "no third Recorder type" (see
  `05-DISCUSSION-LOG.md`, deferred ideas: "Pluggable backend
  abstraction").

Concrete impact of the removal:

- 4 tests in `recorder_test.go` collapse to 1
  (`TestRecorderFactoryReturnsNewRelicWhenBuildVarsSet` plus
  the existing `TestRecorderFactoryReturnsNoopWhenDisabled`).
- 2 tests in `buffer_test.go` (`TestHTTPRecorderSmokeOK` and
  `TestHTTPRecorderFailureStatus`) must either be deleted or
  be ported to use `NewRelicRecorder`. The cleaner choice is
  to delete them and rely on the recorder-test file's
  existing `TestNewRelicRecorder*` tests for smoke coverage.
- `recorder.go:38-50` (`HTTPRecorder` struct + `NewHTTPRecorder`
  constructor) is removed.
- `recorder.go:55-57` (`NewHTTPClientFunc` package var)
  **stays** — `NewRelicRecorder` uses it. The
  `countingTransport` zero-egress test stays valid.
- `recorder.go:107-121` (`SetDefaultFactory`) is rewritten
  to take a 1-field `RecorderConfig{Enabled bool}`.
- `recorder.go:126-139` (`SetDefaultFactory` body) is
  rewritten with the new 2-way logic.

## Pillar 4: Test surface after the refactor

The Phase 3 byte-for-byte test (`TestHTTPRecorderSchemaByteForByte`)
is renamed to `TestNewRelicRecorderSchemaByteForByte` and
updated for 5 fields. This is the canonical source of truth
for the schema; renaming it preserves the test's role in the
codebase.

The factory tests collapse:

- `TestRecorderFactoryReturnsNoopOnEmptyConfig` (still valid).
- `TestRecorderFactorySwapRoundtrip` (still valid).
- `TestRecorderFactoryReturnsNoopWhenEndpointEmpty` (valid —
  the build-time vars are empty in the test, factory returns
  Noop).
- `TestRecorderFactoryReturnsHTTPRecorderWhenConfigured`
  (removed; no HTTPRecorder).
- `TestRecorderFactoryReturnsNoopWhenDisabled` (valid;
  enabled=false short-circuits to Noop).
- `TestRecorderFactoryPicksNewRelicWhenEnvVarsSet` (renamed
  to `...WhenBuildVarsSet`; sets the package vars and asserts
  `*NewRelicRecorder`).
- `TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete`
  (removed; no HTTPRecorder fallback).
- `TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete`
  (valid; empty build vars short-circuit to Noop).
- `TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured`
  (removed; no HTTPRecorder).

Net result: 9 factory tests → 4 factory tests, all of which
are already passing today (only the HTTPRecorder-specific
ones are removed).

## Pillar 5: First-run prompt copy

The current prompt
(`prompt.go:51`):
`"Enable anonymous telemetry? (only command names, no args/paths/PII)"`

The Phase 5 copy appends a one-line off-ramp:
`"Enable anonymous telemetry? (only command names, no args/paths/PII) — use 'telemetry disable' to turn off at any time"`.

This is a copy tweak, not a structural change. The y/N
behavior, the default, the TTY guard, and the sticky persistence
all stay as in Phase 3.

## Pillar 6: `telemetry wipe` implementation

`wipe` is the simplest new subcommand. Implementation:

```go
func newTelemetryWipeCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "wipe",
        Short: "Delete all buffered telemetry events from disk",
        RunE: func(cmd *cobra.Command, _ []string) error {
            appDir := telemetryAppDirFunc()
            path := filepath.Join(appDir, telemetrypkg.BufferFileName)
            if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
                return fmt.Errorf("wipe buffer: %w", err)
            }
            return nil
        },
    }
}
```

Idempotent: `os.Remove` on a missing file returns an error,
which we filter with `os.IsNotExist`. The user runs `wipe`
twice in a row and gets the same result.

Output: print "Wiped N bytes from telemetry-buffer.jsonl" on
success, "Nothing to wipe" if the file didn't exist.

## Pillar 7: `telemetry status` is two lines

The current `status` (Phase 4) prints 8 lines:
Enabled, Endpoint, Recorder, Account ID, Insert key,
Install ID, Host ID, Buffer file. Phase 5 collapses to:

```
Enabled: yes
Recorder: newrelic
```

Two lines. The implementation drops the unused helpers
(`shortAccountID`, `keyPresence`, `shortID`, `emptyAsNone`)
and trims the `RunE` body to two `pterm.Info.Printfln` calls.
`recorderTypeName` collapses from 3-way to 2-way.

## Pitfalls captured

- **P1 — `recorder.go` line 38 is a public package var.**
  `RecorderFactoryFunc` is exported. Removing it would be a
  breaking API change for any third-party test or fork. The
  refactor keeps the var and updates its default closure.
- **P2 — `Service.Identity` is referenced in `cmd/root.go:99-108`
  and in tests.** Removing the field is a structural change to
  the `Service` struct. All call sites must be updated in the
  same commit.
- **P3 — `telemetryIdentity` and `telemetryRotate` package-level
  funcs in `cmd/telemetry.go` are referenced by
  `cmd/telemetry_test.go`.** Removing them requires removing
  the corresponding tests in the same commit. Otherwise
  `go test ./packages/cli/cmd/...` fails.
- **P4 — `identity.go` is imported by `telemetry.go`.** Deleting
  `identity.go` without first removing the import fails the
  build. The refactor removes the import statement in
  `telemetry.go` in the same commit as the file deletion.
- **P5 — `recorder_test.go` has 4 tests that depend on
  `HTTPRecorder`.** They are removed in the same commit as
  the `HTTPRecorder` deletion. The smoke coverage they
  provided is replaced by the existing
  `TestNewRelicRecorderContractEnforced` plus the renames.
- **P6 — `observability_test.go` has
  `TestOBSERVABILITYHasAllSevenSections` and
  `TestOBSERVABILITYExampleMatchesEmitted`.** Both need
  updating: section count goes 7→6, example block drops
  `install_id` and `host_id` and adds the new field order.
- **P7 — The first-run prompt copy change is in `prompt.go:51`.**
  The string is referenced by the
  `TestFirstRunPrompt*` tests in `prompt_test.go`. The
  test must be updated to assert the new copy.
- **P8 — `--telemetry-endpoint` flag is referenced in
  `cmd/root_test.go`.** Removing the flag requires removing
  the test in the same commit. Otherwise
  `go test ./packages/cli/cmd/...` fails.
- **P9 — `NewRelicRecorder{Endpoint, InsertKey}` is
  struct-initialized, not constructor-initialized.** The
  factory creates the struct directly. The factory's
  rewrite reads the build-time `var`s, not the struct
  fields, so the test setup needs to set the package vars
  before calling `SetDefaultFactory`.
- **P10 — `Service.New` calls `LoadOrCreate` unconditionally.**
  After deleting `LoadOrCreate`, the constructor must be
  rewritten to skip the identity step. The new constructor
  is simpler: just build the `Service` with the config
  and the resolved app dir.

## Decisions referenced from prior phases

- Phase 3 byte-for-byte schema test is the canonical source
  of truth for the schema (Phase 3 CONTEXT, Section "Test
  Surface"). Phase 5 renames the test (`TestHTTPRecorder*` →
  `TestNewRelicRecorder*`) and updates the asserted fields,
  preserving the test's role.
- Phase 3 `RecorderFactoryFunc` package-var pattern (Phase 3
  CONTEXT, Section "Established Patterns"). Phase 5 keeps
  the pattern, updates the default closure.
- Phase 4 413/429 hard-drop, 503 single-retry, `X-Insert-Key`
  header, `User-Agent: skill-organizer/<version>` (Phase 4
  CONTEXT, Section "Agent's Discretion"). Phase 5 keeps
  these. `RecorderVersion` stays. The `NewRelicRecorder`
  constructor changes only in the **source** of its endpoint
  + key (build-time `var`s, not env vars).
- Phase 4 `OBSERVABILITY.md` "Backend: New Relic" sub-section
  (lines 106-173 of the current file). Phase 5 rewrites
  this sub-section as "Build-time backend" with the new
  -ldflags contract.

## Out-of-scope research (deferred)

- **A real-account smoke test against New Relic.** Not in
  scope; the codebase uses `httptest.NewServer` for the same
  reason. A future phase can add a build-tag-gated test that
  reads the real endpoint from a secret and asserts a single
  POST.
- **An alternative to `-ldflags` for the build-time var
  injection.** Considered and rejected: a YAML file
  embedded in the binary via `go:embed` is more complex and
  has the same property (the secret is in the binary, just
  in a different place). A config server the binary phones
  home to is more complex and requires a network call at
  startup. `-ldflags -X` is the simplest approach that
  honors the user's design intent.
- **A test-time stub for the build-time vars.** Considered
  and rejected: the existing `RecorderFactoryFunc` package
  var IS the stub. Tests reassign it in `t.Cleanup` to a
  closure that returns a `fakeRecorder` or a configured
  `*NewRelicRecorder`. No additional seam is needed.

## Summary for the planner

Phase 5 is a refactor with three vertical slices:

1. **Recorder core (plan 05-01):** `Event` struct, factory,
   recorder implementations, identity module removal. ~12
   test files updated. The byte-for-byte schema test
   becomes the new canonical 5-field test.
2. **CLI surface (plan 05-02):** `cmd/root.go` PersistentPreRun
   cleanup, `cmd/telemetry.go` subcommand surface, first-run
   prompt copy. ~3 test files updated. Adds `telemetry wipe`.
3. **Docs (plan 05-03):** New `PRIVACY.md`, updated
   `OBSERVABILITY.md`. `observability_test.go` updated.

No greenfield code; everything has a precedent. No new
dependencies; `crypto/rand` and `os.Remove` are stdlib.

The user has accepted the build-time key in the binary as the
v0.x trade-off. This is captured in the CONTEXT and is not
re-litigated by the planner.

---

*Phase: 05-local-only-anonymous-telemetry*
*Research: 2026-06-12*
