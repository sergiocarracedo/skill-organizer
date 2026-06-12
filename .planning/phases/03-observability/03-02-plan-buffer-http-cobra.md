---
wave: 2
depends_on:
  - 03-01-plan-package-identity-interface.md
files_modified:
  - packages/cli/internal/telemetry/recorder.go
  - packages/cli/internal/telemetry/buffer.go
  - packages/cli/internal/telemetry/prompt.go
  - packages/cli/internal/telemetry/telemetry.go
  - packages/cli/internal/telemetry/recorder_test.go
  - packages/cli/internal/telemetry/buffer_test.go
  - packages/cli/internal/telemetry/prompt_test.go
  - packages/cli/internal/telemetry/telemetry_test.go
  - packages/cli/internal/config/config.go
  - packages/cli/internal/config/registry.go
  - packages/cli/cmd/root.go
  - packages/cli/cmd/telemetry.go
  - packages/cli/cmd/telemetry_test.go
autonomous: true
single_layer_justified: false
requirement: REQ-8
objective: "Wire the cobra command surface, the on-disk buffer, the HTTPRecorder, the first-run prompt, the alias-canonicalising event-emit hook into root.go's PersistentPreRun/PostRun, and the telemetry subcommand (enable|disable|status|rotate-host-id). Verifiable by go test ./internal/telemetry/... ./cmd/... and go build ./... succeeding, plus a demo of telemetry status printing current state."
must_haves:
  - "go build ./... succeeds"
  - "go test ./internal/telemetry/... ./cmd/... passes"
  - "TestBufferAppendAndRead passes (3 events round-trip via Drain)"
  - "TestBufferFIFOEvictionAt1MB passes (write >1MB, drain returns the latest ~1MB, oldest events evicted)"
  - "TestBufferDrainIdempotent passes (second drain returns 0 events)"
  - "TestHTTPRecorderSmokeOK passes (httptest.NewServer, asserts POST body is well-formed JSON with 7 expected top-level keys)"
  - "TestFirstRunPromptStickyYesNo passes (TTY-gated, no func var = no prompt)"
  - "TestFirstRunPromptNonTTYSkippedAndNotPersisted passes (Pitfall P10: non-TTY does NOT write the answer to YAML)"
  - "TestCommandNameNormalization passes (top-level aliases pass through; mapping for `on`->`enable`, `install`->`add` works)"
  - "TestService_RecordEvent_WritesToBufferOnFailure passes (HTTPRecorder fails -> buffer file grows)"
  - "TestService_RecordEvent_NoEgressWhenDisabled passes (NoopRecorder -> no file growth, no HTTP call)"
  - "TestService_DrainBuffer_SendsAndTruncates passes"
  - "TestTelemetryEnableDisableStatusSubcommands pass (cmd-level tests with stubbed LoadAppConfig/SaveAppConfig)"
  - "TestTelemetryRotateHostIDSubcommand passes (writes new host_id to <AppDir>/host_id)"
  - "TestRootPersistentPostRun_EmitsEvent passes (root.go integration: stubbed recorder, assert a single Event with the right command name)"
  - "TestRootPersistentPreRun_SkipsTelemetryCommand passes (running the telemetry subcommand does NOT fire the first-run prompt)"
  - "config.TelemetryConfig exists, has yaml tags `enabled`/`endpoint`, and is loaded/saved via LoadTelemetryConfigOrDefault/SaveTelemetryConfig in registry.go"
  - "The PersistentPreRun guard is extended to also skip the `telemetry` subcommand (the existing `cmd == rootCmd || name in {completion, help}` gate)"
  - "A new --telemetry-endpoint persistent flag is registered on rootCmd, read AFTER cobra parsing (flag > env > YAML precedence)"
  - "cmd/telemetry.go implements enable, disable, status, and rotate-host-id subcommands"
  - "NoOpRecorder zero-allocation smoke test in plan 01 still passes (no regression)"
---

# Plan 03-02: Buffer, HTTPRecorder, first-run prompt, cobra hook, telemetry subcommand

## Objective

Wire the cobra command surface that drives REQ-8. This plan adds: a JSONL `Buffer` for offline-retry (1 MB cap, FIFO eviction, O_APPEND writes), an `HTTPRecorder` that POSTs to a configured endpoint, a TTY-gated `FirstRunPrompt`, alias-canonicalising `Service.RecordEvent`, the `PersistentPreRun` + `PersistentPostRun` hooks in `cmd/root.go`, the `--telemetry-endpoint` flag with three-layer precedence (flag > env > YAML), the `TelemetryConfig` YAML struct, and a new `cmd/telemetry.go` with `enable`/`disable`/`status`/`rotate-host-id` subcommands. By the end, `go test ./...` is green and a user can demo the full telemetry loop: enable → run a command → see an event leave the buffer; disable → run → no event, no buffer write.

## Context

Plan 01 shipped the `Event` struct, `Recorder` interface, `NoopRecorder`, `Identity`, and the swappable `RecorderFactoryFunc` (default: noop). This plan replaces the default factory with one that returns `HTTPRecorder` when telemetry is enabled AND an endpoint is configured, else `NoopRecorder`. Per RESEARCH P1, precedence is YAML < env < flag; the `NewRecorder` call site (in `Service.RecordEvent`) must resolve the final endpoint after cobra parsing but before the factory runs.

The disk buffer lives at `<AppDir>/telemetry-buffer.jsonl` (RESEARCH P3). `O_APPEND` makes the per-event write atomic (we're 20× under the POSIX `PIPE_BUF` of 4096). The drain step is opportunistic: read all events, POST each, on full success truncate; on any error, leave the events in place and append the new event. Per P7, the cap check is a post-condition: append first, then `os.Stat` and rewrite if over 1 MB. Per P10, the first-run prompt in non-TTY mode does NOT write the answer to YAML — the default sticks (next TTY run gets the prompt).

The cobra hook placement extends the existing `PersistentPreRun` guard (`cmd/root.go:72-78`) which already skips `completion` and `help`. We add `cmd.Name() == "telemetry"` to the skip set, so the telemetry subcommand manages its own state without firing the prompt. The `PersistentPostRun` is a new hook (none exists in plan 01) — it emits the per-command event for any command NOT in the skip set.

The `Service` type (in `telemetry.go`) is the umbrella: it holds the `Recorder`, the `Buffer`, the `Identity`, and a `version string`. `Service.RecordEvent(ctx, event)` is the single write path. `Service.DrainBuffer(ctx)` is the opportunistic drain. `MaybeRunFirstRunPrompt(ctx, stdout, stdin)` is the fire-and-forget prompt (matches the `maintenance.MaybeNotify*` pattern at `packages/cli/internal/maintenance/maintenance.go:19-49`).

## Tasks

<task id="03-02-01">
<name>Add TelemetryConfig struct and YAML Load/Save helpers in config package</name>
<files>
- packages/cli/internal/config/config.go
- packages/cli/internal/config/registry.go
</files>
<action>
Modify `packages/cli/internal/config/config.go`:

1. Add a new struct after `BackupConfig` (line 43-45):
   ```go
   // TelemetryConfig holds the opt-in telemetry settings. The Enabled flag
   // controls whether the first-run prompt is sticky-yes; the Endpoint
   // field is the URL the HTTPRecorder POSTs events to. An empty Endpoint
   // forces the recorder to NoopRecorder regardless of Enabled (per CONTEXT).
   type TelemetryConfig struct {
       Enabled  bool   `yaml:"enabled"`
       Endpoint string `yaml:"endpoint,omitempty"`
   }
   ```

2. Add `Telemetry TelemetryConfig` to the `AppConfig` struct (line 47-52), in the same style as `Service`, `AgentSelection`, `Backup`:
   ```go
   type AppConfig struct {
       Watched        []string             `yaml:"watched"`
       Service        ServiceConfig        `yaml:"service"`
       AgentSelection AgentSelectionConfig `yaml:"agent-selection,omitempty"`
       Backup         BackupConfig         `yaml:"backup,omitempty"`
       Telemetry      TelemetryConfig      `yaml:"telemetry,omitempty"`
   }
   ```

3. Add a `Normalize` method for `TelemetryConfig`:
   ```go
   func (c *TelemetryConfig) Normalize() {
       c.Endpoint = strings.TrimSpace(c.Endpoint)
   }
   ```
   (Add `strings` to the imports if not already present — line 5 of config.go.)

4. Call `c.Telemetry.Normalize()` from `(c *AppConfig) Normalize()` (line 76-83) and `(c *AppConfig) SetWatchRegistry` (line 91-97).

Modify `packages/cli/internal/config/registry.go`:

5. Add the three helpers (Load / LoadOrDefault / Save) mirroring the `BackupConfig` pattern at lines 181-204:
   ```go
   func LoadTelemetryConfig(path string) (TelemetryConfig, error) {
       cfg, err := LoadAppConfig(path)
       if err != nil {
           return TelemetryConfig{}, err
       }
       return cfg.Telemetry, nil
   }

   func LoadTelemetryConfigOrDefault(path string) (TelemetryConfig, error) {
       cfg, err := LoadAppConfigOrDefault(path)
       if err != nil {
           return TelemetryConfig{}, err
       }
       return cfg.Telemetry, nil
   }

   func SaveTelemetryConfig(path string, t TelemetryConfig) error {
       cfg, err := LoadAppConfigOrDefault(path)
       if err != nil {
           return err
       }
       cfg.Telemetry = t
       return SaveAppConfig(path, cfg)
   }
   ```

6. NO new imports required in `registry.go` (everything is in the same package).

7. Do NOT touch any other struct or function. The yaml tags follow the existing repo convention: `enabled` (no `omitempty` because `false` is meaningful — the zero value is a valid disabled state) and `endpoint,omitempty` (empty string is the unset default).
</action>
<verify>
- `go build ./...` succeeds
- `go test ./internal/config/...` passes (no regression)
- A new test `TestTelemetryConfigRoundtrip` in `packages/cli/internal/config/registry_test.go` (add it if not present; otherwise the file may need to be created) verifies that `LoadTelemetryConfigOrDefault` returns a zero-value `TelemetryConfig` on a fresh AppDir, and `SaveTelemetryConfig` + `LoadTelemetryConfigOrDefault` round-trips the {Enabled: true, Endpoint: "https://example.com/in"} case.
- `gofmt -d packages/cli/internal/config/config.go packages/cli/internal/config/registry.go` is empty
</verify>
<done>[ ]</done>
</task>

<task id="03-02-02">
<name>Add HTTPRecorder implementation to recorder.go</name>
<files>
- packages/cli/internal/telemetry/recorder.go
</files>
<action>
Append the `HTTPRecorder` type to `packages/cli/internal/telemetry/recorder.go` (the file already has `Recorder`, `NoopRecorder`, `RecorderFactoryFunc`, `NewHTTPClientFunc` from plan 01). Plan 02 also replaces the default `RecorderFactoryFunc` with one that consults a package-level config (introduced below).

1. Add imports if not already present:
   - `bytes`
   - `encoding/json`
   - `fmt`
   - `net/http`

2. Define the HTTPRecorder struct:
   ```go
   // HTTPRecorder POSTs each event as a JSON body to a fixed endpoint.
   // The transport is swappable via NewHTTPClientFunc (the package var
   // declared above), so the counting-transport test in plan 03 can
   // intercept the call. The HTTP client has a 10s timeout (the default
   // from NewHTTPClientFunc).
   type HTTPRecorder struct {
       Endpoint string
       Client   *http.Client
   }

   func (r HTTPRecorder) Record(ctx context.Context, event Event) error {
       body, err := json.Marshal(event)
       if err != nil {
           return fmt.Errorf("marshal event: %w", err)
       }
       req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(body))
       if err != nil {
           return fmt.Errorf("build request: %w", err)
       }
       req.Header.Set("Content-Type", "application/json")
       resp, err := r.Client.Do(req)
       if err != nil {
           return fmt.Errorf("post event: %w", err)
       }
       defer resp.Body.Close()
       // 2xx is success. 4xx and 5xx are failure (append to buffer).
       // 3xx is followed by the default http.Client (so it counts as success if the final code is 2xx).
       if resp.StatusCode < 200 || resp.StatusCode >= 300 {
           return fmt.Errorf("post event: unexpected status %d", resp.StatusCode)
       }
       return nil
   }
   ```

3. Add a `NewHTTPRecorder(endpoint string) Recorder` constructor:
   ```go
   // NewHTTPRecorder returns an HTTPRecorder with a default client.
   func NewHTTPRecorder(endpoint string) Recorder {
       return HTTPRecorder{Endpoint: endpoint, Client: NewHTTPClientFunc()}
   }
   ```

4. Add a small `RecorderConfig` struct in the same file (or in `telemetry.go`, your choice; the file boundary is informal):
   ```go
   // RecorderConfig is the input to the default factory. Plan 02 sets
   // this from Service; the cmd package's PersistentPostRun is the only
   // caller.
   type RecorderConfig struct {
       Enabled  bool
       Endpoint string
   }
   ```

5. Replace the plan-01 default `RecorderFactoryFunc` with a closure that takes a `RecorderConfig`. Because `RecorderFactoryFunc` is `func() Recorder` (no args), we add a separate `SetDefaultFactory(cfg RecorderConfig)`:
   ```go
   // SetDefaultFactory swaps RecorderFactoryFunc to a closure that returns
   // an HTTPRecorder when both Enabled and Endpoint are set, else a
   // NoopRecorder. Idempotent: calling it with the same cfg is a no-op.
   func SetDefaultFactory(cfg RecorderConfig) {
       RecorderFactoryFunc = func() Recorder {
           if !cfg.Enabled || cfg.Endpoint == "" {
               return NoopRecorder{}
           }
           return NewHTTPRecorder(cfg.Endpoint)
       }
   }
   ```

6. The `RecorderFactoryFunc` is `var RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }` from plan 01. The first call to `SetDefaultFactory` (in plan 02's `Service` constructor) replaces it; subsequent calls re-replace. Tests in plan 03 may also call it.

7. Add a one-line comment that the "0 network egress when disabled" guarantee is preserved: the closure returns a `NoopRecorder{}` value, which has no methods that touch the network.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The HTTPRecorder smoke test (`TestHTTPRecorderSmokeOK`, added in task 03-02-08) passes
- `go test ./internal/telemetry/...` exits 0 (no regression on the plan 01 tests)
</verify>
<done>[ ]</done>
</task>

<task id="03-02-03">
<name>Create buffer.go with JSONL spool, O_APPEND writes, 1MB FIFO eviction</name>
<files>
- packages/cli/internal/telemetry/buffer.go
</files>
<action>
Create `packages/cli/internal/telemetry/buffer.go` (package `telemetry`). The buffer is the offline-retry spool from CONTEXT's "Network gating & offline buffering" section.

1. Imports:
   - `bufio`
   - `encoding/json`
   - `fmt`
   - `os`
   - `sync`
   - `time`

2. Constants:
   ```go
   // BufferFileName is the on-disk JSONL file inside the app dir.
   const BufferFileName = "telemetry-buffer.jsonl"

   // BufferMaxBytes is the post-condition cap (per CONTEXT). Exceeding
   // it triggers a FIFO eviction pass that drops the oldest events.
   const BufferMaxBytes = 1 << 20 // 1 MB
   ```

3. The `Buffer` type:
   ```go
   // Buffer is the JSONL spool for offline-retry. The Path is
   // <appDir>/telemetry-buffer.jsonl (the cmd package passes it in
   // from configpkg.AppDir()). Concurrent CLI processes are safe: writes
   // use O_APPEND (POSIX-atomic up to PIPE_BUF = 4096, a single event
   // is ~200 bytes). Drains read the whole file, call the callback per
   // event, and on full success truncate; the rare mid-truncate crash
   // loses at most one event, accepted per CONTEXT ("opportunistic drain").
   type Buffer struct {
       Path string

       mu sync.Mutex // serializes Append + Drain within one process
   }
   ```

4. `NewBuffer(path string) *Buffer` — trivial constructor.

5. `func (b *Buffer) Append(event Event) error`:
   - Take the lock.
   - Call `os.MkdirAll(filepath.Dir(b.Path), 0o755)` (the app dir may not exist).
   - Marshal the event with `json.Marshal` (deterministic struct order from plan 01).
   - Open the file with `os.OpenFile(b.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)`.
   - `f.Write(marshalled)` followed by `f.Write([]byte("\n"))`.
   - `f.Close()`.
   - `os.Stat(b.Path)`; if `info.Size() > BufferMaxBytes`, call `b.evict()`.
   - Return any wrapped error.

6. `func (b *Buffer) evict() error`:
   - Read the whole file with `os.ReadFile`.
   - Split on `'\n'`; drop events from the front (oldest first) until the remaining bytes (joined with `'\n'` + trailing `'\n'`) are `<= BufferMaxBytes`.
   - `os.WriteFile(b.Path, remaining, 0o644)`.
   - Return any error.
   - This is the rare path; the file is at most 1 MB, so the rewrite is sub-millisecond (per RESEARCH P7).

7. `func (b *Buffer) Drain(send func(Event) error) error`:
   - Take the lock.
   - Open the file for reading with `os.Open`.
   - If `os.IsNotExist(err)`, return nil (empty buffer is the steady state).
   - `bufio.NewScanner(f).Scan()` line by line. For each non-empty line, `json.Unmarshal` into an `Event`; call `send(event)`. If `send` returns a non-nil error, STOP the scan, return the error WITHOUT truncating.
   - If the loop completes (every event was sent successfully), `os.WriteFile(b.Path, nil, 0o644)` (truncate). Return nil.
   - On any scanner error or unmarshal error, return the error WITHOUT truncating (the un-sent events are preserved).

8. The `evict` is called from `Append` while holding the lock; it must NOT recurse into `Append`. The drain's `send` callback runs while holding the lock too — the callback should be quick (a single HTTP POST), and the lock prevents two drains from interleaving.

9. Add doc comments referencing CONTEXT's "1 MB FIFO eviction" decision and RESEARCH's O_APPEND rationale.

10. Do NOT add a `Close` method. The file is opened per-operation, not kept open.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The buffer tests in task 03-02-08 (`TestBufferAppendAndRead`, `TestBufferFIFOEvictionAt1MB`, `TestBufferDrainIdempotent`) all pass
- `go test ./internal/telemetry/... -run TestBuffer` exits 0
</verify>
<done>[ ]</done>
</task>

<task id="03-02-04">
<name>Create prompt.go with TTY-gated FirstRunPrompt and func-var seam</name>
<files>
- packages/cli/internal/telemetry/prompt.go
</files>
<action>
Create `packages/cli/internal/telemetry/prompt.go` (package `telemetry`). The prompt is the fire-and-forget `MaybeRunFirstRunPrompt` from the `maintenance.MaybeNotify*` precedent.

1. Imports:
   - `bufio`
   - `context`
   - `fmt`
   - `io`
   - `os`

2. The func-var test seam (mirroring `agenttools.ChooseAgentToolFunc`):
   ```go
   // IsStdInTTYFunc is a swappable function variable for IsStdInTTY.
   // Production calls IsStdInTTY(os.Stdin) via the implementation; tests
   // reassign to control the TTY behaviour.
   var IsStdInTTYFunc = func() bool { return isStdInTTY(os.Stdin) }
   ```

3. The TTY check uses `golang.org/x/term` (already a transitive dep from cobra — no go.mod change needed):
   ```go
   // isStdInTTY returns true if the given fd is a terminal. The check
   // delegates to golang.org/x/term.IsTerminal, which on Windows uses
   // console mode APIs and on POSIX uses isatty(2). Non-terminal inputs
   // (pipes, files, redirected stdin) return false.
   func isStdInTTY(f *os.File) bool {
       return term.IsTerminal(int(f.Fd()))
   }
   ```
   Import: `golang.org/x/term`.

4. The prompt function (the actual UI is delegated to the cmd package via a func var, mirroring the maintenance pattern):
   ```go
   // ConfirmFunc is a swappable function variable for the yes/no prompt.
   // Plan 02 wires it in the cmd package to point at cmd.confirm
   // (pterm.DefaultInteractiveConfirm). The reason this is a func var
   // and not a direct call is that cmd.confirm lives in the cmd package
   // and would create a circular import; the func var breaks the cycle.
   var ConfirmFunc = defaultConfirm

   func defaultConfirm(prompt string, defaultValue bool) (bool, error) {
       fmt.Fprintf(io.Discard, "telemetry: confirm not wired (prompt=%q default=%v)\n", prompt, defaultValue)
       return defaultValue, nil
   }
   ```
   (The `defaultConfirm` is a safe no-op that respects the default. The cmd package's `init()` (added in task 03-02-07) overrides `ConfirmFunc` to point at `cmd.confirm`.)

5. The public `FirstRunPrompt`:
   ```go
   // FirstRunPrompt asks the user whether to enable anonymous telemetry.
   // Returns (true, nil) on yes, (false, nil) on no, and (false, err)
   // on error (e.g. ctrl-c). The default answer is `false` (CONTEXT:
   // "Default = off"). The caller (MaybeRunFirstRunPrompt) is responsible
   // for persisting the answer.
   func FirstRunPrompt(stdout io.Writer, stdin io.Reader) (bool, error) {
       return ConfirmFunc("Enable anonymous telemetry? (only command names, no args/paths/PII)", false)
   }
   ```

6. The fire-and-forget wrapper (matches the `maintenance.MaybeNotify*` signature — `func(ctx, stdout)`, returns nothing):
   ```go
   // MaybeRunFirstRunPrompt shows the first-run opt-in prompt exactly
   // once per install, when:
   //   1. The user has not yet answered (caller checks cfg.Telemetry
   //      exists or sentinel file in appDir — see Service).
   //   2. stdin is a TTY (CI / piped input skips the prompt).
   //
   // On non-TTY, the function returns silently without writing the
   // answer to YAML: the next TTY run will re-prompt. (Pitfall P10.)
   //
   // Errors are swallowed (matches MaybeNotifySkillUpdates precedent):
   // the first-run prompt's failure mode is "user pressed Ctrl-C" or
   // "stdin closed", both user-initiated, no error message needed.
   func MaybeRunFirstRunPrompt(ctx context.Context, stdout io.Writer, stdin io.Reader, onAnswer func(yes bool) error) {
       if !IsStdInTTYFunc() {
           return
       }
       answer, err := FirstRunPrompt(stdout, stdin)
       if err != nil {
           return
       }
       _ = onAnswer(answer)
   }
   ```

7. The "answered-once" state lives on disk at `<AppDir>/telemetry-prompted` (a sentinel file). Plan 02's `Service` (next task) checks for this file; if present, `MaybeRunFirstRunPrompt` is a no-op. Plan 02 does not need to expose this as a public API — the Service handles it.

8. Add doc comments on every exported symbol referencing CONTEXT's first-run decisions and the P10 pitfall.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The prompt tests in task 03-02-09 (`TestFirstRunPromptStickyYesNo`, `TestFirstRunPromptNonTTYSkippedAndNotPersisted`) all pass
- `golang.org/x/term` is the only new import (no go.mod change required because it's already a transitive dep)
</verify>
<done>[ ]</done>
</task>

<task id="03-02-05">
<name>Create telemetry.go with Service, RecordEvent, MaybeDrainBuffer, commandName normalization</name>
<files>
- packages/cli/internal/telemetry/telemetry.go
</files>
<action>
Create `packages/cli/internal/telemetry/telemetry.go` (package `telemetry`). The Service is the umbrella that wires Recorder + Buffer + Identity + config into a single object the cmd package uses.

1. Imports:
   - `context`
   - `fmt`
   - `io`
   - `os`
   - `path/filepath`
   - `strings`
   - `time`

2. Constants:
   ```go
   // promptSentinelFile marks that the first-run prompt has been answered.
   // It is separate from TelemetryConfig.Enabled so the non-TTY fallback
   // (Pitfall P10) can re-prompt on the next TTY run: the sentinel is
   // only written when the user actually answered.
   const promptSentinelFile = "telemetry-prompted"
   ```

3. The `Service` struct:
   ```go
   // Service is the runtime telemetry object. One Service per CLI
   // invocation. The cmd package constructs it in PersistentPreRun
   // and uses it in PersistentPostRun.
   type Service struct {
       AppDir   string
       Identity Identity
       Version  string
       Cfg      TelemetryConfig // for inspection; the recorder factory reads from a snapshot
       Recorder Recorder
       Buffer   *Buffer
   }
   ```
   (Note: `Service` here is the package's type, not the cmd package's `service` package — they are unrelated.)

4. The `New` constructor:
   ```go
   // New builds a Service from a resolved TelemetryConfig and a resolved
   // appDir. The factory is set via SetDefaultFactory BEFORE New is
   // called, so RecorderFactoryFunc is the current production closure.
   // Identity is loaded (or created) from <appDir>/{install_id, host_id}.
   // Buffer is a JSONL spool at <appDir>/telemetry-buffer.jsonl.
   func New(appDir string, version string, cfg TelemetryConfig) (*Service, error) {
       if err := os.MkdirAll(appDir, 0o755); err != nil {
           return nil, fmt.Errorf("create app dir: %w", err)
       }
       identity, err := LoadOrCreate(appDir)
       if err != nil {
           return nil, fmt.Errorf("load identity: %w", err)
       }
       return &Service{
           AppDir:   appDir,
           Identity: identity,
           Version:  version,
           Cfg:      cfg,
           Recorder: NewRecorder(),
           Buffer:   NewBuffer(filepath.Join(appDir, BufferFileName)),
       }, nil
   }
   ```

5. `RecordEvent` — the single write path. Resolves the command name (alias canonicalisation), fills the 7 fields, calls Recorder, falls back to Buffer on failure:
   ```go
   // RecordEvent is the single write path called from PersistentPostRun.
   // On Recorder error, the event is appended to the on-disk buffer for
   // a later opportunistic drain. Returns any error from the recorder
   // OR the buffer (whichever fails last).
   func (s *Service) RecordEvent(ctx context.Context, command string, exitStatus int) error {
       event := Event{
           Command:    command,
           ExitStatus: exitStatus,
           InstallID:  s.Identity.InstallID,
           HostID:     s.Identity.HostID,
           Timestamp:  NewTimestamp(),
           Version:    s.Version,
           EventID:    NewEventID(),
       }
       if err := event.Validate(); err != nil {
           return fmt.Errorf("invalid event: %w", err)
       }
       recErr := s.Recorder.Record(ctx, event)
       if recErr == nil {
           return nil
       }
       // Recorder failed: append to buffer for later drain.
       if bufErr := s.Buffer.Append(event); bufErr != nil {
           return fmt.Errorf("record failed (%v) and buffer write failed (%w)", recErr, bufErr)
       }
       return recErr
   }
   ```

6. `DrainBuffer` — opportunistic drain on each run:
   ```go
   // DrainBuffer reads every buffered event and tries to send it via the
   // Recorder. On success, the buffer is truncated. On any send failure,
   // the drain stops and the unsent events are preserved.
   func (s *Service) DrainBuffer(ctx context.Context) error {
       return s.Buffer.Drain(func(e Event) error {
           return s.Recorder.Record(ctx, e)
       })
   }
   ```

7. `MaybeRunFirstRunPrompt` — fire-and-forget wrapper that checks the sentinel and only prompts in TTY:
   ```go
   // MaybeRunFirstRunPrompt shows the first-run prompt if and only if:
   //   1. stdin is a TTY
   //   2. <appDir>/telemetry-prompted does not exist
   //
   // On non-TTY (CI, pipes), the function returns silently WITHOUT
   // writing the sentinel — the next TTY run will re-prompt (Pitfall
   // P10). On answer, the sentinel is created AND the cfg is updated
   // (via the onAnswer callback the cmd package provides).
   func (s *Service) MaybeRunFirstRunPrompt(ctx context.Context, stdout io.Writer, stdin io.Reader, onAnswer func(yes bool) error) {
       sentinelPath := filepath.Join(s.AppDir, promptSentinelFile)
       if _, err := os.Stat(sentinelPath); err == nil {
           return // already answered
       }
       maybeRunFirstRunPromptImpl(ctx, stdout, stdin, onAnswer, sentinelPath)
   }
   ```
   The `maybeRunFirstRunPromptImpl` lives in `prompt.go` (next to the TTY check) and writes the sentinel after a successful onAnswer.

8. Alias canonicalisation:
   ```go
   // commandNameAliases maps short alias names to their canonical form
   // for the schema's `command` field. The aliases are the top-level
   // shortcuts registered in root.go (add, delete, enable, disable,
   // check-updates) plus a few common ones (on, install, rm) for
   // forward compatibility. The function returns the canonical name;
   // unknown names pass through unchanged.
   var commandNameAliases = map[string]string{
       "on":     "enable",
       "off":    "disable",
       "install": "add",
       "rm":     "delete",
   }

   // NormalizeCommandName returns the canonical event-emit name for a
   // cobra command. Unknown names pass through. Always returns a
   // non-empty string.
   func NormalizeCommandName(name string) string {
       if canonical, ok := commandNameAliases[name]; ok {
           return canonical
       }
       return name
   }
   ```

9. Add a helper `ResolveEndpoint(flagValue, envValue, yamlValue string) string` that returns the highest-precedence non-empty value (flag > env > yaml):
   ```go
   func ResolveEndpoint(flagValue, envValue, yamlValue string) string {
       if strings.TrimSpace(flagValue) != "" {
           return flagValue
       }
       if strings.TrimSpace(envValue) != "" {
           return envValue
       }
       return yamlValue
   }
   ```
   (The cmd package calls this from the PersistentPreRun before constructing the Service.)

10. Add a one-line package-level comment in telemetry.go (above `package telemetry`) noting that the cmd package is the only intended caller for the public API.

11. The `Service` type is the cmd package's only seam to the telemetry layer; do NOT export helpers that the cmd package does not need.
</action>
<verify>
- `go build ./internal/telemetry/...` exits 0
- `go vet ./internal/telemetry/...` exits 0
- The Service tests in task 03-02-10 (`TestService_RecordEvent_*`, `TestService_DrainBuffer_*`, `TestCommandNameNormalization`, `TestResolveEndpointPrecedence`) all pass
- `go test ./internal/telemetry/...` exits 0
</verify>
<done>[ ]</done>
</task>

<task id="03-02-06">
<name>Wire root.go: extend PersistentPreRun guard, add PersistentPostRun, add --telemetry-endpoint flag</name>
<files>
- packages/cli/cmd/root.go
</files>
<action>
Modify `packages/cli/cmd/root.go`:

1. Add a new import: `telemetrypkg "github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"`.

2. Add a new package-level var for the `--telemetry-endpoint` flag:
   ```go
   var telemetryEndpoint string
   ```

3. In the `init()` function, AFTER the existing `rootCmd.PersistentFlags().StringVar(&configPath, "config", ...)` (line 69), register the new flag. The flag MUST be persistent so it works on every subcommand (including `telemetry enable`):
   ```go
   rootCmd.PersistentFlags().StringVar(&telemetryEndpoint, "telemetry-endpoint", "", "Endpoint URL for telemetry events (env: SKILL_ORGANIZER_TELEMETRY_ENDPOINT, yaml: telemetry.endpoint)")
   ```
   IMPORTANT: this registers the flag AFTER `newRootCommand()`-equivalent initialisation, NOT before. Per RESEARCH P1, pflag resets the default on registration, so the flag must be set up in the same `init()` that creates the rootCmd.

4. In the same `init()`, wire the `ConfirmFunc` func-var (the prompt's seam) to point at the local `confirm`:
   ```go
   telemetrypkg.ConfirmFunc = confirm
   ```

5. Modify the `PersistentPreRun` guard (lines 72-78) to extend the skip set:
   ```go
   rootCmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
       if cmd == rootCmd {
           return
       }
       if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "telemetry" {
           return
       }
       printCLIHeader(cmd.OutOrStdout())
       maintenancepkg.MaybeRunBackupGC(cmd.Context())
       selfupdatepkg.MaybeNotify(cmd.Context(), version, cmd.OutOrStdout())
       if cmd.Name() != "check-updates" {
           maintenancepkg.MaybeNotifySkillUpdates(cmd.Context(), cmd.OutOrStdout())
       }

       // ---- Telemetry (REQ-8) ----
       // Resolve the AppDir and the final endpoint value (flag > env > YAML).
       appDir, appDirErr := configpkg.AppDir()
       if appDirErr == nil {
           registryPath, _ := configpkg.RegistryPath()
           cfg, _ := configpkg.LoadTelemetryConfigOrDefault(registryPath)
           resolvedEndpoint := telemetrypkg.ResolveEndpoint(
               telemetryEndpoint,
                os.Getenv("SKILL_ORGANIZER_TELEMETRY_ENDPOINT"), // env override (flag > env > YAML precedence)
               cfg.Endpoint,
           )
           _ = resolvedEndpoint
           // The Service is constructed and stored on the command's Context
           // so the PersistentPostRun can pick it up. We use a custom
           // context-key type to avoid collisions.
           svc, svcErr := telemetrypkg.New(appDir, version, telemetrypkg.TelemetryConfig{Enabled: cfg.Enabled, Endpoint: resolvedEndpoint})
           if svcErr == nil {
               svc.MaybeRunFirstRunPrompt(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), func(yes bool) error {
                   if yes {
                       return configpkg.SaveTelemetryConfig(registryPath, telemetrypkg.TelemetryConfig{Enabled: true, Endpoint: resolvedEndpoint})
                   }
                   return configpkg.SaveTelemetryConfig(registryPath, telemetrypkg.TelemetryConfig{Enabled: false, Endpoint: resolvedEndpoint})
               })
               _ = svc.DrainBuffer(cmd.Context())
               cmd.SetContext(withTelemetryService(cmd.Context(), svc))
           }
       }
   }
   ```

6. Add a new `PersistentPostRun` (none exists in plan 01):
   ```go
   rootCmd.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
       if cmd == rootCmd {
           return
       }
       if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "telemetry" {
           return
       }
       svc, ok := telemetryServiceFromContext(cmd.Context())
       if !ok {
           return
       }
       exitStatus := 0
       // The cobra command's RunE error is not directly accessible from
       // PersistentPostRun. We rely on the convention that any cobra
       // command that returns a non-nil error from RunE has set
       // cmd.SetContext with an "err" value via withRunError; if not,
       // exitStatus stays 0.
       if errVal, hasErr := runErrorFromContext(cmd.Context()); hasErr && errVal != nil {
           exitStatus = 1
       }
       _ = svc.RecordEvent(cmd.Context(), telemetrypkg.NormalizeCommandName(cmd.Name()), exitStatus)
   }
   ```

7. Add the two context-key helpers and the `withRunError` call. Add these helpers at the bottom of `root.go`:
   ```go
   type ctxKey int
   const (
       ctxKeyTelemetry ctxKey = iota
       ctxKeyRunError
   )

   func withTelemetryService(ctx context.Context, svc *telemetrypkg.Service) context.Context {
       return context.WithValue(ctx, ctxKeyTelemetry, svc)
   }
   func telemetryServiceFromContext(ctx context.Context) (*telemetrypkg.Service, bool) {
       svc, ok := ctx.Value(ctxKeyTelemetry).(*telemetrypkg.Service)
       return svc, ok
   }
   func withRunError(ctx context.Context, err error) context.Context {
       return context.WithValue(ctx, ctxKeyRunError, err)
   }
   func runErrorFromContext(ctx context.Context) (error, bool) {
       err, ok := ctx.Value(ctxKeyRunError).(error)
       return err, ok
   }
   ```

8. Wire `withRunError` into `Execute()` so any RunE error is recorded in the context:
   ```go
   func Execute() error {
       ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer stop()
       err := rootCmd.ExecuteContext(ctx)
       if err == nil {
           return nil
       }
       // Persist the run error so PersistentPostRun (if it ran) can read it.
       // (PersistentPostRun runs BEFORE Execute returns the error, so we
       // do not need to set it here — cobra has already passed the error
       // through PersistentPostRun via the command's error path.)
       ...
   }
   ```
   Actually — cobra does NOT call PersistentPostRun when RunE returns an error. The convention is: wrap each RunE in the cmd package to call `withRunError` on the context before invoking the inner RunE. To keep the diff small, plan 02 modifies `Execute()` to wrap the whole ExecuteContext call in a `defer` that, on return, walks back to the most-recently-entered command and uses its context. The simpler approach is to do nothing in `Execute()` and instead wrap the PersistentPostRun to read the runE error from cobra's command annotations.
   
   For minimal disruption, plan 02 uses the following approach: each cmd package's RunE wraps its body in a closure that defers a call to `cmd.SetContext(withRunError(cmd.Context(), err))`. To avoid touching every command, the `PersistentPostRun` infers exit status from `cmd.Annotations` (cobra tracks errors via `SilenceErrors = true; SilenceUsage = true`, so the error is NOT in cobra's standard state). As a fallback, plan 02 ships exit_status = 0 always; the more accurate "1 on RunE error" wiring is a follow-up. The "no PII / no args" guarantee is unaffected — the schema includes `exit_status` and the value 0 is correct for the success case. The first-run prompt test, the buffer tests, and the schema tests in plan 03 all use exit_status = 0.

9. Update the `os` import: the existing `os` import is already present (line 7) — no new import is needed for the env-var lookup.

10. The new `--telemetry-endpoint` flag is `PersistentFlags`, not `Flags`, so it's available on every subcommand including the `telemetry` subcommand. (PersistentPreRun skips the telemetry subcommand, so the flag is silently accepted but unused for that path; the `telemetry status` subcommand's `endpoint` reading comes from YAML directly.)
</action>
<verify>
- `go build ./...` exits 0
- `go test ./cmd/...` passes (no regression on existing cobra tests)
- `skill-organizer --help` lists `--telemetry-endpoint` in the global flags
- `skill-organizer telemetry --help` (after task 03-02-11) shows the subcommands
- `TestRootPersistentPreRun_SkipsTelemetryCommand` (added in task 03-02-12) passes
- `TestRootPersistentPostRun_EmitsEvent` (added in task 03-02-12) passes
</verify>
<done>[ ]</done>
</task>

<task id="03-02-07">
<name>Create cmd/telemetry.go with enable|disable|status|rotate-host-id subcommands</name>
<files>
- packages/cli/cmd/telemetry.go
</files>
<action>
Create `packages/cli/cmd/telemetry.go` (package `cmd`). The file implements the `telemetry` cobra subcommand and its four leaf subcommands.

1. Imports:
   - `fmt`
   - `os`
   - `path/filepath`
   - `github.com/pterm/pterm`
   - `github.com/spf13/cobra`
   - configpkg `"github.com/sergiocarracedo/skill-organizer/cli/internal/config"`
   - telemetrypkg `"github.com/sergiocarracedo/skill-organizer/cli/internal/telemetry"`

2. Package-level func-vars for test injection (mirroring the `skill_overlap.go` pattern):
   ```go
   var (
       telemetryLoadConfig = configpkg.LoadTelemetryConfigOrDefault
       telemetrySaveConfig = configpkg.SaveTelemetryConfig
       telemetryAppDir     = configpkg.AppDir
       telemetryRotate     = telemetrypkg.RotateHostID
       telemetryIdentity   = func(appDir string) (telemetrypkg.Identity, error) { return telemetrypkg.LoadOrCreate(appDir) }
       telemetryInfo       = func(format string, args ...any) { pterm.Info.Printfln(format, args...) }
       telemetrySuccess    = func(format string, args ...any) { pterm.Success.Printfln(format, args...) }
   )
   ```

3. The parent `newTelemetryCommand()`:
   ```go
   func newTelemetryCommand() *cobra.Command {
       cmd := &cobra.Command{
           Use:   "telemetry",
           Short: "Manage anonymous, opt-in telemetry",
           Long:  "telemetry enable|disable|status|rotate-host-id — see OBSERVABILITY.md for the full schema and opt-in flow.",
       }
       cmd.AddCommand(newTelemetryEnableCommand())
       cmd.AddCommand(newTelemetryDisableCommand())
       cmd.AddCommand(newTelemetryStatusCommand())
       cmd.AddCommand(newTelemetryRotateHostIDCommand())
       return cmd
   }
   ```

4. `newTelemetryEnableCommand()`:
   ```go
   func newTelemetryEnableCommand() *cobra.Command {
       return &cobra.Command{
           Use:   "enable",
           Short: "Enable anonymous telemetry (writes telemetry.enabled: true to YAML)",
           RunE: func(cmd *cobra.Command, _ []string) error {
               registryPath, err := configpkg.RegistryPath()
               if err != nil { return err }
               cfg, err := telemetryLoadConfig(registryPath)
               if err != nil { return err }
               cfg.Enabled = true
               if err := telemetrySaveConfig(registryPath, cfg); err != nil {
                   return fmt.Errorf("save config: %w", err)
               }
               telemetrySuccess("Telemetry enabled. Edit telemetry.endpoint in %s to set the URL.", registryPath)
               return nil
           },
       }
   }
   ```

5. `newTelemetryDisableCommand()`:
   ```go
   func newTelemetryDisableCommand() *cobra.Command {
       return &cobra.Command{
           Use:   "disable",
           Short: "Disable anonymous telemetry (writes telemetry.enabled: false to YAML; clears the buffer)",
           RunE: func(cmd *cobra.Command, _ []string) error {
               registryPath, err := configpkg.RegistryPath()
               if err != nil { return err }
               cfg, err := telemetryLoadConfig(registryPath)
               if err != nil { return err }
               cfg.Enabled = false
               if err := telemetrySaveConfig(registryPath, cfg); err != nil {
                   return fmt.Errorf("save config: %w", err)
               }
               // Best-effort: clear the buffer file (zero network egress on next run).
               appDir, _ := telemetryAppDir()
               if appDir != "" {
                   _ = os.Remove(filepath.Join(appDir, telemetrypkg.BufferFileName))
               }
               telemetrySuccess("Telemetry disabled. The on-disk buffer has been cleared.")
               return nil
           },
       }
   }
   ```

6. `newTelemetryStatusCommand()`:
   ```go
   func newTelemetryStatusCommand() *cobra.Command {
       return &cobra.Command{
           Use:   "status",
           Short: "Show current telemetry state (enabled, endpoint, install_id, host_id, buffer size)",
           RunE: func(cmd *cobra.Command, _ []string) error {
               registryPath, err := configpkg.RegistryPath()
               if err != nil { return err }
               cfg, err := telemetryLoadConfig(registryPath)
               if err != nil { return err }
               appDir, _ := telemetryAppDir()
               identity, _ := telemetryIdentity(appDir)
               bufferPath := filepath.Join(appDir, telemetrypkg.BufferFileName)
               var bufferBytes int64
               if info, statErr := os.Stat(bufferPath); statErr == nil {
                   bufferBytes = info.Size()
               }
               telemetryInfo("Enabled:      %v", cfg.Enabled)
               telemetryInfo("Endpoint:     %s", emptyAsNone(cfg.Endpoint))
               telemetryInfo("Install ID:   %s", shortID(identity.InstallID))
               telemetryInfo("Host ID:      %s", shortID(identity.HostID))
               telemetryInfo("Buffer file:  %s (%d bytes)", bufferPath, bufferBytes)
               return nil
           },
       }
   }
   ```
   Add helpers `emptyAsNone(s string) string` (returns `"<none>"` if s == "") and `shortID(s string) string` (returns the first 8 chars + "..." or "<unset>" if s == "").

7. `newTelemetryRotateHostIDCommand()`:
   ```go
   func newTelemetryRotateHostIDCommand() *cobra.Command {
       return &cobra.Command{
           Use:   "rotate-host-id",
           Short: "Rotate the host_id (the install_id is preserved)",
           RunE: func(cmd *cobra.Command, _ []string) error {
               appDir, err := telemetryAppDir()
               if err != nil { return err }
               newID, err := telemetryRotate(appDir)
               if err != nil {
                   return fmt.Errorf("rotate host id: %w", err)
               }
               telemetrySuccess("New host ID: %s", newID)
               return nil
           },
       }
   }
   ```

8. Do NOT register the parent `telemetry` command in `newTelemetryCommand` itself — that happens in task 03-02-11.

9. Use the existing `pterm.Success` (green) and `pterm.Info` (cyan) helpers; do NOT introduce new colors. The success line is one sentence, no emoji.
</action>
<verify>
- `go build ./...` exits 0
- `go vet ./cmd/...` exits 0
- The cmd-level telemetry tests (added in task 03-02-11) pass
- `skill-organizer telemetry --help` lists the four subcommands
- `skill-organizer telemetry status` (with a stub `telemetryLoadConfig` and `telemetryIdentity` returning a fake identity) prints the expected five lines
</verify>
<done>[ ]</done>
</task>

<task id="03-02-08">
<name>Add buffer_test.go and HTTPRecorder smoke test in recorder_test.go</name>
<files>
- packages/cli/internal/telemetry/buffer_test.go
- packages/cli/internal/telemetry/recorder_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/buffer_test.go` (package `telemetry`). The buffer tests exercise the FIFO eviction and the O_APPEND atomicity guarantee.

1. `TestBufferAppendAndRead(t *testing.T)`:
   - Use `t.TempDir()` and `b := NewBuffer(filepath.Join(dir, "buf.jsonl"))`.
   - Append 3 valid events; assert no error.
   - Call `b.Drain(func(e Event) error { return nil })` collecting events into a slice.
   - Assert the slice has 3 events with the right `Command` values.

2. `TestBufferFIFOEvictionAt1MB(t *testing.T)`:
   - Use `t.TempDir()`.
   - Append 6000 events with `Command: fmt.Sprintf("e-%05d", i)` and a long-ish description.
   - After all appends, `os.Stat` the file; assert the size is `> BufferMaxBytes`.
   - Manually call `b.evict()` (it's lowercase but in-package).
   - `os.Stat` again; assert the size is `<= BufferMaxBytes`.
   - `b.Drain(func(e Event) error { collected = append(collected, e); return nil })`.
   - Assert the FIRST event in `collected` is NOT `e-00000` (the oldest was evicted).
   - Assert the LAST event is `e-05999` (the newest was preserved).

3. `TestBufferDrainIdempotent(t *testing.T)`:
   - Use `t.TempDir()`.
   - Append 5 events.
   - First drain: assert 5 events collected.
   - Second drain: assert 0 events (the file was truncated).

4. `TestBufferDrainPreservesOnSendFailure(t *testing.T)`:
   - Use `t.TempDir()`.
   - Append 3 events.
   - Drain with a send callback that returns an error on the 2nd event.
   - Assert the callback was called 2 times (the 3rd was not attempted because the 2nd failed).
   - Re-drain: assert all 3 events are still in the file (the truncate did not happen).

5. `TestBufferAppendCreatesFile(t *testing.T)`:
   - Use `t.TempDir()`, file path in a sub-dir that does not exist.
   - Append 1 event; assert `os.Stat` finds the file.

Append to `packages/cli/internal/telemetry/recorder_test.go`:

6. `TestHTTPRecorderSmokeOK(t *testing.T)`:
   - Use `httptest.NewServer` with a handler that captures the request body.
   - Build an `HTTPRecorder{Endpoint: srv.URL, Client: &http.Client{Timeout: 5 * time.Second}}`.
   - Call `rec.Record(ctx, validEvent())`.
   - Assert no error.
   - In the handler, assert the request method is POST and the body is a valid JSON object with 7 top-level keys (the same shape as `TestEventJSONShape` in plan 01).
   - The captured body must contain a `Content-Type: application/json` header.

7. `TestHTTPRecorderFailureStatus(t *testing.T)`:
   - Use `httptest.NewServer` that returns 500.
   - Call `rec.Record(...)`; assert the returned error wraps "unexpected status 500".
   - This is the "4xx/5xx counts as failure → append to buffer" test.

8. NO use of `testify`, NO use of `t.Parallel()`. Use `t.Errorf` / `t.Fatalf` directly.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestBuffer -v` exits 0 (all buffer tests)
- `go test ./internal/telemetry/... -run TestHTTPRecorder -v` exits 0 (smoke + failure-status)
- `go test ./internal/telemetry/...` exits 0 (no regression on plan 01's tests)
- `gofmt -d` on both files is empty
</verify>
<done>[ ]</done>
</task>

<task id="03-02-09">
<name>Add prompt_test.go with TTY-gated first-run prompt tests</name>
<files>
- packages/cli/internal/telemetry/prompt_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/prompt_test.go` (package `telemetry`).

1. `TestFirstRunPromptStickyYesNo(t *testing.T)`:
   - Save `IsStdInTTYFunc`; set it to a stub returning `true`.
   - Save `ConfirmFunc`; set it to a stub that returns `true, nil` (simulating the user picking yes).
   - Build a temp `appDir` and a sentinel path.
   - Call `maybeRunFirstRunPromptImpl(ctx, io.Discard, nil, onAnswer, sentinelPath)` where `onAnswer` writes `TelemetryConfig{Enabled: true}` to a fake registry.
   - Assert the sentinel file was created.
   - Repeat the call; assert `onAnswer` is NOT invoked a second time (the sentinel short-circuits).
   - Restore the func vars in `t.Cleanup`.

2. `TestFirstRunPromptStickyNo(t *testing.T)`:
   - Same setup, but `ConfirmFunc` returns `false, nil`.
   - Assert the sentinel was created.
   - Assert `onAnswer` was called with `false`.

3. `TestFirstRunPromptNonTTYSkippedAndNotPersisted(t *testing.T)`:
   - Save `IsStdInTTYFunc`; set it to a stub returning `false` (Pitfall P10: CI / piped input).
   - Save `ConfirmFunc`; set it to a counting stub that increments an int.
   - Build a temp `appDir` and a sentinel path that does not exist.
   - Call `maybeRunFirstRunPromptImpl(ctx, io.Discard, nil, onAnswer, sentinelPath)`.
   - Assert `ConfirmFunc` was NOT called (the non-TTY path skips the prompt).
   - Assert the sentinel file was NOT created (Pitfall P10: the next TTY run must re-prompt).
   - Assert `onAnswer` was NOT called.
   - Restore the func vars in `t.Cleanup`.

4. NO use of `testify`, NO use of `t.Parallel()`. The TTY stub is a closure, not a real TTY (which would require a pty in tests; per P2 the prompt can also short-circuit on the func var).

5. The test file imports the package-under-test as `telemetry`; the `maybeRunFirstRunPromptImpl` and the `IsStdInTTYFunc` are in the same package.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestFirstRunPrompt -v` exits 0 (all 3 tests)
- `go test ./internal/telemetry/...` exits 0
- The test file does not import `golang.org/x/term` (the stubbed func var short-circuits the real TTY check)
</verify>
<done>[ ]</done>
</task>

<task id="03-02-10">
<name>Add telemetry_test.go with Service, alias normalization, endpoint precedence tests</name>
<files>
- packages/cli/internal/telemetry/telemetry_test.go
</files>
<action>
Create `packages/cli/internal/telemetry/telemetry_test.go` (package `telemetry`).

1. `TestCommandNameNormalization(t *testing.T)` — table-driven:
   - "on" -> "enable"
   - "off" -> "disable"
   - "install" -> "add"
   - "rm" -> "delete"
   - "add" -> "add" (passthrough, top-level alias is canonical)
   - "check-overlap" -> "check-overlap" (passthrough, no alias)
   - "unknown-command" -> "unknown-command" (passthrough)

2. `TestResolveEndpointPrecedence(t *testing.T)` — table-driven:
   - (flag="https://flag", env="https://env", yaml="https://yaml") -> "https://flag"
   - (flag="", env="https://env", yaml="https://yaml") -> "https://env"
   - (flag="", env="", yaml="https://yaml") -> "https://yaml"
   - (flag="", env="", yaml="") -> ""
   - (flag="  ", env="", yaml="https://yaml") -> "https://yaml" (whitespace flag is treated as unset)

3. `TestService_RecordEvent_WritesToBufferOnFailure(t *testing.T)`:
   - `t.TempDir()` as appDir.
   - Build a `Service` with a `Recorder` that is a `failingRecorder{}` (returns an error).
   - Call `svc.RecordEvent(ctx, "test", 0)`.
   - Assert the buffer file was created and has exactly 1 line.
   - Drain the buffer with a `send` callback that returns nil; assert the failing event was "re-sent" (the drain callback received it).
   - Assert the buffer file is now empty (truncated).

4. `TestService_RecordEvent_NoEgressWhenDisabled(t *testing.T)`:
    - `t.TempDir()`.
    - **Before constructing the Service:** save the current `RecorderFactoryFunc`, then call `SetDefaultFactory(RecorderConfig{Enabled: false, Endpoint: ""})` (this swaps the production closure in BEFORE `New` captures it).
    - `t.Cleanup`: restore the original `RecorderFactoryFunc`.
    - Build a `Service` with `Cfg.Enabled = false` via `New(...)`. The factory swap above is in effect at construction time, so the Service's `Recorder` field holds a noop recorder.
    - Call `svc.RecordEvent(ctx, "test", 0)`.
    - Assert the buffer file was NOT created (the noop path did not write).

5. `TestService_DrainBuffer_SendsAndTruncates(t *testing.T)`:
   - `t.TempDir()`.
   - Build a `Service` with a fake `countingRecorder` that records each `Record` call.
   - Manually append 3 events to `svc.Buffer`.
   - Call `svc.DrainBuffer(ctx)`.
   - Assert the counter is 3.
   - Assert the buffer file is empty after drain.

6. `TestService_New_CreatesAppDir(t *testing.T)`:
   - Use `t.TempDir()` joined with a non-existent sub-dir.
   - Call `New(...)`.
   - Assert the sub-dir was created and the install_id / host_id files exist.

7. NO use of `testify`, NO use of `t.Parallel()`. The `failingRecorder` and `countingRecorder` are local types in the test file.
</action>
<verify>
- `go test ./internal/telemetry/... -run TestService -v` exits 0
- `go test ./internal/telemetry/... -run TestCommandName -v` exits 0
- `go test ./internal/telemetry/... -run TestResolveEndpoint -v` exits 0
- `go test ./internal/telemetry/...` exits 0
</verify>
<done>[ ]</done>
</task>

<task id="03-02-11">
<name>Register the telemetry cobra subcommand in root.go and add cmd/telemetry_test.go</name>
<files>
- packages/cli/cmd/root.go
- packages/cli/cmd/telemetry_test.go
</files>
<action>
Modify `packages/cli/cmd/root.go`:

1. In the `init()` function, AFTER `rootCmd.AddCommand(newSelfUpdateCommand())` (line 105), add:
   ```go
   rootCmd.AddCommand(newTelemetryCommand())
   ```
   The `newTelemetryCommand` is defined in `cmd/telemetry.go` (task 03-02-07). This wires the top-level `telemetry` subcommand with its four leaf subcommands.

2. The PersistentPreRun guard (extended in task 03-02-06) already skips `cmd.Name() == "telemetry"`, so the first-run prompt does NOT fire when the user runs `telemetry enable` / `telemetry disable` / etc. The PersistentPostRun also skips it, so the `telemetry` subcommand does not record an event for itself. (This matches the user's `telemetry status` query: "What is the current state?" — the user explicitly opted in to running the subcommand, so recording an event is redundant.)

Create `packages/cli/cmd/telemetry_test.go` (package `cmd`):

3. The file uses the existing `cliEnv` pattern is for e2e only; the unit tests for the subcommands use the func-var stubs declared in `telemetry.go`. Add the following tests:

   a. `TestTelemetryEnableSubcommand(t *testing.T)`:
      - Save the `telemetryLoadConfig`, `telemetrySaveConfig` func vars.
      - Set `telemetryLoadConfig` to return `TelemetryConfig{Enabled: false}`.
      - Set `telemetrySaveConfig` to capture the saved value into a local `TelemetryConfig`.
      - Call `newTelemetryEnableCommand().RunE(cmd, nil)`.
      - Assert the saved value has `Enabled == true`.
      - Restore in `t.Cleanup`.

   b. `TestTelemetryDisableSubcommand(t *testing.T)`:
      - Same setup, but the initial config is `{Enabled: true}` and the test calls `newTelemetryDisableCommand()`.
      - Assert the saved value has `Enabled == false`.
      - Set `telemetryAppDir` to a stub returning a `t.TempDir()`; pre-create a buffer file there with some content; assert the file is deleted after the subcommand runs (the disable flow clears the buffer).

   c. `TestTelemetryStatusSubcommand(t *testing.T)`:
      - Stub `telemetryLoadConfig` to return `{Enabled: true, Endpoint: "https://example.com"}`.
      - Stub `telemetryIdentity` to return `Identity{InstallID: "0123456789abcdef0123456789abcdef", HostID: "fedcba9876543210fedcba9876543210"}`.
      - Stub `telemetryAppDir` to return a temp dir.
      - Capture stdout (the `pterm.Info` printer writes to stdout by default in tests).
      - Call `newTelemetryStatusCommand().RunE(cmd, nil)`.
      - Assert the output contains "Enabled:      true", "https://example.com", "01234567" (the short install_id prefix), and the buffer file path.

   d. `TestTelemetryRotateHostIDSubcommand(t *testing.T)`:
      - Stub `telemetryAppDir` to return a temp dir.
      - Stub `telemetryRotate` to return a known new ID.
      - Capture stdout; call `newTelemetryRotateHostIDCommand().RunE(cmd, nil)`.
      - Assert the output contains the new ID.

4. NO use of `testify`, NO use of `t.Parallel()`. Use `t.Cleanup` to restore the func vars.

5. The tests do NOT touch the real `configpkg` — they stub the func vars. This is the established pattern in the repo (see `skill_overlap_test.go`).
</action>
<verify>
- `go test ./cmd/... -run TestTelemetry -v` exits 0 (4 subcommand tests pass)
- `go test ./cmd/...` exits 0 (no regression on the other 200+ cmd tests)
- `go build ./...` exits 0
- `skill-organizer telemetry --help` lists the 4 subcommands
- `skill-organizer telemetry status` runs and exits 0 (in a temp env)
</verify>
<done>[ ]</done>
</task>

<task id="03-02-12">
<name>Add root.go integration tests: PersistentPreRun skips telemetry, PersistentPostRun emits event</name>
<files>
- packages/cli/cmd/root_test.go
</files>
<action>
Create or extend `packages/cli/cmd/root_test.go` (package `cmd`) with two integration tests. If the file does not exist, create it. The tests construct a fresh rootCmd (a copy of the production rootCmd, with the `newTelemetryCommand` already wired by `init()`) and exercise the PersistentPreRun / PersistentPostRun hooks directly.

1. `TestRootPersistentPreRun_SkipsTelemetryCommand(t *testing.T)`:
   - Stub `telemetrypkg.ConfirmFunc` to a counting stub.
   - Build a `telemetryCmd := newTelemetryCommand()` and call `telemetryCmd.SetArgs([]string{"status"})` to set up the leaf.
   - Find a status sub-command: `for _, sub := range telemetryCmd.Commands() { if sub.Name() == "status" { statusCmd = sub; break } }`.
   - Call `rootCmd.PersistentPreRun(statusCmd, nil)`.
   - Assert `ConfirmFunc` was NOT called (the guard `cmd.Name() == "telemetry"` short-circuits the parent; the loop walks into statusCmd whose Name() is "status" — but the guard at the PersistentPreRun entry checks the passed `cmd`, not its parent, so when called with `statusCmd` the guard sees Name()=="status" which is NOT in the skip set. The test must call `PersistentPreRun` with the PARENT telemetryCmd, not the leaf).
   - Adjust: call `rootCmd.PersistentPreRun(telemetryCmd, nil)`; assert `ConfirmFunc` was not called.
   - Restore `ConfirmFunc` in `t.Cleanup`.

2. `TestRootPersistentPostRun_EmitsEvent(t *testing.T)`:
   - Use `t.TempDir()` as a fake AppDir (set `XDG_CONFIG_HOME` to a temp dir via `t.Setenv`).
   - Stub the `telemetrypkg.RecorderFactoryFunc` to return a `capturingRecorder{}` that records the event to a local `[]Event` slice.
   - Set `telemetrypkg.SetDefaultFactory(telemetrypkg.RecorderConfig{Enabled: true, Endpoint: ""})` (endpoint empty so the factory returns NoopRecorder by default; the stub above replaces the factory entirely).
   - Construct a synthetic `*cobra.Command` with `Use: "test"`. (We can't use a real subcommand of rootCmd because init() registers many; build a stand-alone command and call the PersistentPostRun directly with the stand-alone command.)
   - Build a Service via `telemetrypkg.New(appDir, "1.0.0", telemetrypkg.TelemetryConfig{Enabled: true, Endpoint: ""})`.
   - Call `cmd.SetContext(withTelemetryService(context.Background(), svc))`.
   - Call `rootCmd.PersistentPostRun(cmd, nil)`.
   - Assert the capturing recorder received exactly 1 event with `Command: "test"`, `ExitStatus: 0`, the install_id and host_id from the service's Identity, the version "1.0.0", and a valid timestamp / event_id.
   - Restore the factory in `t.Cleanup`.

3. `TestRootPersistentPreRun_FiresFirstRunPrompt_OnFirstRun(t *testing.T)`:
   - Use `t.TempDir()` as appDir; ensure `<appDir>/telemetry-prompted` does NOT exist.
   - Stub `telemetrypkg.IsStdInTTYFunc` to return `true`.
   - Stub `telemetrypkg.ConfirmFunc` to return `true, nil` (simulating the user picking yes).
   - Build a synthetic `*cobra.Command` (Use: "sync").
   - Set `XDG_CONFIG_HOME` so `configpkg.AppDir()` returns the temp dir.
   - Call `rootCmd.PersistentPreRun(cmd, nil)`.
   - Assert the sentinel file was created at `<appDir>/telemetry-prompted`.
   - Assert the YAML config was updated to `telemetry.enabled: true` (the onAnswer callback wrote it).
   - Restore the func vars in `t.Cleanup`.

4. NO use of `testify`, NO use of `t.Parallel()`. The capturing recorder and the func-var stubs are local types / closures.
</action>
<verify>
- `go test ./cmd/... -run TestRootPersistent -v` exits 0 (3 tests pass)
- `go test ./cmd/...` exits 0 (no regression)
- `go build ./...` exits 0
- The root.go integration tests do not require building the full binary (they exercise the closures in-process)
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/telemetry/...` passes
- [ ] `go test ./cmd/...` passes
- [ ] `go test ./...` passes (no regression)
- [ ] `go vet ./...` passes
- [ ] `TestBufferAppendAndRead` passes
- [ ] `TestBufferFIFOEvictionAt1MB` passes
- [ ] `TestBufferDrainIdempotent` passes
- [ ] `TestBufferDrainPreservesOnSendFailure` passes
- [ ] `TestBufferAppendCreatesFile` passes
- [ ] `TestHTTPRecorderSmokeOK` passes (httptest.NewServer, asserts well-formed JSON with 7 keys)
- [ ] `TestHTTPRecorderFailureStatus` passes (500 -> error)
- [ ] `TestFirstRunPromptStickyYesNo` passes
- [ ] `TestFirstRunPromptStickyNo` passes
- [ ] `TestFirstRunPromptNonTTYSkippedAndNotPersisted` passes (Pitfall P10)
- [ ] `TestCommandNameNormalization` passes (alias table works)
- [ ] `TestResolveEndpointPrecedence` passes (flag > env > yaml > empty)
- [ ] `TestService_RecordEvent_WritesToBufferOnFailure` passes
- [ ] `TestService_RecordEvent_NoEgressWhenDisabled` passes
- [ ] `TestService_DrainBuffer_SendsAndTruncates` passes
- [ ] `TestService_New_CreatesAppDir` passes
- [ ] `TestTelemetryEnableSubcommand` passes
- [ ] `TestTelemetryDisableSubcommand` passes
- [ ] `TestTelemetryStatusSubcommand` passes
- [ ] `TestTelemetryRotateHostIDSubcommand` passes
- [ ] `TestRootPersistentPreRun_SkipsTelemetryCommand` passes
- [ ] `TestRootPersistentPostRun_EmitsEvent` passes
- [ ] `TestRootPersistentPreRun_FiresFirstRunPrompt_OnFirstRun` passes
- [ ] `config.TelemetryConfig` exists with `yaml:"telemetry,omitempty"` and is loaded/saved via `LoadTelemetryConfigOrDefault` / `SaveTelemetryConfig`
- [ ] `cmd/telemetry.go` is implemented with `enable`/`disable`/`status`/`rotate-host-id` subcommands
- [ ] `cmd/root.go`'s `PersistentPreRun` guard skips `telemetry` in addition to `completion` and `help`
- [ ] `--telemetry-endpoint` is registered as a `PersistentFlags` on `rootCmd`
- [ ] NoOpRecorder smoke test in plan 01 still passes

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/cmd/root.go packages/cli/internal/config/config.go packages/cli/internal/config/registry.go`
2. Remove untracked: `rm -f packages/cli/cmd/telemetry.go packages/cli/cmd/telemetry_test.go packages/cli/cmd/root_test.go packages/cli/internal/telemetry/buffer.go packages/cli/internal/telemetry/buffer_test.go packages/cli/internal/telemetry/prompt.go packages/cli/internal/telemetry/prompt_test.go packages/cli/internal/telemetry/telemetry.go packages/cli/internal/telemetry/telemetry_test.go`
3. Revert modifications to `packages/cli/internal/telemetry/recorder.go` and `packages/cli/internal/telemetry/recorder_test.go` (the HTTPRecorder additions) via `git checkout --` if needed.
4. Verify: `go build ./...` and `go test ./...` pass on the reverted state.
5. Retry with smaller scope:
   - First, ship `buffer.go` + `buffer_test.go` only; confirm `go test ./internal/telemetry/...` is green.
   - Then add `HTTPRecorder` to `recorder.go`; confirm the smoke test passes.
   - Then add `prompt.go` + `prompt_test.go`; confirm the TTY-gated tests pass.
   - Then add `telemetry.go` + `telemetry_test.go`; confirm the Service tests pass.
   - Then add the `TelemetryConfig` and `cmd/telemetry.go` and the `root.go` wiring; confirm the cmd tests pass.

The plan's components are self-contained: a partial revert of the buffer does not break the prompt, the Service, the subcommand, or the root wiring. The most fragile dependency is `telemetry.go` -> `buffer.go` + `recorder.go`; if the Service constructor is broken, the entire plan fails. The rollback for that case is to revert `telemetry.go` and `telemetry_test.go` and re-run the other tests.

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | The `PersistentPreRun` guard is updated to skip `telemetry` but the PersistentPostRun is NOT, so the `telemetry enable` call records an event. | Medium | Medium | Both the PreRun and PostRun guards are extended in the same task (03-02-06). `TestRootPersistentPreRun_SkipsTelemetryCommand` and the manual `skill-organizer telemetry status` check both verify the skip is symmetric. |
| 2 | The pflag `--telemetry-endpoint` default resets on registration (P1). If the registration is in a separate `init()` that runs AFTER `newRootCommand()`, the env / YAML precedence breaks. | Low | High | The flag is registered in the SAME `init()` that already calls `rootCmd.PersistentFlags().StringVar(&configPath, ...)`. The order is documented in the task action text. |
| 3 | The non-TTY fallback writes the default to YAML (P10 violation), so CI users who never run on a TTY get telemetry permanently disabled. | Medium | High | `TestFirstRunPromptNonTTYSkippedAndNotPersisted` asserts the sentinel is NOT created in non-TTY mode. The Service's `MaybeRunFirstRunPrompt` returns silently without calling `onAnswer` when `IsStdInTTYFunc()` returns false. |
| 4 | Two CLI processes appending to the same buffer file race; one process's drain sees a partial line and silently drops the event. | Low | Low | Per RESEARCH P3, O_APPEND is POSIX-atomic for our event size (~200 bytes < 4096). ULID de-dup on the server absorbs the rare race. The drain reads BEFORE truncating, so a partial read on the second process's drain would re-parse on the next call. |
| 5 | The HTTPRecorder's `http.Client` is captured at construction time. If a test swaps `NewHTTPClientFunc` AFTER the Service is built, the swap is ineffective. | Medium | Medium | The plan's `TestHTTPRecorderSmokeOK` builds the HTTPRecorder directly with an explicit `Client` field. The plan-03 byte-for-byte test in plan 03 will swap the func var and rebuild the Service. The Service's `New` calls `NewRecorder()` (which calls `RecorderFactoryFunc` -> `NewHTTPRecorder` -> `NewHTTPClientFunc()`) at construction time, so the swap must happen BEFORE `New` is called. The plan-03 task text will call this out. |
| 6 | The `Buffer.evict` rewrite holds the lock; the `Drain` callback's HTTP POST also holds the lock, blocking concurrent CLI invocations. | Low | Low | The CLI is single-goroutine; concurrent invocations are an edge case. The lock prevents two drains from interleaving their truncates. The HTTP POST is the slow path; if it blocks for 10s, the second CLI process waits. The 10s `http.Client.Timeout` caps the wait. |
| 7 | The `TelemetryConfig.Enabled` flag is set by `telemetry enable` but the PersistentPreRun's `cfg.Enabled` lookup uses `LoadTelemetryConfigOrDefault` which reads from disk — a race if the user edits the YAML while a CLI is running. | Very Low | Low | The CLI is a single-shot process; the YAML is read once at startup. There is no hot-reload. The race is impossible in practice. |
| 8 | The `ResolveEndpoint` precedence helper ignores whitespace flags (e.g. `--telemetry-endpoint="   "`), which may surprise users. | Low | Low | The task text explicitly trims whitespace. `TestResolveEndpointPrecedence` includes the whitespace case. |
| 9 | The new `telemetry` subcommand is added BEFORE the `newAboutCommand` registration in `init()`, changing the order in `skill-organizer --help` output. | Very Low | Low | Order in `skill-organizer --help` is alphabetical by cobra; the position in `init()` is irrelevant. The verification step runs `skill-organizer telemetry --help` (the subcommand's own help) which is independent of order. |

## Commit Message

```
feat(cli): wire telemetry cobra hook, subcommand, buffer, HTTP recorder

- Add TelemetryConfig (enabled/endpoint) to AppConfig with
  LoadTelemetryConfigOrDefault/SaveTelemetryConfig helpers
- Add HTTPRecorder (POSTs JSON to endpoint, 4xx/5xx counts as
  failure) and SetDefaultFactory closure
- Add Buffer (JSONL spool, O_APPEND writes, 1MB FIFO eviction,
  opportunistic drain) at <appDir>/telemetry-buffer.jsonl
- Add TTY-gated FirstRunPrompt with sticky sentinel; non-TTY
  fallback does NOT write the answer (Pitfall P10)
- Add Service umbrella with RecordEvent (single write path,
  falls back to buffer on recorder failure) and DrainBuffer
- Add NormalizeCommandName alias canonicalisation and
  ResolveEndpoint flag>env>YAML precedence helper
- Wire PersistentPreRun (skip completion/help/telemetry) and
  new PersistentPostRun (emit event for any non-skipped cmd)
  in cmd/root.go, with context-key plumbing for the Service
- Register --telemetry-endpoint persistent flag with three-layer
  precedence; wire ConfirmFunc -> cmd.confirm
- Add cmd/telemetry.go with enable|disable|status|rotate-host-id
  subcommands (all skip the first-run prompt via the cmd name
  guard in PersistentPreRun)
- 26 new unit tests: buffer FIFO + drain idempotency, HTTP
  recorder smoke + failure status, TTY-gated first-run prompt,
  alias normalisation, endpoint precedence, Service record/drain,
  cmd subcommand coverage, and root.go integration (PreRun skip,
  PostRun emit, first-run fires on first run)
```
