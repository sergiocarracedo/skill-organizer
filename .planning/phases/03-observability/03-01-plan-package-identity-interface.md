---
wave: 1
depends_on: []
files_modified:
  - packages/cli/go.mod
  - packages/cli/go.sum
  - packages/cli/internal/telemetry/event.go
  - packages/cli/internal/telemetry/recorder.go
  - packages/cli/internal/telemetry/identity.go
  - packages/cli/internal/telemetry/event_test.go
  - packages/cli/internal/telemetry/recorder_test.go
  - packages/cli/internal/telemetry/identity_test.go
autonomous: true
single_layer_justified: true
single_layer_justified_reason: "Data/library-only plan: defines the telemetry package (Event, Recorder, Identity, factory). No cobra wire-up, no user-facing behavior change. The user-facing cobra surface arrives in plan 02; plan 01 is the testable library foundation it depends on."
requirement: REQ-8
objective: "Ship the new internal/telemetry package with the Event struct (7 fields), the Recorder interface, a NoopRecorder that drops events, an Identity type that produces 32-hex-char install_id/host_id via crypto/rand (deterministic when the rand source is injected), and a RecorderFactoryFunc package var. Verifiable by go test ./internal/telemetry/... and go build ./... succeeding with no callers yet — the cobra wire-up lives in plan 02."
must_haves:
  - "go build ./... succeeds"
  - "go test ./internal/telemetry/... passes (all event/recorder/identity tests green)"
  - "TestEventValidateFields_* (5 sub-cases) pass (rejects empty fields, rejects malformed install_id/host_id/event_id/timestamp)"
  - "TestNewEventIDProducesValidFormat passes (100 ULIDs, 26 Crockford base32 chars each)"
  - "TestNewTimestampFormat passes"
  - "TestEventJSONShape passes (7 keys, declaration order, types match)"
  - "TestNoopRecorderDropsEvents passes (1000 calls, no panic, no side effects)"
  - "TestRecorderFactoryReturnsNoopOnEmptyConfig passes (the default factory returns a NoopRecorder)"
  - "TestRecorderFactorySwapRoundtrip passes (custom factory is invoked, cleanup restores)"
  - "TestIdentityIs32HexChars passes (seeded rand via bytes.NewReader -> 32 hex chars, deterministic across 2 calls)"
  - "TestIdentityLoadOrCreateCreatesIfMissing passes (uses t.TempDir(), asserts install_id/host_id files exist after first call)"
  - "TestIdentityLoadOrCreateReusesIfPresent passes (two calls return identical IDs)"
  - "TestRotateHostIDChangesHostIDOnly passes (install_id unchanged, host_id differs)"
  - "github.com/oklog/ulid/v2 is in packages/cli/go.mod (added via go get, not hand-edited)"
  - "The cmd/ and config/ trees are untouched; only telemetry/ files and go.mod/go.sum are added"
---

# Plan 03-01: Telemetry package, Event, Recorder interface, Identity

## Objective

Add the new `internal/telemetry` package with the data and dependency-injection surface that REQ-8 needs. This plan ships the **library** layer only — the cobra subcommand and the on-disk buffer land in plan 02. The package exposes the `Event` struct (7 fields, snake_case JSON), a `Recorder` interface with a no-op default, an `Identity` type for `install_id`/`host_id`, and a swappable `RecorderFactoryFunc` for test injection. By the end of this plan, `go build ./...` succeeds and `go test ./internal/telemetry/...` is green; the package is ready to be wired into `cmd/root.go` in plan 02.

## Context

REQ-8 requires an opt-in, anonymous telemetry layer. The CONTEXT locks the 7-field schema (`command`, `exit_status`, `install_id`, `host_id`, `timestamp` RFC3339 UTC, `version`, `event_id` ULID), the two-ID identity model (`install_id` never rotates, `host_id` rotatable), and the factory pattern (a swappable func var) that mirrors `agenttools.ChooseAgentToolFunc` at `packages/cli/internal/agenttools/agenttools.go:172`. The RESEARCH recommends `crypto/rand.Read(buf[:16])` + `encoding/hex.EncodeToString` for the IDs and `github.com/oklog/ulid/v2` for the event ID.

This plan must add `oklog/ulid/v2` to `packages/cli/go.mod` via `go get` (do not hand-edit go.mod). `golang.org/x/term` is already an indirect dep (transitive from cobra), so the non-TTY check in plan 02 will not add a new module.

The cmd package is untouched in this plan — the cobra wire-up is plan 02's job. The package must compile standalone; no caller is required for plan 01 to be green. The `NewHTTPClientFunc` package var is a placeholder that plan 02 will use for the HTTPRecorder (which arrives in plan 02). Plan 01 ships the factory as "always noop" and `NewHTTPClientFunc` as "real client with a default 10s timeout" so callers can already use the interface.

## Tasks

<task id="03-01-01">
<name>Add the oklog/ulid/v2 dependency</name>
<files>
- packages/cli/go.mod
- packages/cli/go.sum
</files>
<action>
1. From `packages/cli/`, run:
   ```
   go get github.com/oklog/ulid/v2
   go mod tidy
   ```
2. Verify the dependency is now in `packages/cli/go.mod`. The output of `grep oklog packages/cli/go.mod` should contain at least one line for the direct or indirect require block. Either placement is acceptable (the dependency is wired correctly in both cases).
3. Verify the project's existing test suite still passes:
   ```
   go test ./...
   go vet ./...
   ```
4. Verify no other file has been modified by `go mod tidy`. The git diff should show only `go.mod` and `go.sum`.
</action>
<verify>
- `go list -m github.com/oklog/ulid/v2` exits 0 and prints a version
- `go test ./...` exits 0 (no regression)
- `go vet ./...` exits 0
- `git diff --stat` shows only `go.mod` and `go.sum` modified
</verify>
<done>[ ]</done>
</task>

<task id="03-01-02">
<name>Create event.go with Event struct, helpers, and Validate</name>
<files>
- packages/cli/internal/telemetry/event.go
</files>
<action>
Create `packages/cli/internal/telemetry/event.go` with the following structure:

1. Package declaration: `package telemetry`

2. Imports:
   - `crypto/rand` (not used in this file, but reserved for the future ID path)
   - `encoding/hex` (not used in this file)
   - `encoding/json`
   - `fmt`
   - `regexp`
   - `time`
   - `github.com/oklog/ulid/v2`

3. Define the 7-field Event struct with `json:"snake_case"` tags in the exact declaration order from CONTEXT:
   ```go
   type Event struct {
       Command    string `json:"command"`
       ExitStatus int    `json:"exit_status"`
       InstallID  string `json:"install_id"`
       HostID     string `json:"host_id"`
       Timestamp  string `json:"timestamp"`
       Version    string `json:"version"`
       EventID    string `json:"event_id"`
   }
   ```

4. Define regex package-level vars for validation (compile once):
   ```go
   var (
       idHexRe         = regexp.MustCompile(`^[0-9a-f]{32}$`)
       ulidRe          = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
       timestampRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
   )
   ```

5. Implement `(e *Event) Validate() error`:
   - Return `fmt.Errorf("event: command is required")` if `e.Command == ""`.
   - Return `fmt.Errorf("event: exit_status must be 0 or 1, got %d", e.ExitStatus)` if `e.ExitStatus != 0 && e.ExitStatus != 1`.
   - Return `fmt.Errorf("event: install_id must be 32 hex chars, got %q", e.InstallID)` if `!idHexRe.MatchString(e.InstallID)`.
   - Return `fmt.Errorf("event: host_id must be 32 hex chars, got %q", e.HostID)` if `!idHexRe.MatchString(e.HostID)`.
   - Return `fmt.Errorf("event: timestamp must be RFC3339 UTC, got %q", e.Timestamp)` if `!timestampRe.MatchString(e.Timestamp)`.
   - Return `fmt.Errorf("event: version is required")` if `e.Version == ""`.
   - Return `fmt.Errorf("event: event_id must be a 26-char ULID, got %q", e.EventID)` if `!ulidRe.MatchString(e.EventID)`.
   - Return nil.

6. Implement `NewEventID() string`:
   - Use `ulid.Make()` (the package default) and return `id.String()`.
   - The default `ulid.Make()` uses `ulid.DefaultEntropyMonotonicReader()` internally, but you can also call `ulid.MustNew(ulid.Timestamp(time.Now()), entropy)` if you want explicit monotonic entropy. Prefer the simpler `ulid.Make()` for plan 01; plan 02's HTTPRecorder will use the events as-is.
   - No error path (the only failure is the entropy source, which is wrapped in `Must`).

7. Implement `NewTimestamp() string`:
   - Return `time.Now().UTC().Format(time.RFC3339)`. RFC3339 already includes the `Z` suffix for UTC.
   - Note: `time.RFC3339` does not include sub-second precision; the regex requires exactly `T...:..:..Z`. Verify with `TestNewTimestampFormat` that the format is regex-compliant.

8. Add a one-line package comment above `package telemetry`:
   ```go
   // Package telemetry records anonymous, opt-in command-invocation events.
   // See OBSERVABILITY.md at the repo root for the schema, opt-in flow,
   // and data retention policy.
   ```

9. Add a doc comment on the Event struct that lists the 7 fields and references the schema doc.

10. NO usage of `crypto/rand` in this file. (The file is the schema layer; the random IDs are in `identity.go`.)
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The file's struct tags are inspectable: `gofmt -d packages/cli/internal/telemetry/event.go` is empty
- `TestEventValidateFields_*` (5 sub-cases) and `TestEventJSONShape`, `TestNewEventIDProducesValidFormat`, `TestNewTimestampFormat` all pass (assertions live in event_test.go, added in task 03-01-05)
</verify>
<done>[ ]</done>
</task>

<task id="03-01-03">
<name>Create recorder.go with Recorder interface, NoopRecorder, factory func var</name>
<files>
- packages/cli/internal/telemetry/recorder.go
</files>
<action>
Create `packages/cli/internal/telemetry/recorder.go` with the following structure:

1. Package: `package telemetry` (same package as event.go — they're co-located)

2. Imports:
   - `context`
   - `net/http`
   - `time`

3. Define the Recorder interface:
   ```go
   // Recorder is the sink for telemetry events. Implementations must be
   // safe to call from a single goroutine (the CLI runs in a single
   // goroutine per invocation). Errors are logged but never fatal.
   type Recorder interface {
       Record(ctx context.Context, event Event) error
   }
   ```

4. Define `NoopRecorder` as a value-receiver struct with a no-op `Record` method:
   ```go
   // NoopRecorder drops every event. It is the default factory return value
   // and the recorder used when telemetry is disabled or no endpoint is
   // configured. Per CONTEXT, this path must produce zero network egress.
   type NoopRecorder struct{}

   func (NoopRecorder) Record(_ context.Context, _ Event) error {
       return nil
   }
   ```

5. Define the `RecorderFactoryFunc` package var and the public `NewRecorder` wrapper:
   ```go
   // RecorderFactoryFunc is a swappable function variable for NewRecorder.
   // Tests reassign this to inject fakes; production code calls NewRecorder
   // (the wrapper) so the swap is transparent to callers.
   var RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }

   // NewRecorder returns the package's current Recorder implementation.
   // Plan 02 replaces the default factory to return an HTTPRecorder when
   // telemetry is enabled and an endpoint is configured.
   func NewRecorder() Recorder {
       return RecorderFactoryFunc()
   }
   ```

6. Define `NewHTTPClientFunc` package var (placeholder, used by plan 02's HTTPRecorder):
   ```go
   // NewHTTPClientFunc is a swappable function variable for the HTTP client
   // the HTTPRecorder uses to POST events. Plan 02 wires it. Tests can
   // swap it to return a client with a counting transport.
   var NewHTTPClientFunc = func() *http.Client {
       return &http.Client{Timeout: 10 * time.Second}
   }
   ```

7. Add a doc comment block on `NoopRecorder` referencing CONTEXT's "zero network egress" guarantee. Add a one-liner on `NewHTTPClientFunc` noting it is a placeholder for plan 02.

8. Do NOT add the HTTPRecorder type in this file — it arrives in plan 02. The `NewHTTPClientFunc` and `RecorderFactoryFunc` placeholders exist so plan 02 can mutate them without re-creating the file.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The file contains a `Recorder` interface, a `NoopRecorder` value type, a `RecorderFactoryFunc` package var, a `NewRecorder` wrapper, and a `NewHTTPClientFunc` package var. (No HTTPRecorder yet — that is plan 02.)
- `TestNoopRecorderDropsEvents`, `TestRecorderFactoryReturnsNoopOnEmptyConfig`, `TestRecorderFactorySwapRoundtrip` all pass (assertions in recorder_test.go, added in task 03-01-06)
</verify>
<done>[ ]</done>
</task>

<task id="03-01-04">
<name>Create identity.go with Identity, LoadOrCreate, RotateHostID</name>
<files>
- packages/cli/internal/telemetry/identity.go
</files>
<action>
Create `packages/cli/internal/telemetry/identity.go` with the following structure:

1. Package: `package telemetry`

2. Imports:
   - `bytes`
   - `crypto/rand`
   - `encoding/hex`
   - `fmt`
   - `io`
   - `os`
   - `path/filepath`
   - `regexp`

3. Constants and paths:
   ```go
   const (
       installIDFile = "install_id"
       hostIDFile    = "host_id"
   )
   ```

4. Define the `Identity` struct:
   ```go
   // Identity is the pair of anonymous IDs associated with this CLI install.
   // InstallID is stable across re-installs; HostID is rotatable via
   // RotateHostID. Neither is tied to a machine fingerprint, hostname,
   // username, or IP.
   type Identity struct {
       InstallID string
       HostID    string
   }
   ```

5. Define the unexported `idRegex` (32 hex chars):
   ```go
   var idRegex = regexp.MustCompile(`^[0-9a-f]{32}$`)
   ```

6. Implement the unexported test seam `generateID(r io.Reader) (string, error)`:
   ```go
   func generateID(r io.Reader) (string, error) {
       var buf [16]byte
       if _, err := io.ReadFull(r, buf[:]); err != nil {
           return "", fmt.Errorf("read random bytes: %w", err)
       }
       return hex.EncodeToString(buf[:]), nil
   }
   ```
   The production `LoadOrCreate` calls this with `crypto_rand.Reader`. Tests use `bytes.NewReader` to get a deterministic output.

7. Implement `(i *Identity) writeFiles(appDir string) error`:
   - Use `os.MkdirAll(appDir, 0o755)` (the AppDir may not exist on a fresh install).
   - Write `i.InstallID` to `filepath.Join(appDir, installIDFile)` with `os.WriteFile`, mode `0o644`.
   - Write `i.HostID` to `filepath.Join(appDir, hostIDFile)` with `os.WriteFile`, mode `0o644`.
   - Return any wrapped error.

8. Implement the unexported `loadIDFile(path string) (string, error)`:
   - Read the file with `os.ReadFile`.
   - If `os.IsNotExist(err)` is true, return `("", nil)` (caller decides whether to create).
   - `strings.TrimSpace` the content.
   - If the trimmed content does not match `idRegex`, regenerate from `crypto_rand.Reader`, write the file, and return the new ID with a `nil` error. (Per CONTEXT: "if not, regenerates and warns." We don't have a logger here; the caller can check whether the returned ID differs from the on-disk content if it wants a warning. This is acceptable: identity corruption is a rare, recoverable condition.)

9. Implement `LoadOrCreate(appDir string) (Identity, error)`:
   - Call `loadIDFile` for both `install_id` and `host_id`.
   - For each, if the result is empty, generate a new ID via `generateID(crypto_rand.Reader)`.
   - Build the `Identity` struct.
   - Call `writeFiles(appDir)` to persist any new IDs.
   - Return the struct.

10. Implement `RotateHostID(appDir string) (string, error)`:
    - Generate a new host_id via `generateID(crypto_rand.Reader)`.
    - Write it to `filepath.Join(appDir, hostIDFile)`.
    - Return the new ID and any error.

11. Do NOT export any function that takes a `io.Reader` for `LoadOrCreate` or `RotateHostID`. The unexported `generateID` is the only test seam; production always uses `crypto_rand.Reader`. (Per RESEARCH P4: `crypto/rand.Read` panic on broken systems is the correct behaviour — the system is unsafe for any crypto operation, so panicking is appropriate.)

12. Add doc comments on `Identity`, `LoadOrCreate`, and `RotateHostID` that reference CONTEXT's "no PII" guarantee.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The file exports `Identity`, `LoadOrCreate`, `RotateHostID` and unexported `generateID`, `loadIDFile`, `idRegex`, `installIDFile`, `hostIDFile`.
- The tests in identity_test.go (added in task 03-01-07) all pass.
</verify>
<done>[ ]</done>
</task>

<task id="03-01-05">
<name>Add event_test.go with Validate, JSON shape, ULID, timestamp tests</name>
<files>
- packages/cli/internal/telemetry/event_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/event_test.go` (package `telemetry`). Add the following test functions:

1. `TestEventValidateFields(t *testing.T)` — table-driven, 5 sub-cases via `t.Run`:
   - `"empty command"` — `Event{Command: ""}` (other fields set to a valid baseline) → expects error containing "command is required".
   - `"invalid exit_status"` — `ExitStatus: 2` → expects error containing "exit_status must be 0 or 1".
   - `"malformed install_id"` — `InstallID: "not-hex"` → expects error containing "install_id must be 32 hex chars".
   - `"malformed timestamp"` — `Timestamp: "yesterday"` → expects error containing "timestamp must be RFC3339 UTC".
   - `"malformed event_id"` — `EventID: "abc"` → expects error containing "event_id must be a 26-char ULID".

   Use a helper `validEvent()` that returns a fully-valid `Event` (all 7 fields set correctly) and override one field per sub-case.

2. `TestEventValidateAcceptsValid(t *testing.T)` — call `validEvent().Validate()` and assert no error. This is the negative control.

3. `TestEventJSONShape(t *testing.T)`:
   - Build a valid `Event` and call `json.Marshal`.
   - Parse the result into a `map[string]any`.
   - Assert the map has exactly 7 keys: `"command"`, `"exit_status"`, `"install_id"`, `"host_id"`, `"timestamp"`, `"version"`, `"event_id"`.
   - Assert the field values have the expected JSON types: `command`, `install_id`, `host_id`, `timestamp`, `version`, `event_id` are strings; `exit_status` is a number (float64 in Go's json package).
   - Assert the JSON key order matches the struct declaration order: re-marshal into a `[]byte` and check the byte offset of each key. (Simpler alternative: assert the bytes contain `"command":"<val>","exit_status":<val>,"install_id":"<val>",...` in that order by string match. The struct-declaration-order guarantee from `encoding/json` is what we're testing.)

4. `TestNewEventIDProducesValidFormat(t *testing.T)`:
   - Loop 100 times: `id := NewEventID()`.
   - Assert `len(id) == 26`.
   - Assert each ID matches the ULID Crockford base32 regex `^[0-9A-HJKMNP-TV-Z]{26}$`.
   - Assert no two consecutive IDs are identical (the entropy source is `crypto/rand` so this is overwhelmingly likely; the test catches a regression where the entropy source is replaced with a constant).

5. `TestNewTimestampFormat(t *testing.T)`:
   - Call `NewTimestamp()`.
   - Assert it matches `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`.
   - Assert it parses via `time.Parse(time.RFC3339, ts)` with no error and the parsed time is in UTC (location == time.UTC).

6. NO use of `testify` or any external assertion library. Use plain `t.Errorf` / `t.Fatalf`.

7. NO use of `t.Parallel()` — the tests are fast and there is no shared mutable state.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestEventValidate -v` runs the 5 sub-cases and the negative control, all pass
- `go test ./internal/telemetry/... -run TestEventJSONShape -v` passes
- `go test ./internal/telemetry/... -run TestNewEventID -v` passes (100 ULIDs validated)
- `go test ./internal/telemetry/... -run TestNewTimestamp -v` passes
- `go test ./internal/telemetry/...` exits 0 (all tests in the package pass)
</verify>
<done>[ ]</done>
</task>

<task id="03-01-06">
<name>Add recorder_test.go with NoopRecorder drops and factory-swap tests</name>
<files>
- packages/cli/internal/telemetry/recorder_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/recorder_test.go` (package `telemetry`). Add the following test functions:

1. `TestNoopRecorderDropsEvents(t *testing.T)`:
   - Construct `rec := NoopRecorder{}`.
   - Call `rec.Record(context.Background(), Event{Command: "x", InstallID: "0123456789abcdef0123456789abcdef", HostID: "0123456789abcdef0123456789abcdef", Timestamp: "2026-06-11T00:00:00Z", Version: "1.0.0", EventID: "01HXYZABCDEFGHJKMNPQRSTVWX"})` 1000 times.
   - Assert every call returns nil.
   - Assert the test did not panic (it doesn't, by virtue of the loop completing).
   - This is the smoke test for the no-op path: if `NoopRecorder` were ever changed to allocate, log, or call out to a network sink, this test would still pass — but the byte-for-byte and counting-transport tests in plan 03 are the ones that catch actual network egress.

2. `TestRecorderFactoryReturnsNoopOnEmptyConfig(t *testing.T)`:
   - Save the current `RecorderFactoryFunc`, set it to the production default (assigning the default func literal).
   - Call `NewRecorder()` and assert the returned value is a `NoopRecorder` (use a type switch or a `_, ok := rec.(NoopRecorder); assert ok`).
   - Restore the factory in `t.Cleanup`.

3. `TestRecorderFactorySwapRoundtrip(t *testing.T)`:
   - Define a `fakeRecorder` value type with a `Record` method that records events to a `[]Event` field (you can use a local `*fakeRecorder` with a method on pointer, or a value type with a slice in a captured closure). The test asserts the factory was invoked.
   - Save the current `RecorderFactoryFunc`, assign a new func that returns a `*fakeRecorder`.
   - Call `NewRecorder()`, assert the returned value is a `*fakeRecorder` (type assertion).
   - Call `rec.Record(ctx, validEvent())` and assert the fake captured the event.
   - Restore the original factory in `t.Cleanup`.

4. NO use of `testify`. Use `t.Errorf` and `t.Fatalf` directly. NO use of `t.Parallel()`.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestNoopRecorder -v` passes
- `go test ./internal/telemetry/... -run TestRecorderFactory -v` passes (both sub-tests)
- `go test ./internal/telemetry/...` exits 0
</verify>
<done>[ ]</done>
</task>

<task id="03-01-07">
<name>Add identity_test.go with hex format, round-trip, and rotation tests</name>
<files>
- packages/cli/internal/telemetry/identity_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/identity_test.go` (package `telemetry`). Add the following test functions:

1. `TestIdentityIs32HexChars(t *testing.T)`:
   - Build a `bytes.NewReader([]byte{0x01, 0x02, ...})` with 32 bytes of known content (deterministic).
   - Call `generateID(reader)` twice.
   - Assert the two calls return identical strings (because the reader is deterministic).
   - Assert the result matches `^[0-9a-f]{32}$` and is 32 characters long.
   - Assert the result is the hex encoding of the input bytes.

2. `TestIdentityLoadOrCreateCreatesIfMissing(t *testing.T)`:
   - Use `appDir := t.TempDir()`.
   - Call `LoadOrCreate(appDir)`.
   - Assert the result has a 32-hex InstallID and a 32-hex HostID.
   - Assert `filepath.Join(appDir, installIDFile)` and `filepath.Join(appDir, hostIDFile)` exist.
   - Assert the file contents match the returned IDs (after trim).

3. `TestIdentityLoadOrCreateReusesIfPresent(t *testing.T)`:
   - Use `appDir := t.TempDir()`.
   - Call `LoadOrCreate(appDir)` once; record the IDs.
   - Call `LoadOrCreate(appDir)` again; assert both IDs are identical to the first call.

4. `TestRotateHostIDChangesHostIDOnly(t *testing.T)`:
   - Use `appDir := t.TempDir()`.
   - Call `LoadOrCreate(appDir)`; record install and host.
   - Call `RotateHostID(appDir)`; record the new host.
   - Assert the new host is 32 hex chars and is NOT equal to the old host.
   - Re-call `LoadOrCreate(appDir)`; assert the install_id is unchanged but the host_id equals the new one.

5. `TestLoadIDFileRegeneratesCorrupted(t *testing.T)`:
   - Use `appDir := t.TempDir()`.
   - Write a corrupted value (e.g. `"not-hex-garbage"`) to `filepath.Join(appDir, installIDFile)`.
   - Call `LoadOrCreate(appDir)`.
   - Assert the returned InstallID is a fresh 32-hex value (not the corrupted input).
   - Assert the file on disk now contains the fresh value (the corrupted file was overwritten).

6. `TestLoadOrCreateCreatesAppDir(t *testing.T)`:
   - Use `appDir := filepath.Join(t.TempDir(), "nested", "config")` (the dir does not exist).
   - Call `LoadOrCreate(appDir)`.
   - Assert `os.Stat(appDir)` returns no error (the dir was created).

7. `TestRotateHostIDRegeneratesOnCall(t *testing.T)`:
   - Use `appDir := t.TempDir()`.
   - Call `RotateHostID(appDir)` twice.
   - Assert the two returned IDs are different (the entropy is fresh on each call; statistically the chance of collision in 80 bits is negligible).
   - Assert the second call's ID is what `LoadOrCreate(appDir)` returns for the host.

8. NO use of `testify`. NO use of `t.Parallel()`.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestIdentity -v` passes (7 tests, all green)
- `go test ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The test file does not import any package other than `bytes`, `os`, `path/filepath`, `regexp`, `testing`, and the package-under-test
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/telemetry/...` passes
- [ ] `go test ./...` passes (no regression)
- [ ] `go vet ./...` passes
- [ ] `TestEventValidateFields_*` (5 sub-cases) all pass
- [ ] `TestEventValidateAcceptsValid` passes
- [ ] `TestEventJSONShape` passes
- [ ] `TestNewEventIDProducesValidFormat` passes (100 ULIDs validated)
- [ ] `TestNewTimestampFormat` passes
- [ ] `TestNoopRecorderDropsEvents` passes (1000 calls, no panic)
- [ ] `TestRecorderFactoryReturnsNoopOnEmptyConfig` passes
- [ ] `TestRecorderFactorySwapRoundtrip` passes
- [ ] `TestIdentityIs32HexChars` passes (seeded rand, deterministic)
- [ ] `TestIdentityLoadOrCreateCreatesIfMissing` passes
- [ ] `TestIdentityLoadOrCreateReusesIfPresent` passes
- [ ] `TestRotateHostIDChangesHostIDOnly` passes
- [ ] `TestLoadIDFileRegeneratesCorrupted` passes
- [ ] `TestLoadOrCreateCreatesAppDir` passes
- [ ] `TestRotateHostIDRegeneratesOnCall` passes
- [ ] `github.com/oklog/ulid/v2` is in `packages/cli/go.mod`
- [ ] No file in `packages/cli/cmd/` or `packages/cli/internal/config/` is modified

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/go.mod packages/cli/go.sum`
2. Remove untracked: `rm -rf packages/cli/internal/telemetry/`
3. Verify: `go build ./...` and `go test ./...` pass on the reverted state
4. Retry with smaller scope:
   - First, add the dependency only (task 01-01), confirm `go.mod` and `go.sum` update cleanly.
   - Then create `event.go` + `event_test.go`, run tests.
   - Then create `recorder.go` + `recorder_test.go`, run tests.
   - Then create `identity.go` + `identity_test.go`, run tests.

The package is self-contained; a partial revert of any single file (event/recorder/identity) does not affect the others. The dependency is only used by `event.go` (for `oklog/ulid`), so removing `event.go` also allows `go.mod` to be re-tidied to drop the dependency.

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | `oklog/ulid` import path is wrong (e.g., `github.com/oklog/ulid` without the `/v2` suffix) — `go get` resolves to the v0 series or fails. | Low | High | The task uses `go get github.com/oklog/ulid/v2` (the v2 module path, which is the de-facto version as of 2025). The verification step `go list -m github.com/oklog/ulid/v2` confirms the import. |
| 2 | `LoadOrCreate` uses `crypto/rand.Reader` directly and is not testable — the test cannot inject a seeded rand. | Medium | Medium | The unexported `generateID(randReader io.Reader)` helper is the test seam: tests in the same package call it directly with `bytes.NewReader(...)`. The exported `LoadOrCreate` always uses `crypto/rand.Reader` (production behaviour). |
| 3 | Field order in `Event` struct drifts from CONTEXT (e.g., a future refactor swaps `Version` and `EventID`), breaking the byte-for-byte test in plan 03. | Low | High | A `TestEventJSONShape` test asserts the JSON marshalling produces keys in declaration order. Future edits that reorder fields will fail the test. |
| 4 | `crypto/rand.Read` panics in test environments without `/dev/urandom` (containers, minimal Linux). | Very Low | Medium | Per RESEARCH P4, the panic is correct and not wrapped. Tests in plan 01 use the unexported `generateID` with `bytes.NewReader`, so they never touch `crypto/rand`. The production `LoadOrCreate` only calls `crypto/rand` on real runs. |
| 5 | `RecorderFactoryFunc` is reassigned by a test and not restored, leaking into the next test. | Low | High | The factory-swap test uses `t.Cleanup` to restore. Plan 03's counting-transport test will follow the same pattern. The cleanup is part of the test action text. |
| 6 | `Identity.InstallID` written to disk contains a trailing newline; the read path's `strings.TrimSpace` strips it. But a future refactor that drops `TrimSpace` would silently break the regex check. | Low | Medium | The `loadIDFile` helper always trims. The test `TestIdentityLoadOrCreateReusesIfPresent` exercises the round-trip; if the trim is dropped, the test fails. |
| 7 | The `go mod tidy` in task 01-01 demotes `oklog/ulid/v2` to indirect (because it's only used in `event.go` and the only direct importer is `event.go`). The dependency is still functional, but a future reviewer may be confused. | Low | Low | Documented in the must-haves: "direct-require block contains oklog/ulid/v2 (or the indirect block — both are fine)". The dependency is required either way. |

## Commit Message

```
feat(cli): add internal/telemetry package with Event, Recorder, Identity

- New packages/cli/internal/telemetry/ package as the foundation
  for REQ-8 opt-in anonymous telemetry
- Event struct with the 7 fields from the CONTEXT schema (command,
  exit_status, install_id, host_id, timestamp, version, event_id)
  in declaration order, snake_case JSON tags, with a Validate
  method that asserts regex shapes for the IDs and timestamp
- Recorder interface (single Record method) with a NoopRecorder
  value receiver and a swappable RecorderFactoryFunc for test
  injection; NewRecorder() is the public wrapper
- Identity type with LoadOrCreate (generates two 32-hex-char
  random IDs via crypto/rand) and RotateHostID (rotates only
  the host ID, preserves install_id), with on-disk files
  <appDir>/install_id and <appDir>/host_id
- NewEventID and NewTimestamp helpers for the record path
- 19 unit tests covering Event validation, JSON shape, ULID
  format, NoopRecorder drops, factory swap, Identity round-trip,
  corruption recovery, and host_id rotation
- oklog/ulid/v2 added to go.mod (no callers yet; the buffer
  and HTTPRecorder arrive in plan 02)
```
