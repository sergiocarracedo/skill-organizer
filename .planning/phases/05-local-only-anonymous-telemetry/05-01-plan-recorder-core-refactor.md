---
wave: 1
depends_on: []
files_modified:
  - packages/cli/internal/telemetry/event.go
  - packages/cli/internal/telemetry/event_test.go
  - packages/cli/internal/telemetry/recorder.go
  - packages/cli/internal/telemetry/recorder_test.go
  - packages/cli/internal/telemetry/buffer_test.go
  - packages/cli/internal/telemetry/telemetry.go
  - packages/cli/internal/telemetry/identity.go (deleted)
  - packages/cli/internal/telemetry/identity_test.go (deleted)
  - packages/cli/cmd/root.go
  - packages/cli/cmd/root_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-10
objective: "5-field Event schema, 2-way Recorder factory, identity module removed"
must_haves:
  - "Event struct has exactly 5 fields: Command, ExitStatus, Timestamp, Version, EventID (no InstallID, no HostID)"
  - "Event.Validate drops install_id and host_id regex checks"
  - "validEvent() test helper returns a 5-field event (no install_id, no host_id)"
  - "TestEventJSONShape asserts exactly 5 JSON keys in marshaled output"
  - "RecorderConfig struct has exactly 1 field: Enabled bool (no Endpoint, no AccountID, no InsertKey)"
  - "SetDefaultFactory takes RecorderConfig{Enabled: bool} and the factory closure is 2-way"
  - "Factory returns NewRelicRecorder when Enabled=true AND NewRelicEndpoint var is non-empty AND NewRelicAPIKey var is non-empty"
  - "Factory returns NoopRecorder in all other cases (disabled, or build vars empty)"
  - "New build-time vars declared at package level: var NewRelicEndpoint = \"\" and var NewRelicAPIKey = \"\""
  - "NewRelicRecorder.Record emits an envelope with the 5 schema fields plus eventType (renamed timestamp->clientTime in the envelope, unchanged from Phase 4)"
  - "HTTPRecorder and NewHTTPRecorder removed from recorder.go"
  - "NewHTTPClientFunc package var stays (used by NewRelicRecorder)"
  - "countingTransport zero-egress test stays valid (uses NewHTTPClientFunc seam)"
  - "identity.go and identity_test.go deleted from internal/telemetry/"
  - "Service.Identity field removed from telemetry.go"
  - "Service.New constructor no longer calls LoadOrCreate; signature simplified"
  - "Service.RecordEvent no longer populates InstallID or HostID"
  - "Service.Event() (or whatever the NewEventID helper is called) still produces a fresh per-event random ID via crypto/rand (ulid.Make)"
  - "All test files updated: 12 test files exercise the changes; no test passes install_id or host_id in fixture data"
  - "Source-lock test asserts that all random-byte generation in the package reads from crypto/rand (no linkable ID is produced)"
  - "go build ./... succeeds, go vet ./... is clean, go test ./... passes with 0 failures"
  - "lefthook pre-commit passes"
---

# Plan 05-01: Recorder core refactor (5-field Event, 2-way factory, identity removal)

## Objective

`internal/telemetry` is collapsed to a 5-field `Event` struct, a
2-way `Recorder` factory, and no identity module. The user-facing
config has one key (`telemetry.enabled`); the New Relic endpoint
and API key are build-time `var`s. The byte-for-byte schema test
is updated to assert the 5-field shape and is renamed
`TestNewRelicRecorderSchemaByteForByte`.

This is a self-contained refactor. No docs change. No CLI
subcommand change. The only user-facing observable is the
5-field JSON shape emitted by the recorder; everything else is
internal.

## Context

The Phase 3/4 telemetry layer has a 7-field Event schema, a
3-way factory, and an `Identity` module that writes two files
to the app dir. Phase 5's REQ-10 acceptance criteria require
collapsing all of that to: 5 fields, 2 impls, no identity
files, build-time wiring. The user's design intent (per
`05-CONTEXT.md` Area D) is "we want anonymous data, so those
ids make no sense" — drop `install_id` and `host_id` entirely.

The 4 `HTTPRecorder` factory tests and the 2 `HTTPRecorder`
buffer-test smokes are removed; the 9 factory tests collapse
to 4; the byte-for-byte schema test is renamed and asserts
5 fields.

## Tasks

<task id="05-01-01">
<name>Drop InstallID and HostID from Event struct + Validate</name>
<files>packages/cli/internal/telemetry/event.go, packages/cli/internal/telemetry/event_test.go</files>
<action>
Edit `event.go`:
- Remove the `InstallID string` and `HostID string` fields from the `Event` struct.
- In `Validate`, remove the two regex checks for `install_id` and `host_id` (the 32-hex checks).
- Add a comment above the struct: "Phase 5 (REQ-10): the schema is 5 fields. No pseudonymous identifiers are emitted. The 2 dropped fields were `install_id` and `host_id`."
- Confirm `NewEventID` still uses `ulid.Make` (which is `crypto/rand`-backed).

Edit `event_test.go`:
- Update `validEvent()` helper to return a 5-field event (drop the two ID lines).
- Update `TestEventValidateFields` to remove the 2 cases for "malformed install_id" and "bad host_id".
- Delete `TestEventValidateRejectsBadHostID` (no host_id to reject).
- Update `TestEventJSONShape` to assert exactly 5 JSON keys: `command`, `exit_status`, `timestamp`, `version`, `event_id`.
- Add a new test `TestEventHasNoIdentityFields` that asserts `json.Marshal(Event{...})` does not contain the substrings `"install_id"` or `"host_id"` in the output.
</action>
<verify>
- `go test -count=1 -run 'TestEvent|TestValidEvent' ./packages/cli/internal/telemetry/` passes.
- `go vet ./packages/cli/internal/telemetry/` is clean.
- The test count for the `event` package stays the same or increases by 1 (rename counts as a replacement, not an addition).
</verify>
<done>[ ]</done>
</task>

<task id="05-01-02">
<name>Collapse RecorderConfig + rewrite factory closure to 2-way</name>
<files>packages/cli/internal/telemetry/recorder.go</files>
<action>
Edit `recorder.go`:
- Reduce the `RecorderConfig` struct to a single field: `Enabled bool`. Remove `Endpoint`, `AccountID`, `InsertKey`.
- Add the two build-time `var` declarations at the top of the file (above `RecorderFactoryFunc`):
  ```go
  // NewRelicEndpoint and NewRelicAPIKey are injected at build time via
  // -ldflags. The user never configures these. An empty value means the
  // binary was not built with credentials; the factory falls back to
  // NoopRecorder.
  var (
      NewRelicEndpoint = ""
      NewRelicAPIKey   = ""
  )
  ```
- Rewrite the default `RecorderFactoryFunc` closure: when `Enabled` is true AND both `NewRelicEndpoint` and `NewRelicAPIKey` are non-empty, return a `*NewRelicRecorder` constructed with those values (plus `RecorderVersion` and `NewHTTPClientFunc()`). Otherwise return `NoopRecorder{}`.
- Rewrite `SetDefaultFactory` to take `RecorderConfig{Enabled: bool}`. The body delegates to the default closure above.
- Update the doc comment on `SetDefaultFactory` to describe the 2-way semantics and the build-time var contract.
</action>
<verify>
- `go build ./packages/cli/internal/telemetry/` succeeds.
- `go vet ./packages/cli/internal/telemetry/` is clean.
- `grep -n "Endpoint\|AccountID\|InsertKey" recorder.go` returns only matches inside `NewRelicRecorder`'s struct fields (which stay) and the new `var` declarations. No other field uses the old names.
</verify>
<done>[ ]</done>
</task>

<task id="05-01-03">
<name>Remove HTTPRecorder; keep NewRelicRecorder; update NewRelic envelope to 5 fields</name>
<files>packages/cli/internal/telemetry/recorder.go, packages/cli/internal/telemetry/recorder_test.go, packages/cli/internal/telemetry/buffer_test.go</files>
<action>
Edit `recorder.go`:
- Delete the `HTTPRecorder` struct, the `NewHTTPRecorder` constructor, and any helper methods (lines 64-100 of the current file).
- Keep `NewHTTPClientFunc` (package var, lines 55-57) — `NewRelicRecorder` uses it.
- Keep `countingTransport` (in `recorder_test.go`) — used by the zero-egress test.
- In `NewRelicRecorder.Record`, the envelope construction drops `install_id` and `host_id` from the inner object. The 5 remaining fields are unchanged (command, exit_status, timestamp→clientTime, version, event_id). The `eventType: "skill_organizer_command"` prefix stays.

Edit `recorder_test.go`:
- Delete the 6 HTTPRecorder-only tests:
  - `TestHTTPRecorderSchemaByteForByte`
  - `TestHTTPRecorderSchemaFieldOrder`
  - `TestHTTPRecorderFieldCount`
  - `TestRecorderFactoryReturnsHTTPRecorderWhenConfigured`
  - `TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete`
  - `TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured`
- Add a new `TestNewRelicRecorderSchemaByteForByte` (replaces the deleted HTTPRecorder byte-for-byte test):
  - Stands up `httptest.NewServer`.
  - Constructs `NewRelicRecorder{Endpoint: srv.URL, InsertKey: "test-key", HTTPClient: ..., Version: "0.4.0"}`.
  - Asserts: POST method, `application/json` content type, valid JSON.
  - Asserts exactly 5 keys in the inner object: command, exit_status, timestamp (or clientTime per the envelope rename), version, event_id. No install_id, no host_id.
  - Asserts deterministic fields byte-for-byte (command, exit_status, version).
  - Asserts volatile fields against regexes (event_id ULID, timestamp RFC3339).
- Add a new test `TestNewRelicRecorderSchemaFieldOrder` that pins the key order: command, exit_status, timestamp, version, event_id.
- Add a new test `TestNewRelicRecorderFieldCount` that asserts exactly 5 keys in the inner object.
- Update `TestNewRelicRecorderContractEnforced` to assert 5 fields (not 7) in the inner object. The envelope-level `eventType` prefix and the `clientTime` rename stay.
- Add a new test `TestRecorderFactoryReturnsNewRelicWhenBuildVarsSet` (replaces `TestRecorderFactoryPicksNewRelicWhenEnvVarsSet`):
  - Save and restore `NewRelicEndpoint` and `NewRelicAPIKey` in `t.Cleanup`.
  - Set both to non-empty values.
  - Call `SetDefaultFactory(RecorderConfig{Enabled: true})`.
  - Assert `NewRecorder()` returns a `*NewRelicRecorder` with the right Endpoint and InsertKey.
- Update `TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete` to set `NewRelicEndpoint` and leave `NewRelicAPIKey` empty (or vice versa), then assert `NewRecorder()` returns `NoopRecorder`.
- Update `TestNoopRecorderNoNetworkCalls` and `TestNoopRecorderDropsEvents` to use a 5-field event (the updated `validEvent()` helper).
- Update `TestRecorderFactorySwapRoundtrip` to use the updated `validEvent()`.

Edit `buffer_test.go`:
- Delete `TestHTTPRecorderSmokeOK` and `TestHTTPRecorderFailureStatus`. The smoke coverage is now provided by `TestNewRelicRecorderContractEnforced` and the 3 new `TestNewRelicRecorder*` tests added above.
</action>
<verify>
- `go test -count=1 ./packages/cli/internal/telemetry/` passes.
- `go vet ./packages/cli/internal/telemetry/` is clean.
- `grep -n "HTTPRecorder" packages/cli/internal/telemetry/recorder.go packages/cli/internal/telemetry/recorder_test.go packages/cli/internal/telemetry/buffer_test.go` returns 0 matches.
- The new `TestNewRelicRecorderSchemaByteForByte` is the canonical source of truth for the 5-field schema (replacing the deleted HTTPRecorder byte-for-byte test).
</verify>
<done>[ ]</done>
</task>

<task id="05-01-04">
<name>Delete identity.go and identity_test.go; rewrite Service to not use Identity</name>
<files>packages/cli/internal/telemetry/identity.go (deleted), packages/cli/internal/telemetry/identity_test.go (deleted), packages/cli/internal/telemetry/telemetry.go</files>
<action>
Delete `packages/cli/internal/telemetry/identity.go` and `packages/cli/internal/telemetry/identity_test.go`.

Edit `telemetry.go`:
- Remove the import of any identity-related code from this file (it imported `crypto/rand` and `encoding/hex` via `identity.go`; check if any are still needed and remove the import if not).
- Remove the `Identity` field from the `Service` struct.
- Rewrite `New(...)` to not call `LoadOrCreate`. The new signature stays the same (`New(appDir, version string, cfg TelemetryConfig) (*Service, error)`) but the body just builds the `Service` with the config; no identity creation or persistence.
- Rewrite `RecordEvent(...)` to not populate `InstallID` and `HostID`. The 5-field event is built inline: command, exit_status, timestamp, version, event_id (still using `NewEventID`).
- Add a comment above the new `New` constructor: "Phase 5 (REQ-10): no identity module. The Service no longer writes install_id or host_id files. Existing files left on disk from a prior version are ignored."

Edit `cmd/root.go`:
- In the PersistentPreRun, remove the calls that read the Identity from the service.
- Update the construction of the telemetry Service to use the new 1-field `RecorderConfig{Enabled: cfg.Enabled}` (no Endpoint, no AccountID, no InsertKey).
- The factory's rewrite (task 05-01-02) reads the build-time `var`s directly, so no env-var reads are needed in `cmd/root.go`.
</action>
<verify>
- `go build ./...` succeeds (no orphan references to the deleted `Identity` type).
- `go test -count=1 ./packages/cli/internal/telemetry/ ./packages/cli/cmd/` passes.
- `go vet ./...` is clean.
- `find packages/cli/internal/telemetry -name "identity*"` returns 0 files.
- The `Service` struct has no `Identity` field; `Service.New` does not call `LoadOrCreate`.
</verify>
<done>[ ]</done>
</task>

<task id="05-01-05">
<name>Add source-lock test for crypto/rand + update cmd/root_test.go to drop Identity assertions</name>
<files>packages/cli/internal/telemetry/recorder_test.go (or new test file), packages/cli/cmd/root_test.go</files>
<action>
Add a new test `TestNoLinkableIDSource` in the telemetry test package:
- Open the source files `event.go` and `recorder.go` (and any other file that might generate IDs).
- Use a regex to assert that any `rand.Read`, `crypto/rand.Read`, or `ulid.Make` call uses `crypto/rand` (not `math/rand`).
- The simplest form: grep the package source for `math/rand` and assert 0 matches.
- Also assert that no field in `Event` has the word "ID" except `EventID` (the per-event random). The dropped `InstallID` and `HostID` must not return.

Edit `cmd/root_test.go`:
- Remove the 2 test cases (or sub-tests) that assert on `Identity.InstallID` and `Identity.HostID`. The new `Service` has no `Identity` field; these tests would fail to compile if left.
- If the tests' bodies cannot be removed (e.g. they cover other behavior), rewrite them to assert on the new `Service` shape: enabled flag, no identity exposed.
</action>
<verify>
- `go test -count=1 ./packages/cli/internal/telemetry/ ./packages/cli/cmd/` passes.
- The new source-lock test is in the telemetry test package and passes.
- No test asserts on `Identity.InstallID` or `Identity.HostID` anywhere in the repo.
</verify>
<done>[ ]</done>
</task>

<task id="05-01-06">
<name>Run full pre-commit + verify all tests + lefthook</name>
<files>none (verification only)</files>
<action>
- Run `go build ./...` (must succeed).
- Run `go vet ./...` (must be clean).
- Run `go test -count=1 ./...` (must pass with 0 failures; new tests added, old tests removed).
- Run `pnpm run test:cli:e2e` (lefthook pre-commit hook must pass; the cli-e2e step is gated on staged file changes — it may skip, which is fine).
- Confirm the byte-for-byte schema test (`TestNewRelicRecorderSchemaByteForByte`) is the canonical assertion for the 5-field shape. The test passes.
- Confirm `grep -rn "install_id\|host_id" packages/cli/internal/telemetry/` returns 0 matches (no stray references).
- Confirm `grep -rn "install_id\|host_id" OBSERVABILITY.md` returns 0 matches (OBSERVABILITY is updated in plan 05-03; if plan 05-01 lands first, this grep will be a pre-existing match in OBSERVABILITY.md and is not a blocker for this plan).
- Commit with message: `refactor(05-01): 5-field Event schema, 2-way Recorder factory, identity module removed`.
</action>
<verify>
- All 4 commands above succeed (build, vet, test, e2e).
- 1 atomic commit created with the 5-field refactor in place.
- The commit message follows conventional commits.
</verify>
<done>[ ]</done>
</task>

---

*Plan: 05-01-recorder-core-refactor*
