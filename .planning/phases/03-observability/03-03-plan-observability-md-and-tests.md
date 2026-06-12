---
wave: 3
depends_on:
  - 03-01-plan-package-identity-interface.md
  - 03-02-plan-buffer-http-cobra.md
files_modified:
  - OBSERVABILITY.md
  - packages/cli/internal/telemetry/recorder_test.go
  - packages/cli/internal/telemetry/observability_test.go
  - packages/cli/e2e_test.go
autonomous: true
single_layer_justified: true
single_layer_justified_reason: "Documentation + tests only: OBSERVABILITY.md (7 sections), byte-for-byte schema test against the doc's example payload, counting-transport zero-egress test, and 3 e2e tests. No new user-facing CLI surface; the cobra wire-up is in plan 02."
requirement: REQ-8
objective: "Ship the 7-section OBSERVABILITY.md at the repo root (data collected, schema, endpoint, opt-in/out, retention, security, contact), the byte-for-byte httptest schema test for HTTPRecorder, the counting-transport zero-egress test for NoopRecorder / disabled state, the byte-for-byte OBSERVABILITY example-payload test, and the e2e test that the first-run prompt fires only on the first run. Verifiable by go test ./... passing and the OBSERVABILITY.md example block matching the recorder's JSON output (modulo the four volatile fields)."
must_haves:
  - "OBSERVABILITY.md exists at the repo root with 7 sections (What is collected, Schema, How to enable / disable, Endpoint configuration, Data retention, Privacy guarantees, FAQ)"
  - "OBSERVABILITY.md schema example payload matches the recorder's output byte-for-byte (modulo the four volatile fields: install_id, host_id, event_id, timestamp)"
  - "TestHTTPRecorderSchemaByteForByte passes (uses httptest.NewServer, captures raw POST body, asserts 7 top-level keys, asserts the four volatile fields match their regexes, asserts the other three fields match a fixed value)"
  - "TestNoopRecorderNoNetworkCalls passes (counting transport wraps http.DefaultTransport, 100 Record calls, counter == 0)"
  - "TestRecorderFactoryReturnsNoopWhenEndpointEmpty passes (cfg.Enabled=true, cfg.Endpoint='' -> factory returns NoopRecorder; counter == 0)"
  - "TestRecorderFactoryReturnsHTTPRecorderWhenConfigured passes (cfg.Enabled=true, cfg.Endpoint=non-empty -> factory returns HTTPRecorder; the httptest server receives the POST)"
  - "TestOBSERVABILITYExampleMatchesEmitted passes (parses the example block in OBSERVABILITY.md, substitutes the four volatile fields with regex placeholders, and asserts the recorder's output matches)"
  - "TestTelemetryFirstRunPromptFiresOnce in e2e_test.go passes (builds the binary, runs it twice with XDG_CONFIG_HOME pointing to a temp dir, asserts the first run shows the prompt and writes the YAML, the second run does not show the prompt)"
  - "TestTelemetryDisabledNoBuffer in e2e_test.go passes (telemetry.enabled=false in YAML, run a command, assert <appDir>/telemetry-buffer.jsonl is not created)"
  - "go test ./... passes (no regression on plans 01 and 02)"
---

# Plan 03-03: OBSERVABILITY.md, byte-for-byte schema test, e2e first-run prompt

## Objective

Close out REQ-8 with the user-facing documentation and the strongest possible test coverage. This plan ships a 7-section `OBSERVABILITY.md` at the repo root, three hardened recorder tests (byte-for-byte schema via `httptest.NewServer`, counting-transport zero-egress, factory short-circuits), a test that the example payload in the doc matches the recorder's output exactly, and two e2e tests that exercise the first-run prompt and the disabled-no-buffer state via the built binary. By the end, REQ-8's acceptance criteria are observably met: a new user reading `OBSERVABILITY.md` knows exactly what the schema is, what the opt-in flow is, and what the data-retention story is.

## Context

Plans 01 and 02 shipped the telemetry package, the cobra wire-up, the on-disk buffer, the first-run prompt, and the `telemetry` subcommand. This plan adds the documentation and the integration tests that prove the wire format is stable.

The OBSERVABILITY.md doc is referenced verbatim in plan 02's must-haves (the `TestOBSERVABILITYExampleMatchesEmitted` test in this plan parses the example block in the doc). The 7 sections are: What is collected, Schema, How to enable / disable, Endpoint configuration, Data retention, Privacy guarantees, FAQ (per CONTEXT's `OBSERVABILITY.md` doc shape section). The example payload in the Schema section must match the recorder's JSON output exactly (modulo the 4 volatile fields).

The byte-for-byte test uses `httptest.NewServer` to capture the raw POST body, parses it as JSON, and asserts the 7 top-level keys plus the shape of each value. This is the strongest possible schema assertion short of running the recorder against the production server (which doesn't exist yet). The counting-transport test wraps `http.DefaultTransport` with a counter, calls `Record` 100 times on a `NoopRecorder`, and asserts the counter is 0. This is the "zero network egress when disabled" acceptance criterion from REQUIREMENTS.md REQ-8, encoded as a runnable test.

The e2e tests build the binary (existing pattern in `e2e_test.go`), set `XDG_CONFIG_HOME` to a temp dir, and run the CLI as a child process. The first-run prompt test asserts the prompt fires only on the first run (the sentinel file gates subsequent runs). The disabled-no-buffer test asserts that when `telemetry.enabled: false` is in the YAML, the buffer file is never created.

This is the closing plan. The 14 must-haves from `STATE.md` for P1 + P2 + P3 are all satisfied by the end of this plan.

## Tasks

<task id="03-03-01">
<name>Create OBSERVABILITY.md at the repo root with 7 sections</name>
<files>
- OBSERVABILITY.md
</files>
<action>
Create `OBSERVABILITY.md` at the repo root (one level above `packages/cli/`). The doc is 7 sections per CONTEXT. Keep it under 200 lines — every section is short and direct, no marketing prose.

Structure:

```
# Observability (REQ-8)

> Anonymous, opt-in telemetry for `skill-organizer`. Disabled by default.
> No args, no paths, no PII. Schema and endpoint are documented below.

## What is collected

For each command invocation, we record exactly 7 fields, sent as a single
JSON object:

- `command` — the cobra subcommand name (e.g. `check-security`, `enable`).
- `exit_status` — `0` on success, `1` on error.
- `install_id` — 32 hex chars. Stable across re-installs.
- `host_id` — 32 hex chars. Rotatable via `skill-organizer telemetry rotate-host-id`.
- `timestamp` — RFC3339 UTC, e.g. `2026-06-11T12:34:56Z`.
- `version` — CLI semver, e.g. `0.4.0`.
- `event_id` — 26-char ULID for server-side de-duplication.

We do NOT collect: command arguments, file paths, environment variables,
machine fingerprints, hostnames, IP addresses, skill content, or any
other data.

## Schema

A single POST to the endpoint. Content-Type: `application/json`.

Example payload (the four volatile fields are placeholders in this doc;
the recorder's real output matches byte-for-byte modulo those four fields):

```json
{
  "command": "check-security",
  "exit_status": 0,
  "install_id": "0123456789abcdef0123456789abcdef",
  "host_id": "fedcba9876543210fedcba9876543210",
  "timestamp": "2026-06-11T12:34:56Z",
  "version": "0.4.0",
  "event_id": "01HXYZABCDEFGHJKMNPQRSTVWX"
}
```

The 4 volatile fields (install_id, host_id, event_id, timestamp) match:
- `install_id` / `host_id`: `^[0-9a-f]{32}$`
- `event_id`: `^[0-9A-HJKMNP-TV-Z]{26}$` (Crockford base32 ULID)
- `timestamp`: `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`

The other 3 fields (`command`, `exit_status`, `version`) are deterministic
and asserted byte-for-byte in the integration test.

## How to enable / disable

Three layers, in order of precedence (highest wins):

1. **CLI flag** — `--telemetry-endpoint=https://example.com/in`
2. **Env var** — `SKILL_ORGANIZER_TELEMETRY_ENDPOINT=https://example.com/in`
3. **YAML** — `telemetry: { enabled: true, endpoint: "https://example.com/in" }`
   in `~/.config/skill-organizer/skill-organizer.yml`.

The first run of the binary asks for consent interactively (TTY only).
The default is **no**. The answer is sticky: the prompt does not fire on
subsequent runs. To re-prompt, delete `<appDir>/telemetry-prompted`.

Subcommands:
- `skill-organizer telemetry enable` — writes `telemetry.enabled: true`
- `skill-organizer telemetry disable` — writes `telemetry.enabled: false` and clears the buffer
- `skill-organizer telemetry status` — prints the current state
- `skill-organizer telemetry rotate-host-id` — re-rolls the host_id

## Endpoint configuration

See "How to enable / disable" for the three-layer precedence. The endpoint
MUST be set for events to be sent; if no endpoint is configured, the
factory returns a `NoopRecorder` that drops events with zero network
egress, regardless of the `enabled` flag.

The default endpoint is empty. The first run with a configured endpoint
prompts for consent; the prompt is skipped on non-TTY (CI, piped input)
and the default ("no") is NOT persisted, so the next TTY run re-prompts.

## Data retention

The on-disk buffer lives at `<AppDir>/telemetry-buffer.jsonl` and is
capped at 1 MB. When the buffer exceeds 1 MB, the oldest events are
dropped (FIFO eviction). The buffer is drained opportunistically on
each run: events are sent, and on success the file is truncated; on
network failure, the unsent events are preserved for the next run.

Server-side retention is **TBD by the server owner**. The CLI does not
ship a server; the team will publish a retention policy when the
server is stood up. Until then, treat the endpoint as "your data, your
responsibility".

## Privacy guarantees

- No args, no paths, no PII. Only the 7 fields above.
- `install_id` and `host_id` are 16 random bytes each, generated via
  `crypto/rand`. They are not derived from the machine, hostname,
  username, or IP address.
- `host_id` is rotatable via `telemetry rotate-host-id`. Deleting
  `<AppDir>/host_id` also regenerates the ID on the next run.
- Disabling telemetry (`telemetry.enabled: false` or
  `telemetry disable`) stops all network egress on the telemetry path.
  The disabled state is asserted at runtime by the
  `TestNoopRecorderNoNetworkCalls` test (counting transport).

## FAQ

**Q: What happens when I'm offline?**
Events are appended to `<AppDir>/telemetry-buffer.jsonl`. The next run
with network connectivity drains the buffer.

**Q: How do I inspect the buffer?**
`ls -la ~/.config/skill-organizer/telemetry-buffer.jsonl`. The file is
JSONL, one event per line.

**Q: How do I verify zero network egress?**
Run `telemetry status` — the buffer size in bytes is reported. When
disabled, the size stays at 0 across runs.

**Q: How do I opt out?**
`skill-organizer telemetry disable` — also clears the buffer.

**Q: Where is the consent sentinel?**
`<AppDir>/telemetry-prompted`. Delete it to re-prompt on the next TTY run.
```

The example payload in the Schema section is the contract. The test in
task 03-03-04 reads this exact block and asserts the recorder's output
matches (modulo the 4 volatile fields).

Length target: ~150 lines. The example block is the longest single
element; the FAQ is 5 Q&A pairs.
</action>
<verify>
- `OBSERVABILITY.md` exists at the repo root
- `wc -l OBSERVABILITY.md` is between 140 and 200
- The 7 section headers are present: `## What is collected`, `## Schema`, `## How to enable / disable`, `## Endpoint configuration`, `## Data retention`, `## Privacy guarantees`, `## FAQ`
- The example JSON block contains exactly 7 top-level keys
- The `TestOBSERVABILITYExampleMatchesEmitted` test (task 03-03-04) parses the block successfully
</verify>
<done>[ ]</done>
</task>

<task id="03-03-02">
<name>Add HTTPRecorder byte-for-byte schema test with httptest.NewServer</name>
<files>
- packages/cli/internal/telemetry/recorder_test.go
</files>
<action>
Append to `packages/cli/internal/telemetry/recorder_test.go` (the file already has the plan 01 NoopRecorder tests and the plan 02 `TestHTTPRecorderSmokeOK` and `TestHTTPRecorderFailureStatus`).

1. `TestHTTPRecorderSchemaByteForByte(t *testing.T)`:
   - Build a `validEvent()` helper that returns a fully-valid `Event` with:
     - `Command: "check-security"`, `ExitStatus: 0`, `Version: "0.4.0"`
     - `InstallID: "0123456789abcdef0123456789abcdef"`, `HostID: "fedcba9876543210fedcba9876543210"`
     - `Timestamp: "2026-06-11T12:34:56Z"`, `EventID: "01HXYZABCDEFGHJKMNPQRSTVWX"`
   - Use `httptest.NewServer` with a handler that captures the raw body into a `bytes.Buffer`.
   - Build the recorder: `rec := HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}`.
   - Call `rec.Record(ctx, validEvent())`.
   - Assert no error.
   - Assert the captured body is valid JSON (`json.Valid(body)`).
   - Unmarshal into `map[string]any`; assert exactly 7 keys.
   - Assert the value of `command == "check-security"`, `exit_status == float64(0)` (Go's json package), `version == "0.4.0"`.
   - Assert `install_id` and `host_id` match `^[0-9a-f]{32}$`.
   - Assert `event_id` matches `^[0-9A-HJKMNP-TV-Z]{26}$`.
   - Assert `timestamp` matches `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`.
   - Assert the Content-Type header was `application/json`.
   - Assert the HTTP method was POST.

2. `TestHTTPRecorderSchemaFieldOrder(t *testing.T)`:
   - Same setup as above, but inspect the raw body as a string.
   - Assert the body contains the keys in the exact order: `command`, `exit_status`, `install_id`, `host_id`, `timestamp`, `version`, `event_id`.
   - This is the structural test: the byte-for-byte match in the doc.

3. `TestHTTPRecorderFieldCount(t *testing.T)`:
   - Same setup; assert the JSON has exactly 7 top-level keys (no more, no less).
   - This catches "we added a field by accident" regressions.

4. NO use of `testify`. NO use of `t.Parallel()`. The `validEvent` helper is shared with the other tests in the file.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestHTTPRecorder -v` exits 0 (smoke + failure-status + byte-for-byte + field-order + field-count, all 5 pass)
- `go test ./internal/telemetry/...` exits 0
- The byte-for-byte test asserts the exact 3 deterministic fields (`command`, `exit_status`, `version`) and the regex shape of the 4 volatile fields
</verify>
<done>[ ]</done>
</task>

<task id="03-03-03">
<name>Add counting-transport zero-egress test for NoopRecorder and disabled state</name>
<files>
- packages/cli/internal/telemetry/recorder_test.go
</files>
<action>
Append to `packages/cli/internal/telemetry/recorder_test.go`:

1. Define a `countingTransport` type:
   ```go
   type countingTransport struct {
       calls atomic.Int64
   }
   func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
       c.calls.Add(1)
       return &http.Response{
           StatusCode: 200,
           Body:       io.NopCloser(strings.NewReader("")),
           Header:     make(http.Header),
       }, nil
   }
   ```
   (Imports: `io`, `strings`, `sync/atomic`. The package is already imported; `strings` may need to be added.)

2. `TestNoopRecorderNoNetworkCalls(t *testing.T)`:
   - Build a `countingTransport{}` and a client that uses it: `client := &http.Client{Transport: &counter}`.
   - Wrap the NoopRecorder in a loop: call `NoopRecorder{}.Record(ctx, validEvent())` 100 times.
   - Assert `counter.calls.Load() == 0`.
   - This is the "NoopRecorder drops events, never touches the network" assertion. Per CONTEXT, the NoopRecorder is the default factory return value and MUST produce zero network egress.

3. `TestRecorderFactoryReturnsNoopWhenEndpointEmpty(t *testing.T)`:
   - Save the current `RecorderFactoryFunc`; restore in `t.Cleanup`.
   - Call `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: ""})`.
   - Assert `NewRecorder()` returns a `NoopRecorder` value (type assertion).
   - This is the "even with `enabled: true`, no endpoint means noop" assertion from CONTEXT.

4. `TestRecorderFactoryReturnsHTTPRecorderWhenConfigured(t *testing.T)`:
   - Save the current `RecorderFactoryFunc`; restore in `t.Cleanup`.
   - Use `httptest.NewServer` with a handler that returns 200.
   - Call `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: srv.URL})`.
   - Assert `NewRecorder()` returns an `HTTPRecorder` value (type assertion).
   - Call `rec.Record(ctx, validEvent())`; assert no error; assert the server handler was called (capture a counter on the handler).
   - This is the "factory wires the configured endpoint" assertion.

5. `TestRecorderFactoryReturnsNoopWhenDisabled(t *testing.T)`:
   - Save the current `RecorderFactoryFunc`; restore in `t.Cleanup`.
   - Call `SetDefaultFactory(RecorderConfig{Enabled: false, Endpoint: "https://example.com"})` (endpoint set, but disabled).
   - Assert `NewRecorder()` returns a `NoopRecorder`.
   - The disabled state overrides the endpoint; the "no network egress when disabled" guarantee is preserved.

6. NO use of `testify`. NO use of `t.Parallel()`. The `countingTransport` is local to the test file.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestNoopRecorderNoNetworkCalls -v` passes
- `go test ./internal/telemetry/... -run TestRecorderFactory -v` passes (3 sub-tests)
- `go test ./internal/telemetry/...` exits 0
- The counting transport's `RoundTrip` is the only path that would increment the counter; the test asserts the counter is 0 after 100 calls on the NoopRecorder
</verify>
<done>[ ]</done>
</task>

<task id="03-03-04">
<name>Add OBSERVABILITY.md byte-for-byte example payload test</name>
<files>
- packages/cli/internal/telemetry/observability_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/observability_test.go` (package `telemetry`). The test parses the example block in OBSERVABILITY.md and asserts the recorder's output matches.

1. Imports:
   - `bytes`
   - `encoding/json`
   - `os`
   - `path/filepath`
   - `regexp`
   - `strings`
   - `testing`
   - `time`

2. `TestOBSERVABILITYExampleMatchesEmitted(t *testing.T)`:
   - Find OBSERVABILITY.md by walking up from the test's working directory until `OBSERVABILITY.md` is found. Use the convention: `for dir := "."; dir != "/"; dir = filepath.Dir(dir) { if _, err := os.Stat(filepath.Join(dir, "OBSERVABILITY.md")); err == nil { ... } }`. The repo root has the file.
   - Read the file with `os.ReadFile`.
   - Extract the JSON code block (between ````json` and ```` `, exclusive). Use a regex: ``var exampleRe = regexp.MustCompile("(?s)```json\\n(.*?)\\n```")`` and find the first match.
   - Unmarshal the example into `map[string]any`.
   - Assert the example has exactly 7 keys.
   - For each of the 3 deterministic keys (`command`, `exit_status`, `version`), assert the example value is the expected type (string for command/version, number for exit_status) — no substitution needed.
   - For each of the 4 volatile keys (`install_id`, `host_id`, `event_id`, `timestamp`), assert the example value matches its regex shape (the doc's example values are placeholders; the assertion is "the shape is correct", not "the value is correct").
   - Build a recorder and emit a real event: `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: "https://placeholder.invalid"})` (the endpoint is never called — we discard the recorder). Call `NewRecorder().Record(ctx, event)` to get the JSON-serialised event. (The actual recorder writes to the endpoint; for this test we marshal the event directly to avoid the network call: `body, _ := json.Marshal(event)`.)
   - Parse the emitted body and the example body; for each volatile key, assert the emitted value matches the regex; for each deterministic key, assert the values are equal.
   - Assert the volatile keys' values DIFFER from the example's placeholders (the recorder generates fresh values).

3. `TestOBSERVABILITYHasAllSevenSections(t *testing.T)`:
   - Read OBSERVABILITY.md; assert it contains the 7 required `## ` section headers (case-sensitive).
   - This catches "a refactor accidentally dropped a section" regressions.

4. `TestOBSERVABILITYExampleIsValidJSON(t *testing.T)`:
   - Extract the JSON block (same regex as test 2); assert `json.Valid` returns true.
   - The example is the wire format; an invalid JSON example in the doc is a documentation bug.

5. NO use of `testify`. NO use of `t.Parallel()`. The test file's package is `telemetry` (so it can call `SetDefaultFactory` and `NewRecorder` directly).
</action>
<verify>
- `go test ./internal/telemetry/... -run TestOBSERVABILITY -v` exits 0 (3 tests pass)
- `go test ./internal/telemetry/...` exits 0
- The test file's `obsPath` resolution walks up to the repo root (works regardless of `go test ./internal/telemetry/...` vs `go test ./...`)
</verify>
<done>[ ]</done>
</task>

<task id="03-03-05">
<name>Add e2e tests: first-run prompt fires once, disabled no buffer</name>
<files>
- packages/cli/e2e_test.go
</files>
<action>
Append to `packages/cli/e2e_test.go` (the file already has the `cliEnv` test helper and the binary-building logic from plan 02's pattern).

1. `TestTelemetryFirstRunPromptFiresOnce(t *testing.T)`:
   - `env := newCLIEnv(t)` (existing helper).
   - The first run: `output := env.runInteractive(t, env.workspace, nil, []interactiveStep{{waitFor: "telemetry", send: "\r"}}, "sync")`. (The prompt is the pterm `DefaultInteractiveConfirm`; the wait-for string is "telemetry" or "Enable anonymous" — pick the most likely stable string; the test fails clearly if the prompt is not shown.)
   - Assert the output contains "telemetry" (the prompt text was displayed).
   - Read `<configHome>/skill-organizer/skill-organizer.yml`; assert it contains `enabled:` and the value is `false` (the user pressed Enter on the default of "no"). The test asserts the sticky behavior: the answer is written.
   - The second run: `output2 := env.run(t, env.workspace, nil, "sync")`. (No interactiveStep — the prompt should not fire.)
   - Assert the output2 does NOT contain "telemetry" (or the prompt text) — the sticky answer short-circuits.
   - Assert the sentinel file `<configHome>/skill-organizer/telemetry-prompted` exists.

2. `TestTelemetryDisabledNoBuffer(t *testing.T)`:
   - `env := newCLIEnv(t)`.
   - Pre-write the YAML with `telemetry: { enabled: false, endpoint: "" }` to `<configHome>/skill-organizer/skill-organizer.yml`. The `writeProjectConfig()` helper does not set this — add a small helper or inline the write.
   - Run a command: `env.run(t, env.workspace, nil, "sync")` (sync is the simplest read-only command; it may fail without a project config, but the telemetry event emit happens regardless of the command's success).
   - Assert the file `<configHome>/skill-organizer/telemetry-buffer.jsonl` does NOT exist (the noop recorder never writes).
   - As a positive control: also assert `<configHome>/skill-organizer/install_id` and `<configHome>/skill-organizer/host_id` DO exist (the Identity is loaded even when telemetry is disabled, per the factory short-circuit on Endpoint).

3. `TestTelemetryStatusSubcommandE2E(t *testing.T)`:
   - `env := newCLIEnv(t)`.
   - Pre-write the YAML with `telemetry: { enabled: true, endpoint: "https://example.invalid" }` (the endpoint is never called because no events fire during the status subcommand — the subcommand name is in the skip set).
   - Run `env.run(t, env.workspace, nil, "telemetry", "status")`.
   - Assert the output contains "Enabled:" and "true" and "https://example.invalid" and "Install ID:" and "Host ID:" (the 5 lines from the status subcommand).

4. NO use of `testify`. The tests use the existing `cliEnv` helper; they do NOT call `t.Parallel()` because they share the `cliEnv`'s `configHome` via `t.TempDir()` (which is per-test, so parallelism is safe, but the existing tests in `e2e_test.go` call `t.Parallel()` — follow the pattern).

5. The `runInteractive` helper waits for a string on stdout before sending. The first-run prompt's exact text is determined by the pterm `confirm` helper; use the most stable substring ("telemetry" is part of the prompt and of the "Telemetry enabled" success line — use the latter to disambiguate; or wait for "Enable anonymous" which is the prompt header from the docs).

6. The test for `TestTelemetryStatusSubcommandE2E` must use a YAML value that does not require network resolution (the `https://example.invalid` endpoint is never contacted because the status subcommand is in the skip set; if the test inadvertently exercises the emit path, the DNS resolution would fail and the test would be slow). The skip set is in `cmd/root.go`'s `PersistentPreRun` and `PersistentPostRun`.
</action>
<verify>
- `go test ./... -run TestTelemetry` exits 0 (3 e2e tests pass; the run takes ~10s per test because the binary is built once per test via `t.Parallel()` + shared buildBinary)
- `go test ./...` exits 0 (no regression on the other e2e tests)
- The OBSERVABILITY.md doc + the e2e tests together prove REQ-8 acceptance: the schema is documented, the opt-in flow is testable, and the disabled-no-egress state is verifiable
</verify>
<done>[ ]</done>
</task>

<task id="03-03-06">
<name>Final verification: all 3 plans green together, end-to-end demo path</name>
<files>
- (no new files; this is the integration verification step)
</files>
<action>
This task is the final green-bar check across all 3 plans. Run the following commands and confirm zero failures:

1. `go build ./...` — exits 0
2. `go vet ./...` — exits 0
3. `go test ./internal/telemetry/...` — exits 0 (all plan 01 + 02 + 03 telemetry tests)
4. `go test ./cmd/...` — exits 0 (the telemetry subcommand tests + the existing 200+ cmd tests)
5. `go test ./...` — exits 0 (no regression anywhere)
6. `go test ./... -count=1` — re-run with `-count=1` to bypass the test cache (the byte-for-byte schema test must pass on every run, not just cached runs)
7. `git status` — clean working tree except for the new files added in plans 01, 02, 03
8. `git diff --stat` — show the line counts of all modifications; assert no file in `packages/cli/internal/{overlap,security,agenttools,maintenance,skills,backup,config,service,selfupdate,sync,watch,mover,logging,status,versionfmt,remote}/` was modified by plans 02 or 03 (the only changes are in `telemetry/`, `config/`, `cmd/`, plus `go.mod`/`go.sum`)

Then verify the end-to-end demo path manually:

9. Build the binary: `cd packages/cli && go build -o /tmp/skill-organizer .`
10. Set up a fresh config home: `export XDG_CONFIG_HOME=$(mktemp -d)`.
11. Run the binary with a TTY (or with `script`/`unbuffer` if not in a real TTY) to trigger the first-run prompt: `/tmp/skill-organizer sync`. Press Enter (default = no).
12. Inspect: `cat $XDG_CONFIG_HOME/skill-organizer/skill-organizer.yml` — should show `telemetry: { enabled: false }`.
13. Inspect: `ls $XDG_CONFIG_HOME/skill-organizer/telemetry-prompted` — should exist.
14. Run again: `/tmp/skill-organizer sync` — should NOT show the prompt (sentinel).
15. Run `/tmp/skill-organizer telemetry status` — should print 5 lines (Enabled, Endpoint, Install ID, Host ID, Buffer file).
16. Run `/tmp/skill-organizer telemetry enable` — should print the success line.
17. Inspect YAML: `cat $XDG_CONFIG_HOME/skill-organizer/skill-organizer.yml` — should show `telemetry: { enabled: true }`.
18. Run `/tmp/skill-organizer telemetry status` — should show `Enabled: true`.

If any step fails, the prior plan's tests should be re-run; do not skip the failure. The demo path is the user-facing acceptance for REQ-8.
</action>
<verify>
- All 8 build/test commands exit 0
- `git diff --stat` shows changes only in `packages/cli/cmd/{telemetry.go, telemetry_test.go, root.go, root_test.go}`, `packages/cli/internal/telemetry/`, `packages/cli/internal/config/{config.go, registry.go}`, `packages/cli/go.mod`, `packages/cli/go.sum`, and the new `OBSERVABILITY.md`
- The 10-step manual demo path completes without error
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `OBSERVABILITY.md` exists at the repo root with 7 sections
- [ ] `OBSERVABILITY.md` is between 140 and 200 lines
- [ ] The example payload in the Schema section contains exactly 7 top-level keys
- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/telemetry/...` passes
- [ ] `go test ./cmd/...` passes
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `TestHTTPRecorderSchemaByteForByte` passes
- [ ] `TestHTTPRecorderSchemaFieldOrder` passes
- [ ] `TestHTTPRecorderFieldCount` passes
- [ ] `TestNoopRecorderNoNetworkCalls` passes (counting transport, 0 calls)
- [ ] `TestRecorderFactoryReturnsNoopWhenEndpointEmpty` passes
- [ ] `TestRecorderFactoryReturnsHTTPRecorderWhenConfigured` passes
- [ ] `TestRecorderFactoryReturnsNoopWhenDisabled` passes
- [ ] `TestOBSERVABILITYExampleMatchesEmitted` passes
- [ ] `TestOBSERVABILITYHasAllSevenSections` passes
- [ ] `TestOBSERVABILITYExampleIsValidJSON` passes
- [ ] `TestTelemetryFirstRunPromptFiresOnce` passes in e2e_test.go
- [ ] `TestTelemetryDisabledNoBuffer` passes in e2e_test.go
- [ ] `TestTelemetryStatusSubcommandE2E` passes in e2e_test.go
- [ ] All plan 01 and plan 02 must-haves still pass (no regression)
- [ ] The end-to-end manual demo path (build -> run -> prompt -> status -> enable) completes

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/internal/telemetry/recorder_test.go packages/cli/e2e_test.go`
2. Remove the new doc: `rm -f OBSERVABILITY.md`
3. Remove the new test file: `rm -f packages/cli/internal/telemetry/observability_test.go`
4. Verify: `go build ./...` and `go test ./...` pass on the reverted state (the tests in plans 01 and 02 are still green; the new tests are gone).
5. Retry with smaller scope:
   - First, ship `OBSERVABILITY.md` only (task 03-01).
   - Then add the byte-for-byte + counting-transport tests (task 03-02 + 03-03).
   - Then add the OBSERVABILITY example payload test (task 03-04).
   - Then add the e2e tests (task 03-05).
   - Then run the final verification (task 03-06).

The OBSERVABILITY.md doc is independent of the Go code: it can ship first without breaking the build. The test that parses the doc depends on the doc's exact wording, so a doc fix requires a corresponding test fix in the same commit.

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | The OBSERVABILITY.md example payload drifts from the recorder's output (e.g., a future PR changes the field order in the Event struct, the doc is not updated). | Medium | High | `TestOBSERVABILITYExampleMatchesEmitted` parses the doc and asserts the recorder's output matches. The test fails on any drift, catching the regression at CI time. |
| 2 | The counting transport's atomic counter is not reset between tests, leaking state into a later test that expects 0 calls. | Low | Medium | Each test creates a fresh `countingTransport{}` value; the counter is per-instance. The atomic load is the only place the value is read. The `t.Cleanup` pattern in plan 02's tests is the precedent. |
| 3 | The e2e test's first-run prompt never appears in CI (the pterm `confirm` requires a real TTY), and the test silently passes because the wait-for string times out. | High | Medium | The `runInteractive` helper has a `waitFor` that times out after `testTimeout` (90s). If the prompt never appears, the test fails with a timeout error, not a silent pass. The "second run" assertion in `TestTelemetryFirstRunPromptFiresOnce` is the negative control: if the first run somehow did not show the prompt, the second run would also not show it, and the test would fail on the second assertion. |
| 4 | The httptest server is closed before the recorder's request is fully sent, causing a "use of closed connection" error. | Low | Low | The test does NOT explicitly close the server; `httptest.NewServer` returns a `*Server` whose lifecycle is tied to the test's `t.Cleanup` (via `t.Cleanup(srv.Close)`). The recorder's `http.Client.Do` returns the response before the server is closed. |
| 5 | The doc's `wc -l` count drifts above 200 (e.g., a future PR adds verbose prose), violating the "short and direct" style. | Low | Low | The 200-line cap is a guideline, not a hard limit. The `wc -l` verification in task 03-01 catches egregious drift (>200 lines); small additions (a few lines) are acceptable. |
| 6 | A future PR removes the `## FAQ` section from OBSERVABILITY.md (or renames it), and the OBSERVABILITY example payload test still passes (the test does not check section headers), so the regression slips through. | Low | Medium | `TestOBSERVABILITYHasAllSevenSections` explicitly asserts the 7 section headers are present. The test runs in the same suite as the example test; both must pass. |
| 7 | The e2e tests build the binary once per test, taking ~10s per test. If the suite already has 30 e2e tests, adding 3 more adds ~30s of CI time. | Low | Low | The existing `cliEnv` calls `buildBinary(t, root)` (lines 315-332 of e2e_test.go) per test. The cost is bounded; no caching is implemented in the existing suite, so this plan follows the same pattern. |
| 8 | The e2e test's `telemetry status` subcommand prints to pterm's default stdout, which in the test environment may be captured but not asserted. | Low | Low | The `env.run` helper returns the captured output as a string. The test asserts the string contains the expected substrings. The pterm color codes are stripped by the test runner (the existing e2e tests follow this pattern; see `assertContains` in `e2e_test.go`). |

## Commit Message

```
feat(cli): ship OBSERVABILITY.md, byte-for-byte schema test, e2e tests

- Add OBSERVABILITY.md at the repo root with 7 sections:
  What is collected, Schema, How to enable / disable, Endpoint
  configuration, Data retention, Privacy guarantees, FAQ
- Add HTTPRecorder byte-for-byte schema test (httptest.NewServer,
  captures raw POST body, asserts 7 top-level keys, asserts the
  3 deterministic fields byte-for-byte and the 4 volatile fields
  via regex)
- Add counting-transport zero-egress test for NoopRecorder and
  the factory short-circuits (endpoint empty / disabled)
- Add OBSERVABILITY example payload test (parses the doc's JSON
  block, asserts the recorder's output matches)
- Add 3 e2e tests in e2e_test.go: first-run prompt fires only
  on the first run, disabled state never writes the buffer,
  telemetry status prints the 5 expected lines
- All plan 01 and plan 02 must-haves still pass
- Closes REQ-8 acceptance: default install emits no telemetry,
  first-run opt-in flow is documented and works, schema doc
  matches the emitted payload byte-for-byte
```
