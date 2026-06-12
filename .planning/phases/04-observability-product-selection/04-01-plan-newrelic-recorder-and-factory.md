---
wave: 1
depends_on: []
files_modified:
  - packages/cli/internal/telemetry/recorder.go
  - packages/cli/internal/telemetry/recorder_test.go
  - packages/cli/cmd/root.go
  - packages/cli/cmd/telemetry.go
  - packages/cli/cmd/telemetry_test.go
  - .planning/phases/04-observability-product-selection/04-01-plan-SUMMARY.md
autonomous: true
single_layer_justified: false
requirement: REQ-9
objective: "Add a NewRelicRecorder struct (per the Phase 4 CONTEXT) that POSTs a New-Relic-shaped array envelope ([{eventType, command, exit_status, install_id, host_id, clientTime, version, event_id}]) with the X-Insert-Key auth header, hard-drops events on 413/429 (return nil, no buffer write), 1-retries 503s with a 250ms context-aware timer (time.NewTimer + select on ctx.Done()), and sets User-Agent to skill-organizer/<version>. Extend the existing RecorderConfig with AccountID and InsertKey fields; the default factory closure picks NewRelicRecorder when both env vars are set, then HTTPRecorder (if endpoint is set), then NoopRecorder. Wire the two new env vars (SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID, SKILL_ORGANIZER_NEWRELIC_INSERT_KEY) in cmd/root.go. Extend telemetry status to print Recorder type, Account ID prefix, and Insert key presence. Add an httptest.NewServer smoke test that asserts POST URL path, X-Insert-Key header, body is array of length 1, eventType field, and the 7 schema fields match (with timestamp renamed to clientTime in the envelope). Verifiable by go test ./internal/telemetry/... and go test ./cmd/... passing."
must_haves:
  - "go build ./... succeeds"
  - "go test ./internal/telemetry/... passes (all existing + new tests green)"
  - "go test ./cmd/... passes (all existing + new status-output tests green)"
  - "TestNewRelicRecorderContractEnforced passes (httptest.NewServer captures r.URL.Path == /v1/accounts/{accountID}/events, X-Insert-Key header matches, body unmarshals to []map[string]any of length 1, arr[0][eventType] == skill_organizer_command, the 7 schema fields match the recorder's input modulo 4 volatile fields with timestamp renamed to clientTime in the envelope)"
  - "TestNewRelicRecorderHardDropsOn413 passes (server returns 413, Record returns nil, the on-disk buffer is NOT written)"
  - "TestNewRelicRecorderHardDropsOn429 passes (server returns 429, Record returns nil, the on-disk buffer is NOT written)"
  - "TestNewRelicRecorderRetriesOn503 passes (server returns 503 then 200, Record returns nil, server hit count == 2)"
  - "TestNewRelicRecorderHonorsContextCancellation passes (server returns 503, ctx is cancelled before the backoff completes, Record returns ctx.Err() and does not retry)"
  - "TestNewRelicRecorderUserAgentHeader passes (User-Agent: skill-organizer/<version> is set on the POST)"
  - "TestRecorderFactoryPicksNewRelicWhenEnvVarsSet passes (both env vars set, factory returns NewRelicRecorder with the right AccountID/InsertKey/Endpoint)"
  - "TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete passes (only one of the two env vars set + endpoint set, factory returns HTTPRecorder)"
  - "TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete passes (only one of the two env vars set + endpoint empty, factory returns NoopRecorder)"
  - "TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured passes (no env vars, endpoint set, factory returns HTTPRecorder — Phase 3 behavior preserved)"
  - "TestTelemetryStatusSubcommand shows Recorder type, Account ID prefix, and Insert key presence lines"
  - "No changes to the Event struct or the byte-for-byte schema test (HTTPRecorder still emits the flat 7-field object)"
  - "No new packages or dependencies; everything lives in internal/telemetry/ and cmd/"
  - "RecorderConfig is backwards-compatible (existing callers that set Enabled + Endpoint continue to work, the NewRelic branch fires only when AccountID AND InsertKey are set)"
---

# Plan 04-01: NewRelicRecorder, factory extension, status output, smoke test

## Objective

Ship the `NewRelicRecorder` struct and the factory wiring that picks it
when the user has configured the New Relic backend (via
`SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` and `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY`).
The recorder wraps the project's flat 7-field `Event` in a backend-specific
envelope (a JSON array with an `eventType` prefix), sets the `X-Insert-Key`
auth header, hard-drops events on 413/429 (no buffer fallback), and
1-retries 503s with a context-aware 250ms backoff. The factory closure
becomes a 3-way switch: New Relic env vars set → `NewRelicRecorder` →
else `HTTPRecorder` (if endpoint) → else `NoopRecorder`. The
`telemetry status` command is extended to print the resolved recorder
type, the account ID prefix, and the insert-key presence. The smoke
test uses `httptest.NewServer` and asserts the 5 properties from the
CONTEXT spec (URL path, header, body is array of length 1, `eventType`,
and the 7 schema fields with `timestamp` → `clientTime` in the envelope).

## Context

Phase 3 shipped the `HTTPRecorder` (a product-agnostic passthrough that
POSTs the flat 7-field object), the `RecorderFactoryFunc` swappable
closure, the byte-for-byte schema test, and the `telemetry status` /
`enable` / `disable` / `rotate-host-id` subcommands. Phase 4 picks the
**receive side** — a real, free-of-charge backend that accepts the
events the CLI emits. The CONTEXT locks New Relic Insights Events API
as the chosen product; the user-facing flow is:

1. User signs up for New Relic (free tier).
2. User creates an Insights insert key.
3. User exports `SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID` and
   `SKILL_ORGANIZER_NEWRELIC_INSERT_KEY`.
4. User runs `skill-organizer telemetry enable`.
5. Events leave the buffer and arrive at
   `https://insights-collector.newrelic.com/v1/accounts/{account_id}/events`.

Per RESEARCH, the New Relic Insights Events API expects a JSON **array**
of events, each prefixed with a string `eventType` field. The project's
flat 7-field schema is the canonical contract (unchanged); the array +
`eventType` prefix is a **backend-specific envelope** that the
`NewRelicRecorder` adds. The HTTPRecorder (Phase 3) is retained as a
passthrough for power users who want to point at a custom proxy.

Two pitfalls drive the design:

- **NP1 (timestamp reserved by New Relic):** New Relic reserves the
  `timestamp` attribute for Unix-epoch integers. An RFC3339 string
  sent in the `timestamp` field is silently dropped at ingest. The
  recorder must rename the field to `clientTime` in the **envelope
  only** — the flat 7-field object and the byte-for-byte schema test
  are unchanged.
- **NP4 (no buffer fallback on 413/429):** Phase 3's
  `Service.RecordEvent` writes to the buffer on recorder failure. If
  the NewRelicRecorder returns a non-nil error for 413/429, the
  Service appends the event to the buffer, the next drain re-POSTs
  it, and the server returns 413/429 again — an infinite loop. The
  recorder must return **nil** on 413/429 (the event is dropped, a
  one-line warning is logged via pterm), distinct from network errors
  and other 4xx/5xx which still return errors and trigger the buffer
  fallback.

The 503 retry uses `time.NewTimer(250*ms)` + `select` on
`ctx.Done()` (Pitfall NP3) so cancellation is honored mid-backoff.
The HTTP client is built via the existing `NewHTTPClientFunc` package
var (Phase 3 `recorder.go:52-54`) so the counting-transport zero-egress
test pattern extends unchanged.

The `RecorderConfig` struct gains two fields (`AccountID` and
`InsertKey`); existing callers (`cmd/root.go:106` passes
`TelemetryConfig{Enabled, Endpoint}`) continue to work because the new
fields are zero-valued unless the env vars are set, and the
`SetDefaultFactory` factory closure only enters the NewRelic branch
when both are non-empty.

## Tasks

<task id="04-01-01">
<name>Extend RecorderConfig and SetDefaultFactory to pick NewRelicRecorder</name>
<files>
- packages/cli/internal/telemetry/recorder.go
</files>
<action>
Modify `packages/cli/internal/telemetry/recorder.go`:

1. Extend the `RecorderConfig` struct (lines 99-105) with two new fields:
   ```go
   // RecorderConfig is the input to the default factory. The cmd
   // package's PersistentPreRun sets this from the resolved
   // TelemetryConfig (Phase 3) plus the two New Relic env vars
   // (Phase 4). AccountID and InsertKey are env-only (no YAML) per
   // the CONTEXT: secrets don't belong in the user's YAML file.
   type RecorderConfig struct {
       Enabled   bool
       Endpoint  string
       AccountID string // SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID
       InsertKey string // SKILL_ORGANIZER_NEWRELIC_INSERT_KEY
   }
   ```

2. Update `SetDefaultFactory` (lines 107-121) to a 3-way switch. The
   factory closure now returns `NewRelicRecorder` when both `AccountID`
   and `InsertKey` are non-empty (and `Enabled` is true); else
   `HTTPRecorder` (if `Enabled && Endpoint != ""`); else `NoopRecorder`.
   The `NoopRecorder` short-circuit (Pitfall NP9-style — "disabled or
   unconfigured must produce zero egress") is preserved:
   ```go
   func SetDefaultFactory(cfg RecorderConfig) {
       RecorderFactoryFunc = func() Recorder {
           if !cfg.Enabled {
               return NoopRecorder{}
           }
           if cfg.AccountID != "" && cfg.InsertKey != "" {
               return NewNewRelicRecorder(cfg.AccountID, cfg.InsertKey, cfg.Endpoint)
           }
           if cfg.Endpoint != "" {
               return NewHTTPRecorder(cfg.Endpoint)
           }
           return NoopRecorder{}
       }
   }
   ```

3. Add a package-level `RecorderVersion` var (placeholder for the
   `User-Agent` header). Declare it next to the existing
   `RecorderFactoryFunc` and `NewHTTPClientFunc` vars:
   ```go
   // RecorderVersion is set by the cmd package (cmd/root.go) at
   // PersistentPreRun time. The default empty string means the
   // User-Agent header is omitted; production sets it to the CLI
   // semver. Exported so cmd/root.go can write to it.
   var RecorderVersion = ""
   ```

4. Add the `NewNewRelicRecorder` constructor (placeholder body — the
   full struct is added in task 04-01-02). The signature is:
   ```go
   // NewNewRelicRecorder returns a Recorder that POSTs a
   // New-Relic-shaped array envelope to the New Relic Insights
   // Events API. The endpointTemplate is the URL template with the
   // account_id placeholder; the constructor substitutes the
   // AccountID at recorder-construction time. The InsertKey is
   // sent in the X-Insert-Key header per the CONTEXT decision.
   func NewNewRelicRecorder(accountID, insertKey, endpointTemplate string) Recorder {
       endpoint := newRelicEndpointTemplate
       if endpointTemplate != "" {
           endpoint = endpointTemplate
       }
       endpoint = strings.ReplaceAll(endpoint, "$ACCOUNT_ID", accountID)
       return &NewRelicRecorder{
           Endpoint:   endpoint,
           InsertKey:  insertKey,
           HTTPClient: NewHTTPClientFunc(),
           Version:    RecorderVersion,
       }
   }
   ```
   Add `"strings"` to the import list. Use `strings.ReplaceAll` (not
   `strings.Replace` with a count) because the template contains
   exactly one placeholder; either works.

5. Add a `newRelicEndpointTemplate` package-level constant near the
   other vars:
   ```go
   // newRelicEndpointTemplate is the default endpoint for the New
   // Relic Insights Events API. The $ACCOUNT_ID placeholder is
   // substituted with cfg.AccountID in the constructor. Per
   // CONTEXT, the EU data center variant
   // (insights-collector.eu01.nr-data.net) is documented in
   // OBSERVABILITY.md but not defaulted here — users in the EU
   // set telemetry.endpoint to override.
   const newRelicEndpointTemplate = "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events"
   ```

6. Add a brief package-level doc comment for the new recorder
   surface, just above the `NewNewRelicRecorder` declaration, in
   the same style as the existing `HTTPRecorder` block (lines 56-64).
   The comment notes: (a) this is the New-Relic-specific backend; (b)
   the envelope wraps the flat 7-field schema; (c) `timestamp` is
   renamed to `clientTime` in the envelope (NP1 workaround); (d)
   413/429 are hard drops; (e) 503 gets one context-aware retry.

7. Do NOT change the `Recorder` interface, `NoopRecorder`,
   `HTTPRecorder`, or `NewHTTPClientFunc` — those are stable from
   Phase 3.
</action>
<verify>
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/telemetry/... -count=1` exits 0 (no regression
  on the existing tests: noop drops events, factory returns noop on
  empty config, factory swap roundtrip, HTTPRecorder byte-for-byte /
  field order / field count, noop zero-egress counting transport,
  factory short-circuits, etc.)
- The diff for `recorder.go` adds: (a) the two new RecorderConfig
  fields, (b) the rewritten SetDefaultFactory, (c) the new
  RecorderVersion var, (d) the newNewRelicRecorder constructor
  (placeholder, struct added in task 02), (e) the
  newRelicEndpointTemplate constant, (f) `"strings"` in the import
  list. No deletions, no changes to the existing
  `HTTPRecorder` / `NoopRecorder` / `Recorder` interface code.
</verify>
<done>[ ]</done>
</task>

<task id="04-01-02">
<name>Add NewRelicRecorder struct, Record method, and the NewRelic-shaped envelope</name>
<files>
- packages/cli/internal/telemetry/recorder.go
</files>
<action>
Append to `packages/cli/internal/telemetry/recorder.go` (after the
`NewNewRelicRecorder` constructor from task 04-01-01):

1. Add the `NewRelicRecorder` struct:
   ```go
   // NewRelicRecorder POSTs events to the New Relic Insights Events
   // API. The envelope is a JSON array of length 1 (one event per
   // POST — the buffer drain calls Record once per event, not in
   // batches). The envelope adds:
   //
   //   - "eventType": "skill_organizer_command" (New Relic requires
   //     this field; it groups events in the NRDB UI).
   //   - "clientTime": the RFC3339 string from event.Timestamp.
   //     New Relic RESERVES the "timestamp" attribute name for
   //     Unix-epoch integers (RESEARCH NP1); an RFC3339 string sent
   //     in the "timestamp" field is silently dropped at ingest. The
   //     rename is an envelope-only transform — the flat 7-field
   //     schema in event.go and the byte-for-byte HTTPRecorder test
   //     are unchanged.
   //
   // Status code handling:
   //   - 2xx: return nil. Service moves on.
   //   - 413, 429: log a one-line warning via pterm and return nil.
   //     The event is DROPPED (no buffer write). Per CONTEXT, the
   //     local buffer is for network-down, not server-quota.
   //     Returning a non-nil error here would trigger Service's
   //     "recorder failed -> buffer write" path, creating an
   //     infinite drain loop (RESEARCH NP4).
   //   - 503: 1 retry with 250ms context-aware backoff. Final 503
   //     returns the error (so the event is buffered for the next
   //     drain).
   //   - Other 4xx, 5xx, network errors: return the error. The
   //     buffer is the right fallback for transient failures.
   //
   // The X-Insert-Key header is the New Relic Insights Event API
   // auth method (per CONTEXT lock). The User-Agent is
   // "skill-organizer/<version>" for ops visibility on the New
   // Relic side.
   type NewRelicRecorder struct {
       Endpoint   string // resolved URL (account_id substituted)
       InsertKey  string // X-Insert-Key header value
       HTTPClient *http.Client
       Version    string // for User-Agent; may be empty
   }
   ```

2. Implement the `Record` method. Use `[]map[string]any{...}` for
   the envelope so `encoding/json` marshals the inner object with
   key-sorted order. Use snake_case for the project's schema fields
   and camelCase for the New Relic-specific keys (`eventType` is a
   New Relic requirement; `clientTime` is the rename to dodge the
   reserved `timestamp`):
   ```go
   func (r NewRelicRecorder) Record(ctx context.Context, event Event) error {
       elem := map[string]any{
           "eventType":  "skill_organizer_command",
           "command":    event.Command,
           "exit_status": event.ExitStatus,
           "install_id": event.InstallID,
           "host_id":    event.HostID,
           "clientTime": event.Timestamp, // renamed: see struct doc
           "version":    event.Version,
           "event_id":   event.EventID,
       }
       body, err := json.Marshal([]map[string]any{elem})
       if err != nil {
           return fmt.Errorf("marshal newrelic envelope: %w", err)
       }

       send := func() (int, error) {
           req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
           if err != nil {
               return 0, fmt.Errorf("build newrelic request: %w", err)
           }
           req.Header.Set("Content-Type", "application/json")
           req.Header.Set("X-Insert-Key", r.InsertKey)
           if r.Version != "" {
               req.Header.Set("User-Agent", "skill-organizer/"+r.Version)
           }
           resp, err := r.HTTPClient.Do(req)
           if err != nil {
               return 0, fmt.Errorf("post event to newrelic: %w", err)
           }
           defer resp.Body.Close()
           return resp.StatusCode, nil
       }

       status, err := send()
       if err != nil {
           return err
       }
       // 413/429: hard drop, return nil, log a warning.
       if status == http.StatusRequestEntityTooLarge || status == http.StatusTooManyRequests {
           WarningFunc("telemetry: dropping event due to %d from New Relic (quota or rate-limit; will not retry)", status)
           return nil
       }
       // 503: 1 retry with 250ms context-aware backoff.
       if status == http.StatusServiceUnavailable {
           select {
           case <-time.After(250 * time.Millisecond):
               // fall through to the retry
           case <-ctx.Done():
               return ctx.Err()
           }
           status, err = send()
           if err != nil {
               return err
           }
           if status == http.StatusRequestEntityTooLarge || status == http.StatusTooManyRequests {
               WarningFunc("telemetry: dropping event due to %d from New Relic (quota or rate-limit after retry)", status)
               return nil
           }
       }
       if status < 200 || status >= 300 {
           return fmt.Errorf("post event to newrelic: unexpected status %d", status)
       }
       return nil
   }
   ```

3. Add a package-level `WarningFunc` var near the other
   `NewHTTPClientFunc` / `RecorderFactoryFunc` vars. This is the
   test seam for the hard-drop warning:
   ```go
   // WarningFunc is a swappable function variable for emitting
   // warnings. The default writes to stderr via pterm.Warning
   // (light-magenta, per the project's color rules — yellow is
   // reserved for keyboard hints). Tests reassign in t.Cleanup
   // to capture or silence the output.
   var WarningFunc = func(format string, args ...any) {
       pterm.Warning.Printfln(format, args...)
   }
   ```
   Add `"github.com/pterm/pterm"` to the import list. pterm is
   already a direct dep in the CLI (it's used in cmd/telemetry.go
   for the status output), so no `go get` is needed.

4. No changes to the existing `Event` struct, `HTTPRecorder`, or
   the byte-for-byte schema test.
</action>
<verify>
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./internal/telemetry/... -count=1` exits 0 (no regression)
- The inner envelope map's keys are exactly: `eventType`,
  `command`, `exit_status`, `install_id`, `host_id`, `clientTime`,
  `version`, `event_id` (count: 8 keys total — 1 is the `eventType`
  prefix, 6 are the project's snake_case schema fields, 1 is the
  `clientTime` rename)
- The `Record` method returns `nil` on 413/429 and on 2xx; returns
  the error on other 4xx/5xx and network errors
- The 503 retry uses `time.After(250 * time.Millisecond)` +
  `select` on `ctx.Done()` (NP3 pattern)
- The User-Agent header is set to `skill-organizer/<version>` when
  `r.Version` is non-empty; omitted when empty
</verify>
<done>[ ]</done>
</task>

<task id="04-01-03">
<name>Wire New Relic env vars and version in cmd/root.go</name>
<files>
- packages/cli/cmd/root.go
</files>
<action>
Modify `packages/cli/cmd/root.go`:

1. In the `init()` function (lines 63-170), extend the telemetry
   block in `PersistentPreRun` (lines 92-114) to read the two New
   Relic env vars and pass them to `SetDefaultFactory` (now with the
   extended signature from task 04-01-01). The two new reads go
   **after** the existing endpoint resolution and **before** the
   `SetDefaultFactory` call (note: the existing root.go does NOT
   call `SetDefaultFactory` directly — the factory is set inside
   `Service.New` via `NewRecorder()` → `RecorderFactoryFunc()` at
   construction time, per BUG #2 from STATE.md). The two env vars
   are written to package-level vars that the factory closure reads:

   ```go
   // ---- Telemetry (REQ-8 / REQ-9) ----
   // Resolve the AppDir and the final endpoint value (flag > env > YAML).
   appDir, appDirErr := configpkg.AppDir()
   if appDirErr == nil {
       registryPath, _ := configpkg.RegistryPath()
       cfg, _ := configpkg.LoadTelemetryConfigOrDefault(registryPath)
       resolvedEndpoint := telemetrypkg.ResolveEndpoint(
           telemetryEndpoint,
           os.Getenv("SKILL_ORGANIZER_TELEMETRY_ENDPOINT"),
           cfg.Endpoint,
       )
       // Phase 4: read the two New Relic env vars and the version.
       // The RecorderVersion var is read by NewNewRelicRecorder
       // (User-Agent header).
       telemetrypkg.RecorderVersion = version
       newRelicAccountID := os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID")
       newRelicInsertKey := os.Getenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY")
       // Set the default factory with the new fields. The factory
       // picks NewRelicRecorder when both AccountID and InsertKey
       // are set, else HTTPRecorder (if endpoint), else NoopRecorder.
       telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{
           Enabled:   cfg.Enabled,
           Endpoint:  resolvedEndpoint,
           AccountID: newRelicAccountID,
           InsertKey: newRelicInsertKey,
       })
       // The Service is constructed and stored on the command's
       // Context so the PersistentPostRun can pick it up. We use
       // a custom context-key type to avoid collisions.
       svc, svcErr := telemetrypkg.New(appDir, version, telemetrypkg.TelemetryConfig{Enabled: cfg.Enabled, Endpoint: resolvedEndpoint})
       if svcErr == nil {
           svc.MaybeRunFirstRunPrompt(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), func(yes bool) error {
               return configpkg.SaveTelemetryConfig(registryPath, telemetrypkg.TelemetryConfig{Enabled: yes, Endpoint: resolvedEndpoint})
           })
           _ = svc.DrainBuffer(cmd.Context())
           cmd.SetContext(withTelemetryService(cmd.Context(), svc))
       }
   }
   ```

2. Also update the `TelemetryConfig` value passed to
   `telemetrypkg.New(...)` (line 106) to include the new
   `NewRelic`-related fields IF you choose to add them to
   `configpkg.TelemetryConfig`. RECOMMENDED: do NOT modify
   `configpkg.TelemetryConfig` (the YAML persistence layer) — the
   env vars are secrets and per CONTEXT should not be in the YAML
   file. The factory closure reads them via the package-level vars
   (RecorderVersion) and via the RecorderConfig fields (AccountID,
   InsertKey) which are set in `SetDefaultFactory`. The
   `TelemetryConfig.Enabled` and `TelemetryConfig.Endpoint` fields
   are sufficient for the Service struct.

3. The existing 3-line `init()` (lines 92-114) becomes ~6 lines
   longer. The order matters: `SetDefaultFactory` must run BEFORE
   `telemetrypkg.New(...)` because `New` calls `NewRecorder()` →
   `RecorderFactoryFunc()` at construction time (BUG #2 fix from
   STATE.md). The placement above preserves that order.

4. No changes to the `RecorderFactoryFunc` package var directly
   (the cmd package always goes through `SetDefaultFactory`, never
   the raw var).
</action>
<verify>
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./cmd/... -count=1` exits 0 (no regression on the
  existing root_test.go: PreRun skips telemetry, PostRun emits
  event, PreRun fires first-run prompt)
- `go test ./internal/telemetry/... -count=1` exits 0 (no
  regression on the existing tests)
- `git diff packages/cli/cmd/root.go` shows the 2 new
  `os.Getenv` reads, the `RecorderVersion = version` assignment,
  and the extended `SetDefaultFactory` call with the 2 new
  fields. No other files in `cmd/` are modified.
- `SetDefaultFactory` is called BEFORE `telemetrypkg.New(...)`
  (the order is the BUG #2 fix; verify by reading the diff)
</verify>
<done>[ ]</done>
</task>

<task id="04-01-04">
<name>Extend telemetry status output with recorder type, account ID, and insert key presence</name>
<files>
- packages/cli/cmd/telemetry.go
- packages/cli/cmd/telemetry_test.go
</files>
<action>
Modify `packages/cli/cmd/telemetry.go`:

1. Add 3 new package-level func-vars for test injection (next to
   the existing `telemetryInfo` / `telemetrySuccess` / `telemetryLoadConfig`
   etc. vars at lines 18-28). These let tests stub the New Relic env
   reads without leaking into other tests:
   ```go
   // Phase 4: read the New Relic env vars for the status output.
   // Package-level func-vars for test injection.
   var (
       telemetryNewRelicAccountID = func() string { return os.Getenv("SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID") }
       telemetryNewRelicInsertKey  = func() string { return os.Getenv("SKILL_ORGANIZER_NEWRELIC_INSERT_KEY") }
   )
   ```

2. Modify `newTelemetryStatusCommand` (lines 107-135) to:
   - Compute the resolved recorder type by calling
     `RecorderFactoryFunc()` and `fmt.Sprintf("%T", rec)`. The
     type strings are: `*telemetry.NewRelicRecorder` (no, wait —
     we constructed it as `&NewRelicRecorder{...}`, so the type is
     `*telemetry.NewRelicRecorder`). Use a `typeToString` helper
     to map the 3 known types to clean names: `NewRelicRecorder`,
     `HTTPRecorder`, `NoopRecorder`.
   - Truncate the account_id to the first 4 chars (or `<unset>`
     when empty).
   - Print the insert-key presence as `present` or `<not set>`.
   The new lines go AFTER the existing 5 lines (don't replace
   the `Endpoint:` line — both are useful). Use `telemetryInfo`
   (cyan via pterm) for the new lines, matching the existing
   style.

   The updated RunE body:
   ```go
   RunE: func(cmd *cobra.Command, _ []string) error {
       registryPath, err := configpkg.RegistryPath()
       if err != nil {
           return err
       }
       cfg, err := telemetryLoadConfig(registryPath)
       if err != nil {
           return err
       }
       appDir, _ := telemetryAppDir()
       identity, _ := telemetryIdentity(appDir)
       bufferPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
       var bufferBytes int64
       if info, statErr := os.Stat(bufferPath); statErr == nil {
           bufferBytes = info.Size()
       }
       // Phase 4: resolve the recorder type via the factory closure.
       // The factory was set by root.go's PersistentPreRun (when
       // running a real command). For the `telemetry status`
       // subcommand, we set the factory here using the same inputs
       // root.go would have set, so the resolved type matches what
       // a real command invocation would pick.
       newRelicAccountID := telemetryNewRelicAccountID()
       newRelicInsertKey := telemetryNewRelicInsertKey()
       telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{
           Enabled:   cfg.Enabled,
           Endpoint:  cfg.Endpoint,
           AccountID: newRelicAccountID,
           InsertKey: newRelicInsertKey,
       })
       recType := recorderTypeName(telemetrypkg.NewRecorder())
       telemetryInfo("Enabled:      %v", cfg.Enabled)
       telemetryInfo("Endpoint:     %s", emptyAsNone(cfg.Endpoint))
       telemetryInfo("Recorder:     %s", recType)
       telemetryInfo("Account ID:   %s", shortAccountID(newRelicAccountID))
       telemetryInfo("Insert key:   %s", keyPresence(newRelicInsertKey))
       telemetryInfo("Install ID:   %s", shortID(identity.InstallID))
       telemetryInfo("Host ID:      %s", shortID(identity.HostID))
       telemetryInfo("Buffer file:  %s (%d bytes)", bufferPath, bufferBytes)
       return nil
   },
   ```

3. Add 3 small helpers at the end of `telemetry.go` (after
   `shortID`):
   ```go
   // recorderTypeName maps the 3 known Recorder implementations to
   // a short, human-friendly name for the status output. Unknown
   // types fall back to "%T" (the standard Go type-string) so a
   // future custom recorder is still observable.
   func recorderTypeName(r telemetrypkg.Recorder) string {
       switch r.(type) {
       case *telemetrypkg.NewRelicRecorder:
           return "NewRelicRecorder"
       case telemetrypkg.HTTPRecorder:
           return "HTTPRecorder"
       case telemetrypkg.NoopRecorder:
           return "NoopRecorder"
       default:
           return fmt.Sprintf("%T", r)
       }
   }

   // shortAccountID returns the first 4 chars of the account ID
   // plus "..." for the status output, or "<unset>" if empty.
   // The full account_id is a New Relic account number (a positive
   // integer); the 4-char prefix is enough to disambiguate users in
   // the status output.
   func shortAccountID(id string) string {
       if id == "" {
           return "<unset>"
       }
       if len(id) <= 4 {
           return id
       }
       return id[:4] + "..."
   }

   // keyPresence returns "present" if the key is non-empty,
   // "<not set>" otherwise.
   func keyPresence(key string) string {
       if key == "" {
           return "<not set>"
       }
       return "present"
   }
   ```

4. Update `packages/cli/cmd/telemetry_test.go` to:
   - Stub the new `telemetryNewRelicAccountID` and
     `telemetryNewRelicInsertKey` func vars in
     `TestTelemetryStatusSubcommand`.
   - Add new substrings to the `wantSubstrings` list: `Recorder:`,
     `Account ID:`, `Insert key:`.
   - Add a new sub-test `TestTelemetryStatusSubcommand_NewRelicConfigured`
     that sets both env stubs to non-empty values, asserts the
     status output contains `Recorder: NewRelicRecorder`,
     `Account ID: 1234...`, and `Insert key: present`.
   - Add a new sub-test `TestTelemetryStatusSubcommand_NewRelicIncomplete`
     that sets only the AccountID (not the InsertKey), asserts the
     status output contains `Recorder: HTTPRecorder` (fallback)
     and `Insert key: <not set>`.

5. The `telemetry status` command's `RunE` now also calls
   `telemetrypkg.SetDefaultFactory(...)` to mirror what root.go
   does. This is harmless (the factory is a package-level var;
   swapping it is idempotent for the same inputs) and ensures the
   `telemetry status` subcommand's output matches what a real
   command invocation would see, even when status is the only
   subcommand the user has run.
</action>
<verify>
- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test ./cmd/... -count=1` exits 0 (the 3 updated/new
  telemetry_test.go tests pass; the existing
  `TestTelemetryEnableSubcommand`,
  `TestTelemetryDisableSubcommand`,
  `TestTelemetryStatusSubcommand`,
  `TestTelemetryRotateHostIDSubcommand` continue to pass)
- `go test ./internal/telemetry/... -count=1` exits 0
- The new status output contains exactly 8 lines in this order:
  Enabled, Endpoint, Recorder, Account ID, Insert key, Install ID,
  Host ID, Buffer file
- `recorderTypeName` returns `NewRelicRecorder` for
  `*telemetrypkg.NewRelicRecorder`, `HTTPRecorder` for
  `telemetrypkg.HTTPRecorder` (value, not pointer), and
  `NoopRecorder` for `telemetrypkg.NoopRecorder` (value)
</verify>
<done>[ ]</done>
</task>

<task id="04-01-05">
<name>Add httptest.NewServer smoke test for NewRelicRecorder (5 CONTEXT assertions + hard-drop + retry + factory + status)</name>
<files>
- packages/cli/internal/telemetry/recorder_test.go
- packages/cli/cmd/telemetry_test.go
</files>
<action>
Append to `packages/cli/internal/telemetry/recorder_test.go` (the
existing tests are unchanged; add the new tests after the last
existing test, `TestRecorderFactoryReturnsNoopWhenDisabled` at
line 369):

1. `TestNewRelicRecorderContractEnforced` — the 5 CONTEXT
   assertions, plus the User-Agent check. Use the existing
   `validEvent()` helper (defined in `event_test.go:13`).
   ```go
   func TestNewRelicRecorderContractEnforced(t *testing.T) {
       const (
           accountID  = "test-12345"
           insertKey  = "test-key-xxxxxx"
           versionStr = "0.4.0"
       )
       var (
           gotPath        string
           gotInsertKey   string
           gotUserAgent   string
           gotContentType string
           gotBody        []byte
       )
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           gotPath = r.URL.Path
           gotInsertKey = r.Header.Get("X-Insert-Key")
           gotUserAgent = r.Header.Get("User-Agent")
           gotContentType = r.Header.Get("Content-Type")
           body, err := io.ReadAll(r.Body)
           if err != nil {
               t.Errorf("server: read body: %v", err)
               http.Error(w, "read body", http.StatusInternalServerError)
               return
           }
           gotBody = body
           w.WriteHeader(http.StatusOK)
       }))
       defer srv.Close()

       expectedPath := "/v1/accounts/" + accountID + "/events"
       endpoint := srv.URL + expectedPath
       rec := NewRelicRecorder{
           Endpoint:   endpoint,
           InsertKey:  insertKey,
           HTTPClient: &http.Client{Timeout: 5 * time.Second},
           Version:    versionStr,
       }
       if err := rec.Record(t.Context(), validEvent()); err != nil {
           t.Fatalf("NewRelicRecorder.Record() = %v, want nil", err)
       }

       // Assertion 1: POST URL path.
       if gotPath != expectedPath {
           t.Fatalf("URL path = %q, want %q", gotPath, expectedPath)
       }
       // Assertion 2: X-Insert-Key header.
       if gotInsertKey != insertKey {
           t.Fatalf("X-Insert-Key = %q, want %q", gotInsertKey, insertKey)
       }
       // User-Agent header (bonus, per Phase 4 agent's discretion).
       wantUA := "skill-organizer/" + versionStr
       if gotUserAgent != wantUA {
           t.Fatalf("User-Agent = %q, want %q", gotUserAgent, wantUA)
       }
       if !strings.HasPrefix(gotContentType, "application/json") {
           t.Fatalf("Content-Type = %q, want application/json", gotContentType)
       }
       if !json.Valid(gotBody) {
           t.Fatalf("server body is not valid JSON: %s", gotBody)
       }
       // Assertion 3: body is array of length 1.
       var arr []map[string]any
       if err := json.Unmarshal(gotBody, &arr); err != nil {
           t.Fatalf("json.Unmarshal([]) = %v\nbody = %s", err, gotBody)
       }
       if len(arr) != 1 {
           t.Fatalf("body array length = %d, want 1 (got %s)", len(arr), gotBody)
       }
       // Assertion 4: first element has eventType.
       if arr[0]["eventType"] != "skill_organizer_command" {
           t.Fatalf("arr[0][eventType] = %v, want %q", arr[0]["eventType"], "skill_organizer_command")
       }
       // Assertion 5: the 7 schema fields match the recorder's input
       // (with timestamp renamed to clientTime in the envelope).
       // The 3 deterministic fields are byte-for-byte; the 4 volatile
       // fields are checked against the regexes.
       if arr[0]["command"] != "check-security" {
           t.Fatalf("arr[0][command] = %v, want %q", arr[0]["command"], "check-security")
       }
       if arr[0]["exit_status"] != float64(0) {
           t.Fatalf("arr[0][exit_status] = %v, want 0", arr[0]["exit_status"])
       }
       if arr[0]["version"] != "0.4.0" {
           t.Fatalf("arr[0][version] = %v, want %q", arr[0]["version"], "0.4.0")
       }
       hexRe := regexp.MustCompile(`^[0-9a-f]{32}$`)
       ulidRe := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
       tsRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
       for _, key := range []string{"install_id", "host_id"} {
           v, ok := arr[0][key].(string)
           if !ok {
               t.Fatalf("arr[0][%q] is not a string (got %T)", key, arr[0][key])
           }
           if !hexRe.MatchString(v) {
               t.Fatalf("arr[0][%q] = %q, want 32 hex chars", key, v)
           }
       }
       if v, ok := arr[0]["event_id"].(string); !ok || !ulidRe.MatchString(v) {
           t.Fatalf("arr[0][event_id] = %v, want 26-char ULID", arr[0]["event_id"])
       }
       // The timestamp is renamed to clientTime in the envelope.
       if v, ok := arr[0]["clientTime"].(string); !ok || !tsRe.MatchString(v) {
           t.Fatalf("arr[0][clientTime] = %v, want RFC3339 UTC (the renamed timestamp)", arr[0]["clientTime"])
       }
       // The "timestamp" key must NOT be present in the envelope
       // (NP1 — would be silently dropped at ingest by New Relic).
       if _, present := arr[0]["timestamp"]; present {
           t.Fatalf("arr[0][timestamp] must NOT be present in the New Relic envelope (renamed to clientTime; NP1)")
       }
   }
   ```

2. `TestNewRelicRecorderHardDropsOn413` — asserts 413 returns nil
   AND that the Service's buffer is NOT written (we test the
   Service level because the recorder-level `return nil` is
   meaningless without the consumer's behavior). Use a
   `*countingBuffer`-style check OR use the real Buffer with a
   temp file. RECOMMENDED: use the real Buffer (the simplest path
   to a real assertion). The test:
   ```go
   func TestNewRelicRecorderHardDropsOn413(t *testing.T) {
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
           w.WriteHeader(http.StatusRequestEntityTooLarge)
       }))
       defer srv.Close()
       // Silence the warning; assert it WAS called with the 413
       // message in TestNewRelicRecorderWarningOnHardDrop.
       originalWarning := WarningFunc
       var warnCalled int
       WarningFunc = func(format string, args ...any) { warnCalled++ }
       t.Cleanup(func() { WarningFunc = originalWarning })

       rec := NewRelicRecorder{
           Endpoint:   srv.URL,
           InsertKey:  "test-key",
           HTTPClient: &http.Client{Timeout: 5 * time.Second},
       }
       if err := rec.Record(t.Context(), validEvent()); err != nil {
           t.Fatalf("NewRelicRecorder.Record() on 413 = %v, want nil (hard drop, no buffer fallback)", err)
       }
       if warnCalled != 1 {
           t.Fatalf("WarningFunc called %d times, want 1 (the hard-drop warning)", warnCalled)
       }
   }
   ```
   And the 429 variant is identical with `StatusTooManyRequests`.

3. `TestNewRelicRecorderRetriesOn503` — asserts 503 → 503 → 200
   path, OR 503 → 200 success path. The 250ms backoff means this
   test takes ~250ms:
   ```go
   func TestNewRelicRecorderRetriesOn503(t *testing.T) {
       var hits int
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
           hits++
           if hits == 1 {
               w.WriteHeader(http.StatusServiceUnavailable)
               return
           }
           w.WriteHeader(http.StatusOK)
       }))
       defer srv.Close()
       rec := NewRelicRecorder{
           Endpoint:   srv.URL,
           InsertKey:  "test-key",
           HTTPClient: &http.Client{Timeout: 5 * time.Second},
       }
       if err := rec.Record(t.Context(), validEvent()); err != nil {
           t.Fatalf("Record() = %v, want nil (first 503 retries to 200)", err)
       }
       if hits != 2 {
           t.Fatalf("server hits = %d, want 2 (1 initial + 1 retry)", hits)
       }
   }
   ```

4. `TestNewRelicRecorderHonorsContextCancellation` — asserts that
   a 503 response, with a context cancelled before the backoff
   elapses, returns `ctx.Err()` and does NOT retry:
   ```go
   func TestNewRelicRecorderHonorsContextCancellation(t *testing.T) {
       var hits int
       srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
           hits++
           w.WriteHeader(http.StatusServiceUnavailable)
       }))
       defer srv.Close()
       ctx, cancel := context.WithCancel(t.Context())
       // Cancel the context before the recorder is called. The
       // first send will still hit the server (we already built
       // the request), but the 503 backoff select on ctx.Done()
       // returns immediately, and we never retry.
       cancel()
       rec := NewRelicRecorder{
           Endpoint:   srv.URL,
           InsertKey:  "test-key",
           HTTPClient: &http.Client{Timeout: 5 * time.Second},
       }
       err := rec.Record(ctx, validEvent())
       if !errors.Is(err, context.Canceled) {
           t.Fatalf("Record() on cancelled ctx = %v, want context.Canceled", err)
       }
       if hits != 1 {
           t.Fatalf("server hits = %d, want 1 (cancelled before retry)", hits)
       }
   }
   ```
   Add `"errors"` to the import list of `recorder_test.go`.

5. `TestNewRelicRecorderUserAgentHeader` — already covered by
   `TestNewRelicRecorderContractEnforced` above. SKIP if redundant.

6. `TestRecorderFactoryPicksNewRelicWhenEnvVarsSet` —
   `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: "https://x", AccountID: "1234", InsertKey: "abcd"})`
   → `NewRecorder()` returns `*NewRelicRecorder` whose Endpoint
   has `1234` substituted in the placeholder.

7. `TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete`
   — `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: "https://x", AccountID: "1234"})` (no
   InsertKey) → `NewRecorder()` returns `HTTPRecorder` (the
   passthrough, value not pointer).

8. `TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete` —
   `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: "", AccountID: "1234"})` (no
   InsertKey, no endpoint) → `NewRecorder()` returns `NoopRecorder`.

9. `TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured`
   — `SetDefaultFactory(RecorderConfig{Enabled: true, Endpoint: "https://x"})` (no
   New Relic fields) → `NewRecorder()` returns `HTTPRecorder`. This
   is the Phase 3 happy path; the new fields are zero-valued.

10. Update `packages/cli/cmd/telemetry_test.go` per task 04-01-04
    step 4 (the 3 new status sub-test cases).

11. Add `"errors"` to the `recorder_test.go` import list (for
    `errors.Is` in the cancellation test).
</action>
<verify>
- `go test ./internal/telemetry/... -count=1 -run TestNewRelic` exits 0
- `go test ./cmd/... -count=1 -run TestTelemetryStatus` exits 0
- `go test ./internal/telemetry/... -count=1` exits 0 (no
  regression on the existing 14+ tests)
- `go test ./cmd/... -count=1` exits 0 (no regression on the
  existing 65+ cmd tests)
- The 5 CONTEXT assertions are observable in the test source:
  `r.URL.Path`, `r.Header.Get("X-Insert-Key")`, `len(arr) == 1`,
  `arr[0]["eventType"]`, and the 7 schema fields (with
  `clientTime` instead of `timestamp`)
- The 413/429 hard-drop tests use a stubbed `WarningFunc` to
  assert the warning fired
- The 503 retry test takes ~250ms (acceptable; tests run in <2s)
</verify>
<done>[ ]</done>
</task>

<task id="04-01-06">
<name>Write the 04-01 plan SUMMARY and run final verification</name>
<files>
- .planning/phases/04-observability-product-selection/04-01-plan-SUMMARY.md
- (verification only)
</files>
<action>
1. Write the SUMMARY at
   `.planning/phases/04-observability-product-selection/04-01-plan-SUMMARY.md`.
   Use the same shape as the Phase 3 SUMMARYs (e.g.,
   `03-01-plan-SUMMARY.md`): 1-2 paragraphs on what shipped, the
   5-7 atomic commits, the deviations from the plan (e.g., the
   "snake_case vs camelCase in the inner envelope map" pitfall
   from task 04-01-02 step 4 — the action text had a typo; the
   final code uses snake_case for the project's 6 schema fields),
   and the 2 plan-checker bugs fixed in the executor (if any).
   Include the final must-haves checklist (one bullet per
   must-have from the frontmatter, each with a tick).

2. Run the final verification (the 8 build/test commands from
   Phase 3 plan 03 task 03-06, abbreviated):
   - `go build ./...` — exits 0
   - `go vet ./...` — exits 0
   - `go test ./internal/telemetry/... -count=1` — exits 0
   - `go test ./cmd/... -count=1` — exits 0
   - `go test ./... -count=1` — exits 0 (no regression anywhere
     in the 200+ tests)
   - `git status` — clean except for the new files added in this plan
   - `git diff --stat` — show the line counts of all modifications;
     assert no file outside the `files_modified` list was touched
</action>
<verify>
- `04-01-plan-SUMMARY.md` exists and is 50-150 lines
- All 6 build/test commands exit 0
- `git diff --stat` shows modifications only in:
  - `packages/cli/internal/telemetry/recorder.go`
  - `packages/cli/internal/telemetry/recorder_test.go`
  - `packages/cli/cmd/root.go`
  - `packages/cli/cmd/telemetry.go`
  - `packages/cli/cmd/telemetry_test.go`
  - `04-01-plan-SUMMARY.md` (the new file)
  - `04-01-plan-newrelic-recorder-and-factory.md` (the plan itself)
  No other files modified
- The diff is reviewable in <500 lines
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/telemetry/... -count=1` passes
- [ ] `go test ./cmd/... -count=1` passes
- [ ] `go test ./... -count=1` passes (no regression on the 200+ existing tests)
- [ ] `TestNewRelicRecorderContractEnforced` passes (httptest.NewServer, 5 CONTEXT assertions + User-Agent + the "timestamp key absent" check)
- [ ] `TestNewRelicRecorderHardDropsOn413` passes
- [ ] `TestNewRelicRecorderHardDropsOn429` passes
- [ ] `TestNewRelicRecorderRetriesOn503` passes
- [ ] `TestNewRelicRecorderHonorsContextCancellation` passes
- [ ] `TestRecorderFactoryPicksNewRelicWhenEnvVarsSet` passes
- [ ] `TestRecorderFactoryFallsBackToHTTPRecorderWhenNewRelicIncomplete` passes
- [ ] `TestRecorderFactoryFallsBackToNoopWhenNewRelicIncomplete` passes
- [ ] `TestRecorderFactoryPicksHTTPRecorderWhenNewRelicNotConfigured` passes
- [ ] `TestTelemetryStatusSubcommand` updated assertions pass (Recorder, Account ID, Insert key substrings)
- [ ] `TestTelemetryStatusSubcommand_NewRelicConfigured` passes
- [ ] `TestTelemetryStatusSubcommand_NewRelicIncomplete` passes
- [ ] The 8-line `telemetry status` output is observable in the test
- [ ] The inner envelope map's keys are exactly: `eventType`, `command`, `exit_status`, `install_id`, `host_id`, `clientTime`, `version`, `event_id` (no `timestamp` key)
- [ ] `RecorderConfig` is backwards-compatible: existing callers with `{Enabled, Endpoint}` continue to work
- [ ] No new packages or dependencies
- [ ] `04-01-plan-SUMMARY.md` exists

## Rollback Guide

If this plan fails:

1. Revert the code changes:
   ```
   git checkout -- packages/cli/internal/telemetry/recorder.go \
                    packages/cli/internal/telemetry/recorder_test.go \
                    packages/cli/cmd/root.go \
                    packages/cli/cmd/telemetry.go \
                    packages/cli/cmd/telemetry_test.go
   ```
2. Remove the new plan SUMMARY:
   ```
   rm -f .planning/phases/04-observability-product-selection/04-01-plan-SUMMARY.md
   ```
3. Verify: `go build ./...` and `go test ./...` pass on the
   reverted state (the Phase 3 tests are still green; the
   `NewRelicRecorder` struct and the new factory branch are gone).
4. Retry with smaller scope:
   - First, ship the factory extension + the 4 new factory tests
     (task 04-01-01 step 1-2 + task 04-01-05 steps 6-9). No
     `NewRelicRecorder` struct yet; the factory's NewRelic branch
     is unreachable.
   - Then add the `NewRelicRecorder` struct + `Record` method +
     the 5 CONTEXT smoke tests (tasks 04-01-02 + 04-01-05 steps
     1-5). The struct is wired but the factory's NewRelic branch
     still doesn't pick it (the `if` is unreachable).
   - Then wire the env vars in root.go (task 04-01-03) + extend
     `telemetry status` (task 04-01-04). The full path is now
     reachable end-to-end.
   - Then run the final verification (task 04-01-06).

The 6-task split matches the natural fault lines: (1) data shape
(RecorderConfig + factory), (2) behavior (NewRelicRecorder struct
+ Record), (3) wiring (root.go env reads), (4) user visibility
(status output), (5) acceptance (smoke test), (6) closeout
(SUMMARY + verification). The split is also the natural commit
boundaries (one task = one commit, per the project's
`feat:`/`refactor:`/`test:`/`docs:` convention).

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | The `NewRelicRecorder` returns a non-nil error on 413/429, and the Service's `RecordEvent` writes the event to the on-disk buffer (RESEARCH NP4). The next drain re-POSTs it, the server returns 413/429 again, and the buffer thrashes until FIFO eviction. | Medium | High | The hard-drop test (`TestNewRelicRecorderHardDropsOn413/429`) asserts the recorder returns `nil`. The `WarningFunc` var is the test seam for the warning message. The Service-level test would require a real Buffer + recorder chain; the recorder-level test is sufficient because the Service's "recorder failed → buffer" path is the only consumer of the error. |
| 2 | The `timestamp` field is sent in the envelope (instead of being renamed to `clientTime`), and the New Relic server silently drops the field at ingest. The user sees no error; the data is lost. | Low | High | The `TestNewRelicRecorderContractEnforced` test asserts (a) `arr[0]["clientTime"]` is present and matches the RFC3339 regex, AND (b) `arr[0]["timestamp"]` is NOT present. The test fails if either is wrong. The action text in task 04-01-02 step 4 calls out the snake_case key set as a deliberate pitfall guard. |
| 3 | The 503 retry uses `time.Sleep(250ms)` instead of `time.After` + `select` on `ctx.Done()`, and a cancelled context (Ctrl-C during the backoff) is not honored until the sleep returns. The 10-second HTTP client timeout makes this a slow cancellation in a CLI. | Low | Medium | The action text in task 04-01-02 step 2 mandates the `time.After` + `select` pattern. The `TestNewRelicRecorderHonorsContextCancellation` test asserts the cancellation returns `context.Canceled` and does not retry. The test cancels the context before `Record` is called, so the first send hits the server (the request was already built) but the backoff's `select` returns immediately. |
| 4 | The factory closure is set in `SetDefaultFactory` but `telemetrypkg.New(...)` is called BEFORE the closure is set, so the `Service.Recorder` field captures the old (no-op) factory. BUG #2 from Phase 3 (see STATE.md "Recent decisions"). | Low | High | The action text in task 04-01-03 step 1 explicitly mandates: `SetDefaultFactory` runs BEFORE `telemetrypkg.New(...)`. The 4 factory tests in task 04-01-05 call `SetDefaultFactory` then `NewRecorder()` in that order, so any swap-after-`New` bug is caught at test time. |
| 5 | The `WarningFunc` package var is reassigned by a test but not restored in `t.Cleanup`, leaking the stub into other tests. | Low | Medium | Every test that reassigns `WarningFunc` wraps the swap in `t.Cleanup(func() { WarningFunc = originalWarning })`. The pattern is the same as the `countingTransport` test in Phase 3's `recorder_test.go:284-304`. |
| 6 | The `telemetry status` command's `SetDefaultFactory` call in `RunE` interferes with other tests' factory state (the cmd package is global). | Low | Medium | The `telemetry status` test in `telemetry_test.go` already stubs `telemetryLoadConfig` / `telemetryAppDir` / `telemetryIdentity` / `telemetryInfo` / `telemetrySuccess` in `t.Cleanup`. Add `RecorderFactoryFunc` restoration to the same `t.Cleanup`. The 3 new status tests follow the pattern. |
| 7 | The inner envelope map's key naming drifts from snake_case (e.g., a future PR changes `install_id` to `installId` to "match the New Relic docs example"). NRQL is case-sensitive (RESEARCH NP2), and the case-only difference silently breaks server-side queries. | Low | High | The `TestNewRelicRecorderContractEnforced` test asserts `arr[0]["install_id"]` (snake_case) is present and matches the 32-hex regex. The action text in task 04-01-02 step 4 calls out the snake_case key set as the schema contract; the camelCase keys are reserved for the New Relic-specific fields (`eventType`, `clientTime`). |
| 8 | The `http.Client` used by the NewRelicRecorder is built once at construction time and reused across all events. A future change that uses `http.DefaultClient` (no timeout) would create a slow-CLI regression when the New Relic endpoint is slow. | Low | Medium | The `NewNewRelicRecorder` constructor calls `NewHTTPClientFunc()`, which returns `&http.Client{Timeout: 10 * time.Second}` (Phase 3). The 5-second test timeout in the smoke test is a separate concern; production uses the 10s timeout. The action text in task 04-01-01 step 4 calls out `NewHTTPClientFunc()` as the source of the client. |

## Commit Message

```
feat(cli): add NewRelicRecorder with hard-drop on 413/429, 1-retry on 503

- Add NewRelicRecorder struct in internal/telemetry/recorder.go
  (Endpoint, InsertKey, HTTPClient, Version fields)
- Add NewNewRelicRecorder constructor (substitutes the account_id
  placeholder at construction time, uses NewHTTPClientFunc)
- Add newRelicEndpointTemplate constant (New Relic Insights Events
  API URL with $ACCOUNT_ID placeholder)
- Recorder.Record: builds a JSON array envelope of length 1 with
  eventType="skill_organizer_command" + the 7 schema fields, with
  timestamp RENAMED to clientTime in the envelope (NP1 — New Relic
  reserves "timestamp" for Unix-epoch integers)
- Recorder.Record: sets X-Insert-Key auth header (CONTEXT lock)
  and User-Agent: skill-organizer/<version>
- Status code handling: 2xx → return nil; 413/429 → log warning via
  WarningFunc, return nil (HARD DROP, no buffer write — NP4); 503 →
  1 retry with 250ms context-aware backoff (time.After + select on
  ctx.Done() — NP3); other 4xx/5xx/network → return error
- Add WarningFunc package var (test seam for the hard-drop warning,
  uses pterm.Warning light-magenta per the project's color rules)
- Add RecorderVersion package var (User-Agent source; set by
  cmd/root.go at PersistentPreRun)
- Extend RecorderConfig with AccountID and InsertKey fields
  (backwards-compatible: existing callers with {Enabled, Endpoint}
  continue to work)
- SetDefaultFactory: 3-way switch — both New Relic env vars set
  → NewRelicRecorder; else HTTPRecorder (if Endpoint); else
  NoopRecorder
- Wire SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID and
  SKILL_ORGANIZER_NEWRELIC_INSERT_KEY env vars in cmd/root.go's
  PersistentPreRun (SetDefaultFactory runs BEFORE telemetrypkg.New
  per the BUG #2 fix)
- Extend telemetry status to print 3 new lines: Recorder type,
  Account ID prefix (first 4 chars + "..."), Insert key presence
- Add TestNewRelicRecorderContractEnforced (5 CONTEXT assertions
  + User-Agent + "timestamp key absent" check, httptest.NewServer)
- Add TestNewRelicRecorderHardDropsOn413/429 (asserts Record
  returns nil and WarningFunc was called)
- Add TestNewRelicRecorderRetriesOn503 (asserts server hit count
  == 2 after 503 → 200)
- Add TestNewRelicRecorderHonorsContextCancellation (asserts
  context.Canceled is returned and the retry is skipped)
- Add 4 factory tests (NewRelic picked, fall back to HTTPRecorder,
  fall back to Noop, Phase 3 happy path preserved)
- Add 3 new cmd/telemetry_test.go status sub-tests (NewRelic
  configured, NewRelic incomplete, default)
- No changes to the Event struct or the byte-for-byte schema test
- No new packages or dependencies
- Closes REQ-9 acceptance: New Relic Insights Events API is the
  configured backend; the factory picks it when both env vars are
  set; the recorder's contract is enforced by httptest tests
```
