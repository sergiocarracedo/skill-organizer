# Phase 3: Observability (REQ-8) — Research

**Researched:** 2026-06-11
**Phase goal:** Opt-in, anonymous telemetry that records command invocations
without args / paths / PII. Disabled by default. Documented schema and endpoint.

**Scope of this research:** what the planner needs to know to break REQ-8
into small, shippable plans. We assume the decisions captured in
`.planning/phases/03-observability/03-CONTEXT.md` are locked; the
planner should treat this doc as the *implementation playbook* for
those decisions, not a re-litigation of them.

Confidence levels (HIGH / MEDIUM / LOW) reflect how much of the
recommendation is verified against the existing codebase and current
external docs versus extrapolation.

---

## Don't Hand-Roll

| Problem | Recommended solution | Why | Confidence |
|---|---|---|---|
| ULID generation (de-dup ID for the event) | `github.com/oklog/ulid/v2` (v2.1.1, May 2025, Apache-2.0, 5k stars) | De-facto Go ULID library; monotonic entropy + 80 bits of entropy fits the "no PII" guarantee; we don't need to re-derive the spec. [VERIFIED: https://github.com/oklog/ulid] | HIGH |
| Random bytes for `install_id` / `host_id` | `crypto/rand.Read(buf[:16])` then `encoding/hex.EncodeToString` | Stdlib `Reader` is a global, concurrent-safe CSPRNG; `Read` "never returns an error" outside legacy Linux. v2.1.1 docs explicitly recommend `crypto/rand` for security-sensitive use cases. [VERIFIED: https://pkg.go.dev/crypto/rand] | HIGH |
| HTTP POST in `HTTPRecorder` | Stdlib `net/http` with an injected `*http.Client` (package-level `NewHTTPClientFunc` var) | The default `http.Client` works, but we need a test-injection seam for the counting-transport zero-egress assertion. Stdlib covers the only feature we need (POST + `Content-Type: application/json`). | HIGH |
| JSON serialization for the schema | Stdlib `encoding/json` with an explicit `struct` + `json:"snake_case"` tags on every field | The byte-for-byte schema test in CONTEXT requires a deterministic field order. `encoding/json` marshals struct fields in declaration order, which we control. Don't use `map[string]any` — the server would see a random key order. | HIGH |
| Counting HTTP transport for the zero-egress test | `http.RoundTripper` wrapper that increments an atomic counter; swap in via `NewHTTPClientFunc` for the duration of the test | This is the standard Go pattern for asserting "no network calls" without relying on `httptest`. See [VERIFIED: `crypto/rand` "safe for concurrent use"] for the concurrent-safety precedent — the counter must be `sync/atomic` or a mutex. | HIGH |
| First-run prompt | Reuse `confirm(prompt, defaultValue bool)` from `cmd/prompt.go:127` (which uses `pterm.DefaultInteractiveConfirm`) | It already supports the "default = off" UX (`WithDefaultValue(false)`); the prompt is skippable by pressing Enter; it integrates with the existing pterm style. | HIGH |
| JSONL spool file | Plain `os.OpenFile(path, O_APPEND\|O_CREATE\|O_WRONLY, 0o644)` + a `ReadDir`-style scan for the cap-eviction pass | We don't need an external library: a JSONL file is `os.WriteFile` per line, and the 1 MB cap check is a one-time `os.Stat` + read-all + rewrite. Use a single goroutine (the CLI) — no concurrent writers. | MEDIUM |
| Hex encoding of `install_id` / `host_id` | `encoding/hex.EncodeToString` | Stdlib; matches the CONTEXT decision ("32 hex chars from 16 random bytes"). Don't roll your own base16. | HIGH |
| Time formatting | `time.Now().UTC().Format(time.RFC3339)` | Stdlib, produces the exact format the CONTEXT spec asks for. | HIGH |
| Three-layer config precedence (flag > env > YAML) | Existing `configpkg.AppConfig` for YAML + `pflag.Flag` lookup + `os.Getenv` | The repo already uses cobra/pflag (pflag is a transitive dep of cobra) and `os.Getenv`; no new dependency needed. | HIGH |

**Avoid** these tempting hand-rolls:

- **Don't write your own UUID/ULID.** ULID is a 130-line spec; `oklog/ulid`
  has 130 commits and unit tests for monotonicity, monotonic within a
  millisecond, binary round-trips, and the entropy injection. We'd
  re-discover those bugs.
- **Don't use `math/rand` for `install_id` / `host_id`.** The docs explicitly
  warn: "Security-sensitive use cases should always use cryptographically
  secure entropy provided by `crypto/rand`." [VERIFIED:
  https://github.com/oklog/ulid README]
- **Don't use `time.Now().UnixNano()` for `event_id` and assume uniqueness.**
  Two invocations in the same millisecond would collide. ULID handles this
  with 80 bits of entropy.
- **Don't depend on `golang.org/x/exp/rand` for the IDs.** It's a deprecated
  module path; oklog's docs mention it but the current recommendation is
  `crypto/rand`.
- **Don't bring in a logging library just for the HTTP body.** Stdlib
  `encoding/json` produces deterministic output for a fixed struct.

---

## Common Pitfalls

### P1. pflag / env / YAML precedence order

**What goes wrong:** The CONTEXT specifies precedence: `flag > env > YAML`.
If we just read the three sources and apply them in the order they're
"noticed" (pflag during cobra parsing, env during `os.Getenv`, YAML
during `LoadAppConfig`), the result depends on init order. Specifically:
cobra flag default values are set *before* `PersistentPreRun` runs, so
if we read env after pflag, env silently overrides an explicit
`--telemetry-endpoint=...` flag.

**How to avoid:** Resolve the final value in this exact order, in this
exact function, after `cobra.ExecuteContext` has parsed flags but
before the recorder is built:

1. Read YAML → `cfg.Telemetry.Endpoint` (default "").
2. Read env `SKILL_ORGANIZER_TELEMETRY_ENDPOINT` → if non-empty, override.
3. Read `cmd.Flags().Lookup("--telemetry-endpoint").Value.String()` → if
   non-empty AND the user actually set it (cobra tracks this via
   `Changed`), override.
4. If the final value is empty, return `NoopRecorder` *regardless* of
   `cfg.Telemetry.Enabled` (CONTEXT: "If none is set, the factory
   returns NoopRecorder regardless of the enabled flag").

The "did the user actually set it" check matters: cobra's
`Lookup().Value.String()` returns the default if the flag was not
passed. We need a separate `Changed` check (or use `cmd.Flags().Changed`)
to distinguish "user passed empty string" from "user did not pass the
flag at all". Treat the former as a deliberate override (force the
no-op recorder by setting endpoint to empty).

### P2. First-run prompt fires on `help` / `version` / `completion` and breaks scripts

**What goes wrong:** The CONTEXT says "the very first invocation of the
binary (any subcommand, or the bare `skill-organizer`) shows the
opt-in prompt before the command runs." If the user types
`skill-organizer --version` or `skill-organizer completion bash | sh`,
they will get an interactive prompt that hangs the script.

**How to avoid:** In `PersistentPreRun`, gate the prompt on:

- `cmd != rootCmd` is *not* a reason to skip — CONTEXT says "any
  subcommand" counts. But:
- `cmd.Name() == "completion" || cmd.Name() == "help"` — skip (the
  existing code in `root.go:76` already does this for the backup GC and
  self-update notifier — reuse that gate).
- `cmd.Name() == "telemetry"` — skip (the telemetry subcommand manages
  its own state; we don't want the prompt to fire when the user is
  trying to *disable* telemetry).
- Non-TTY stdin — skip (CI / piped input). Use `isatty(os.Stdin)` via
  `golang.org/x/term` (already an indirect dep through cobra) or check
  for `os.Getenv("CI") != ""` as a fallback. When we skip the prompt,
  treat it as the "no" default and write the sticky answer so we never
  ask again. The CONTEXT says "default = off" — non-TTY users
  automatically get the default.

This is the same gate the existing `PersistentPreRun` already uses for
backup GC and self-update notifier (`root.go:72-85`), so the
"PersistentPreRun skips `completion` and `help`" precedent is
established — extend it, don't reinvent.

### P3. Buffer race condition between concurrent invocations

**What goes wrong:** A user (or a CI script) could run two
`skill-organizer` processes in parallel via `&`. The second process
opens the JSONL buffer, the first process appends, and the second
process's drain reads a partial line. JSON parse fails; we lose that
event.

**How to avoid:** Three layered mitigations, in order of preference:

1. **Best (use this):** open the buffer with `O_APPEND` for writes —
   POSIX guarantees atomicity for writes up to `PIPE_BUF` (4096 bytes).
   A single event JSON is ~200 bytes; we're 20× under the limit. This
   handles the interleave-on-write case. The "read after write" race
   in the drain step is a different problem: solve it by reading
   the file *before* writing, not by file locking (see below).
2. **Drain ordering:** `drainBuffer()` reads, parses, and POSTs each
   event; only after the read-and-parse succeeds do we truncate the
   file. If two processes race on the drain, both POST the same event
   — the server de-duplicates by `event_id` (ULID), so duplicate
   delivery is fine. Don't truncate-while-the-other-process-is-reading;
   truncate by overwriting the file with the remaining un-sent events
   *after* a successful parse, and accept the rare lost-event on
   crash mid-truncate.
3. **No file locking.** `flock(2)` / `LOCK_EX` would prevent the race
   entirely, but the cost is wrong for an interactive CLI that may run
   in containers without `/var/lock` or where the AppDir is
   read-only. The CONTEXT decision ("no retry timers, no daemon, the
   drain is opportunistic") implies we accept the rare race; the
   ULID de-dup absorbs the cost.

**Test coverage:** A race-condition test is not in scope (CI doesn't
exercise parallel CLI runs). The "drain" test asserts the order:
read → POST → truncate.

### P4. `crypto/rand.Read` panics on broken systems

**What goes wrong:** Per the stdlib docs, `crypto/rand.Read` "crashes
the program irrecoverably" on legacy Linux that returns an error from
`getrandom(2)`. [VERIFIED: https://pkg.go.dev/crypto/rand]

**How to avoid:** This is acceptable behavior for an ID-generation
function — if the OS can't supply random bytes, the system is broken
in a way that affects TLS, file permissions, etc. Don't wrap `Read` in
a `recover()`; the panic is correct. Document this in a comment at
the call site: "A panic here indicates the OS RNG is broken; the
system is unsafe for any crypto operation."

### P5. PersistentPreRun is not `E`-variant; can't return errors

**What goes wrong:** The existing `PersistentPreRun` in
`root.go:72-85` does not return an error. If the first-run prompt
fails (e.g., pterm panics, stdin is closed mid-read), the error is
swallowed and the command continues with the default state. The user
gets no feedback that the prompt didn't actually take.

**How to avoid:** Three options, pick (a):

(a) **Best:** Mirror the `Maintenance.MaybeNotifySkillUpdates` pattern
(`packages/cli/internal/maintenance/maintenance.go:14-49`): the
function is `func(ctx, stdout)`, returns nothing, and any error is
silently dropped. This matches the existing style. The first-run
prompt's failure mode is "user pressed Ctrl-C" or "stdin closed" —
both are user-initiated, no error message needed.

(b) Refactor `PersistentPreRun` to `PersistentPreRunE` and return
errors. Touches every command's pre-run path; out of scope for REQ-8.

(c) Wrap the prompt in a goroutine and bubble the error to a
`defer recover()`. Over-engineered.

Use (a). Add a one-line comment: "Errors are intentionally swallowed
to match the existing `MaybeNotify*` precedent."

### P6. Cobra `cmd.Name()` for aliased subcommands returns the alias, not the canonical name

**What goes wrong:** `root.go:107-115` registers top-level aliases for
`add`, `delete`, `enable`, `disable`, `check-updates`. The schema wants
the canonical name (e.g., `enable`, not whatever the alias resolves
to). `cmd.Name()` returns the alias as the user typed it, not the
canonical target.

**How to avoid:** Walk up to the parent group when emitting the
event: if `cmd.Name()` is a known alias, use the parent group's
canonical name. The simplest fix is to use the parent
`Skill` group's `Name()` for subcommands of `skill`. Concretely:
`eventCommand = cmd.Name()` if `cmd.Parent().Name() == "skill"` →
look up the canonical name in the existing `addRootAlias` registration
table. If we don't do this, the schema will have both `add` and
`skill add` as separate `command` values, which makes
de-dup and counting hard on the server.

Alternative: always use the fully-qualified path
(`skill add`, `project sync`) by joining `cmd.Parent().Name() + " " +
cmd.Name()`. CONTEXT says "the cobra subcommand name, e.g.
`check-security`, `enable`, `add`, `check-overlap`" — so the
unqualified name is wanted. Filter aliases in the event-emit
function: build a `map[string]bool{"add": true, "delete": true,
"enable": true, "disable": true, "check-updates": true}` and skip the
top-level alias form (the parent-relative path is the canonical one).

### P7. Buffer cap eviction: read-modify-write race with the cap check

**What goes wrong:** The CONTEXT decision: "When the buffer would
exceed 1 MB, the oldest events are dropped (FIFO)." The naive
implementation (read file → parse → append new event → check size →
drop oldest if over cap → rewrite) rewrites the file from scratch on
every run. That's O(N) on every CLI invocation, which is fine for
N ≤ ~5000 events (~200 bytes each) but gets slow on a long-offline
machine that has accumulated the full 1 MB.

**How to avoid:** Two-step approach:

1. **Append the new event** (with `O_APPEND`, atomic per P3) to the
   buffer file.
2. **After the append**, check `os.Stat` size. If `≤ 1 MB`, done.
3. **If `> 1 MB`**, read the whole file, drop events from the front
   until size ≤ 1 MB, and rewrite. The rewrite is the rare path —
   the user has to be offline for 1 MB worth of events, which is
   ~5000 commands. Rewrite the file in a single
   `os.WriteFile` (the file is at most 1 MB; this is sub-millisecond).

The "always append first, then check" pattern is correct because the
cap is a *post-condition* (the file is never observed to exceed
1 MB), not a *pre-condition* (we don't need to check before writing).
This is simpler and avoids the "read size, decide, then write" race.

### P8. Schema test stability: timestamps and ULIDs are non-deterministic

**What goes wrong:** The CONTEXT asks for a "byte-for-byte" schema
test. But `timestamp` is `time.Now().UTC()` and `event_id` is a fresh
ULID. A naive equality assertion will fail every run.

**How to avoid:** Two test helpers:

1. **`assertSchemaShape(t, payload []byte)`** — unmarshal into
   `map[string]any`, assert exactly 7 keys, assert the JSON types of
   each field. This catches "missing field", "extra field", and
   "wrong type" without depending on the value.
2. **`assertSchemaExample(t, payload []byte)`** — assert the *example*
   payload in `OBSERVABILITY.md` matches what the recorder emits
   (modulo `event_id`, `install_id`, `host_id`, `timestamp`).
   Substitute the four volatile fields with regex/placeholder checks
   (e.g., `event_id` matches `^[0-9A-HJKMNP-TV-Z]{26}$` for
   Crockford base32; `install_id` and `host_id` match `^[0-9a-f]{32}$`;
   `timestamp` matches `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).

Plus: an integration test with `httptest.NewServer` that captures the
raw POST body and asserts the *unparsed* JSON has the right shape
(`json.Valid`, the seven expected top-level keys). This is the
"byte-for-byte" check the CONTEXT asks for.

### P9. The factory func var can shadow the wrong recorder in tests

**What goes wrong:** The CONTEXT says "Test code can override the
factory via a swappable func var (same pattern as
`agenttools.ChooseAgentToolFunc`)." The existing pattern is a
package-level `var XxxFunc = xxxImpl` and a function `Xxx()`
that calls `XxxFunc()`. Test code does
`t.Cleanup(func() { agenttools.ChooseAgentToolFunc = ... })`. If the
planner copies this pattern but forgets the `t.Cleanup` reset, the
func var leaks into the next test, and the next test sees the wrong
recorder.

**How to avoid:** Standard test pattern in the repo:
`maintenance.IsServiceRunningFunc` is set in `root.go:62-68` to a
non-default value at init time, but for *tests* the
func var is reassigned in `t.Cleanup`. The new code should:

- Default `RecorderFactoryFunc = newDefaultRecorder`
- In tests, `old := telemetry.RecorderFactoryFunc;
telemetry.RecorderFactoryFunc = func() { return fakeRecorder };
t.Cleanup(func() { telemetry.RecorderFactoryFunc = old })`

Also, since the recorder is created *per command* (we want a fresh
HTTP client per run to avoid connection reuse across processes), the
factory is called inside `RunE`, not at init time.

### P10. The first-run prompt is non-TTY in CI, but the user expected a TTY

**What goes wrong:** A user runs `skill-organizer` for the first time
in their terminal (TTY) — gets the prompt, picks "yes", config is
written. They re-run in CI (non-TTY) — the prompt is *not* shown
(because non-TTY), but the "yes" answer is already in YAML, so
telemetry fires. Good. But: if the user later *edits* the YAML to
remove the `telemetry.enabled` key (or the file is deleted), the
prompt will fire on the *next non-TTY* run too — because we can't
ask, we just record the default ("no"). Now telemetry is permanently
off for that user, even if they wanted it on.

**How to avoid:** The non-TTY fallback should *not* write the default
to the config — it should leave the config untouched so that the
*next TTY run* gets the prompt. Concretely:

- TTY + no answer in YAML: ask, write the answer (yes or no).
- Non-TTY + no answer in YAML: skip the prompt; do not write to YAML.
  Emit a one-line info message to stderr: "telemetry: first-run
  prompt skipped (no TTY); set `telemetry.enabled: true` in
  `~/.config/skill-organizer/skill-organizer.yml` to enable."
- TTY + answer in YAML: skip (already sticky).

This is a small but real behavior difference from "default = off"
when the user is in a non-TTY environment. Document it in the
`OBSERVABILITY.md` "How to enable / disable" section.

---

## Existing Patterns in This Codebase

These are the patterns the planner should reuse, with the exact
file:line citations so the plan actions can be checked.

- **Func-var test injection** — `agenttools.ChooseAgentToolFunc` is
  declared at `packages/cli/internal/agenttools/agenttools.go:172`,
  with the swappable `StartSpinnerFunc`, `SelectInstalledToolFunc`,
  `LaunchSessionFunc`, etc. [VERIFIED:
  packages/cli/internal/agenttools/agenttools.go:172-258]. The new
  `RecorderFactoryFunc`, `NewHTTPClientFunc`, `IsStdInTTYFunc`, and
  `FirstRunPromptFunc` should follow the same pattern (package-level
  `var XxxFunc = xxxImpl`, plus a wrapper function `Xxx()` that
  delegates to the var). Test code reassigns in `t.Cleanup`.

- **`AppConfig` sub-struct for first-class config** — `ServiceConfig`,
  `AgentSelectionConfig`, `BackupConfig` are sub-structs of
  `AppConfig` at `packages/cli/internal/config/config.go:32-52`.
  Add `TelemetryConfig` as the next sub-struct, with
  `LoadTelemetryConfigOrDefault` / `SaveTelemetryConfig` next to
  `LoadBackupConfigOrDefault` / `SaveBackupConfig` at
  `packages/cli/internal/config/registry.go:181-204`. Mirror the
  Load/Save pattern exactly: the `Save` function reads the full
  `AppConfig`, mutates the sub-struct, writes back. [VERIFIED:
  packages/cli/internal/config/registry.go:181-204]

- **YAML migration for renamed config keys** — the
  `rawOverlapKeys` pattern at
  `packages/cli/internal/config/registry.go:13-18` migrates the old
  `overlap.*` keys to `agent-selection.*`. Use the same pattern if
  we ever rename `telemetry.*` keys in a future version. Not needed
  for the initial P3.

- **`AppDir()` for the new on-disk files** — `configpkg.AppDir()` at
  `packages/cli/internal/config/registry.go:22-28` returns
  `<UserConfigDir>/skill-organizer`. The new
  `<AppDir>/install_id`, `<AppDir>/host_id`,
  `<AppDir>/telemetry-buffer.jsonl` files all live here. [VERIFIED:
  packages/cli/internal/config/registry.go:22-28]

- **`confirm(prompt, defaultValue bool)` for the first-run prompt** —
  `packages/cli/cmd/prompt.go:127-134` wraps
  `pterm.DefaultInteractiveConfirm.WithDefaultValue(...)`. The CONTEXT
  asks for "default = off" → pass `false` as `defaultValue`. Don't
  roll a new prompt UI; reuse this.

- **Opportunistic maintenance in `PersistentPreRun`** — the existing
  `maintenance.MaybeRunBackupGC` and
  `maintenance.MaybeNotifySkillUpdates` (at
  `packages/cli/internal/maintenance/maintenance.go:14-49, 56-...`)
  are the exact "fire-and-forget, swallow errors" pattern the
  telemetry first-run prompt and buffer drain need to follow. Use
  the same signature: `func(ctx context.Context, stdout io.Writer)`,
  return nothing, drop errors. [VERIFIED:
  packages/cli/internal/maintenance/maintenance.go:14-49]

- **Cobra flag definitions** — use
  `cmd.PersistentFlags().StringVar(&ptr, "name", default, "help")`
  for the `--telemetry-endpoint` flag (so it works on every
  subcommand, including the telemetry subcommand itself). The
  existing pattern at `packages/cli/cmd/root.go:69` uses
  `rootCmd.PersistentFlags()` for the `--config` flag. [VERIFIED:
  packages/cli/cmd/root.go:69]

- **Cobra `RunE` with error wrapping** — every command wraps
  errors with `fmt.Errorf("...: %w", err)`. The recorder should
  follow the same convention when logging buffer-write failures
  (use `pterm.Warning` to stderr, not `panic`).

- **pterm color rules** — from `AGENTS.md`: yellow is reserved for
  keyboard hints; magenta/cyan/light-magenta for status; green/red
  for success/failure. The first-run prompt's prompt text should
  use the default pterm style (no special color); the success/fail
  line after the user answers can use `pterm.Success` / `pterm.Info`
  for green/cyan.

- **Test fixtures: `t.TempDir` + `t.Setenv` + `t.Cleanup`** — the
  `e2e_test.go` and `*_test.go` files use this pattern. For
  telemetry tests, set `XDG_CONFIG_HOME` (or
  `HOME` on macOS) to point to `t.TempDir()` so
  `configpkg.AppDir()` returns a writable temp dir. Use
  `t.Setenv` (auto-restored). [VERIFIED: existing
  `skill_overlap_test.go` uses this pattern.]

- **Stdlib only — no testify, no omock** — from
  `.planning/codebase/CONVENTIONS.md` and `AGENTS.md`. The new
  test code uses `testing.T`, `httptest.NewServer`, and the
  counting transport. No new dependencies.

---

## Recommended Approach

The smallest correct implementation is a single new package
`packages/cli/internal/telemetry/` containing seven files, plus
small changes in `cmd/root.go` and `cmd/skill.go` (or a new
`cmd/telemetry.go`) to wire the cobra subcommand and the
PersistentPreRun / PersistentPostRun hooks. The
`OBSERVABILITY.md` doc is a 7-section markdown file at the repo
root.

**1. New package `internal/telemetry/`** [HIGH confidence]
  - `event.go` — the `Event` struct (7 fields, snake_case JSON tags,
    in the order from CONTEXT: `command`, `exit_status`,
    `install_id`, `host_id`, `timestamp`, `version`, `event_id`).
    Use a `MarshalJSON` if you need field order, but `encoding/json`
    on a struct with explicit tags is sufficient.
  - `recorder.go` — the `Recorder` interface
    (`Record(ctx, event Event) error`), the `NoopRecorder` (zero
    allocations, drops), the `HTTPRecorder` (POSTs to an
    `endpoint string` with a `*http.Client`), the
    `RecorderFactoryFunc` package var, and the
    `NewHTTPClientFunc` package var (for the counting transport
    test injection).
  - `identity.go` — `Identity{InstallID, HostID string}` plus
    `LoadOrCreate(appDir string) (Identity, error)` and
    `RotateHostID(appDir string) (string, error)`. Uses
    `crypto/rand.Read(buf[:16])` and
    `encoding/hex.EncodeToString`. Writes one file per ID
    (`<AppDir>/install_id`, `<AppDir>/host_id`). On read, validates
    the on-disk file is `^[0-9a-f]{32}$`; if not, regenerates and
    warns. (Old or corrupted IDs are rare but possible — the
    fix is "regenerate, the user has not opted out").
  - `buffer.go` — JSONL spool with `O_APPEND` writes,
    `drain(callback func(Event) error)` method (reads, parses,
    calls the callback, on success truncates), and
    `evict()` (drops oldest events until ≤ 1 MB). The `Buffer` type
    is constructed with a `path string` and exposes
    `Append(event Event) error` and
    `Drain(send func(Event) error) error`.
  - `prompt.go` — `FirstRunPrompt(stdout io.Writer, stdin io.Reader) (bool, error)`
    that wraps the existing `confirm` helper. Returns
    `false, nil` on non-TTY (so the default sticks, but the YAML
    is *not* written — see Pitfall P10).
  - `telemetry.go` — the umbrella file with the public API
    `New(cfg TelemetryConfig, appDir string) (*Service, error)`
    returning a `*Service` that holds the `Recorder`, the
    `*Buffer`, and the `Identity`. Also exports
    `MaybeRunFirstRunPrompt`, `MaybeDrainBuffer`,
    `RecordEvent` for use from `cmd/root.go`.
  - `*_test.go` co-located with each file. Tests use
    `httptest.NewServer` for the byte-for-byte schema test and
    the counting transport for the zero-egress test. The
    `buffer_test.go` exercises the FIFO eviction (write 5000
    events, assert file size ≤ 1 MB and the first events are
    gone).

**2. New cobra subcommand `cmd/telemetry.go`** [HIGH confidence]
  - Subcommands: `enable`, `disable`, `status`, `rotate-host-id`.
  - `enable` / `disable` write `telemetry.enabled: true|false` to
    YAML via the existing `configpkg.SaveTelemetryConfig` helper.
  - `status` prints the current state (enabled, endpoint,
    install_id prefix, host_id prefix, buffer size) to stdout.
    The CONTEXT's FAQ section references this command.
  - `rotate-host-id` calls `telemetry.RotateHostID(appDir)`.
  - All four skip the first-run prompt (the
    `cmd.Name() == "telemetry"` guard from Pitfall P2).

**3. Wire `cmd/root.go`** [HIGH confidence]
  - Add `cmd.PersistentFlags().StringVar(&telemetryEndpoint,
    "telemetry-endpoint", "", "...")` for the flag.
  - In `PersistentPreRun`, *after* the existing
    `maintenancepkg.MaybeRunBackupGC` and
    `selfupdatepkg.MaybeNotify` calls, add
    `telemetrypkg.MaybeRunFirstRunPrompt(cmd.Context(),
    cmd.OutOrStdout(), cmd.InOrStdin())` and
    `telemetrypkg.MaybeDrainBuffer(cmd.Context(),
    cmd.OutOrStdout())`. Both must respect the existing
    `cmd == rootCmd` / `cmd.Name() == "completion"` / etc. gate
    already in `root.go:73-78`.
  - Add a new `PersistentPostRun` that emits the
    per-command event: reads the cobra `cmd.Name()`, normalizes
    aliases (Pitfall P6), looks up the exit status from the
    cobra command's `RunE` error (nil → 0, non-nil → 1), and
    calls `telemetrypkg.RecordEvent(ctx, event)`. The factory
    is called per-command (Pitfall P9).

**4. New `TelemetryConfig` in `internal/config/config.go`** [HIGH]
  - Struct fields: `Enabled bool` (yaml: `enabled`), `Endpoint
    string` (yaml: `endpoint`). Add to `AppConfig` at
    `packages/cli/internal/config/config.go:47-52` as
    `Telemetry TelemetryConfig` with `yaml:"telemetry,omitempty"`.
  - Add `LoadTelemetryConfigOrDefault` /
    `SaveTelemetryConfig` to
    `packages/cli/internal/config/registry.go` mirroring the
    `LoadBackupConfigOrDefault` / `SaveBackupConfig` pattern at
    `registry.go:181-204`.

**5. New `OBSERVABILITY.md` at the repo root** [HIGH]
  - The 7 sections from CONTEXT. The schema section's example
    payload must match the Go struct's JSON output exactly.
    The test `TestOBSERVABILITYExampleMatchesEmitted` reads
    the example block, substitutes the four volatile fields
    (`install_id`, `host_id`, `event_id`, `timestamp`) with
    regex placeholders, and asserts the remaining structure
    matches the recorder's output. This is the byte-for-byte
    test from CONTEXT.

**6. Tests** [HIGH]
  - `internal/telemetry/recorder_test.go`:
    - `TestNoopRecorderNoNetworkCalls` — wraps
      `http.DefaultTransport` with a counting transport, calls
      `Record` 100 times, asserts the counter is 0. (Per CONTEXT.)
    - `TestHTTPRecorderSchemaByteForByte` — uses
      `httptest.NewServer` to capture the POST body, asserts
      `json.Valid(body)`, asserts exactly 7 top-level keys,
      asserts the value of `command`, `exit_status`, `version`.
      Asserts the volatile fields match their regexes.
    - `TestRecorderFactoryReturnsNoopWhenDisabled` — confirms
      the factory short-circuits on `cfg.Enabled == false`.
    - `TestRecorderFactoryReturnsNoopWhenEndpointEmpty` —
      confirms the factory short-circuits when no endpoint is
      set, *regardless* of `Enabled`. (CONTEXT: "If none is
      set, the factory returns NoopRecorder regardless of the
      enabled flag.")
  - `internal/telemetry/buffer_test.go`:
    - `TestBufferAppendAndRead` — write 3 events, drain, assert
      callback got 3.
    - `TestBufferFIFOEvictionAt1MB` — write 6000 events (~1.2
      MB), assert drain returns 5000 events (the latest
      ~1 MB) and the first ~1000 are gone.
    - `TestBufferDrainIdempotent` — drain twice, second time
      returns 0 events.
  - `internal/telemetry/identity_test.go`:
    - `TestIdentityGenerateIs32HexChars` — generates 100
      identities, asserts each matches `^[0-9a-f]{32}$`.
    - `TestIdentityLoadOrCreateCreatesIfMissing` — uses
      `t.TempDir`, asserts the files exist after first call.
    - `TestIdentityLoadOrCreateReusesIfPresent` — calls twice,
      asserts the second call returns the same IDs.
    - `TestRotateHostIDChangesHostIDOnly` — rotates, asserts
      `install_id` is unchanged and `host_id` differs.
  - `cmd/root_test.go` (or a new
    `cmd/telemetry_test.go`):
    - `TestFirstRunPromptSkippedInNonTTY` — pipes
      `bytes.Buffer` to stdin, asserts the prompt is not shown
      and the config is *not* written (Pitfall P10).
    - `TestFirstRunPromptStickyYes` — sets TTY (via the
      func var), answers yes, asserts YAML now has
      `telemetry.enabled: true`, re-runs the command, asserts
      the prompt is *not* shown.
    - `TestFirstRunPromptStickyNo` — same, with "no".
  - `OBSERVABILITY.md` validation:
    - `TestOBSERVABILITYExampleMatchesEmitted` — see step 5.

**Suggested plan split (3-4 plans):**

- **Plan 03-01: New `internal/telemetry` package + identity
  files** — no cobra integration yet. Ship the Event struct,
  Recorder interface, NoopRecorder, HTTPRecorder, identity
  files, unit tests. One PR.
- **Plan 03-02: Buffer + first-run prompt + cobra integration**
  — add the JSONL buffer, the first-run prompt logic, wire into
  `cmd/root.go` (PersistentPreRun + PersistentPostRun), add the
  `telemetry` subcommand. One PR. Includes the byte-for-byte
  schema test.
- **Plan 03-03: `OBSERVABILITY.md` + `telemetry` subcommand
  + status command** — doc, status output, e2e test that runs
  the binary and checks the buffer file is created when
  enabled, not when disabled. One PR.

**Alternative 1-plan split:** If the user prefers fewer PRs, all
three can be one plan — total file count is ~7 new Go files +
`OBSERVABILITY.md` + small edits to 3 existing files. The P1
precedent shipped in 4 plans, P2 in 2 plans, so 3 plans fits the
established cadence.

**Alternative plan 03-00 (optional):** a research spike for the
schema example block — write the OBSERVABILITY.md doc, get user
sign-off on the exact schema and the FAQ wording, *before* the
implementation. This is recommended but not required (the user
may prefer to see the running code first). Per P2's precedent
("Refactor deliverable was a no-op"), defer the doc copy edits
to a single plan rather than scattering them.

---

## Open questions for the planner

1. **Status command output format** — table vs key-value vs YAML.
   The existing `status.go` uses a custom renderer. Match that
   style or use plain `pterm.Info.Printfln` lines?
   *Recommended:* `pterm.Info.Printfln` — it's a diagnostic, not
   a status report, and matches the `Maintenance.MaybeNotify`
   style. The status command is rarely run; readability beats
   polish.

2. **Buffer path on Windows** — `<AppDir>` resolves to
   `%AppData%\skill-organizer\` on Windows. The buffer file
   `<AppDir>\telemetry-buffer.jsonl` is fine; the FIFO
   `O_APPEND` semantics also hold on Windows for files opened
   with `FILE_APPEND_DATA`. No special handling needed.

3. **What if `~/.config/skill-organizer/` is read-only** — the
   `configpkg.SaveTelemetryConfig` will fail. The factory
   should log a warning and fall back to `NoopRecorder` for
   this run. Don't crash; the user might be on a system where
   the AppDir is locked down but they still want to use the
   CLI.

4. **Telemetry event for the telemetry subcommand itself** —
   the schema includes `command: "telemetry"` for the
   `telemetry status` call, but the `telemetry enable` and
   `telemetry disable` calls change the *next* run's behavior,
   not the current one. Should the `telemetry enable` call
   *also* emit an event? CONTEXT doesn't say. *Recommended:*
   yes, emit it — the server can dedupe by `event_id`, and the
   "user enabled telemetry" signal is high-value analytics.

5. **Server-side schema validation** — the doc says "schema
   byte-for-byte" but the server (which doesn't exist yet)
   might add fields over time. The Go recorder should NOT
   include any field the server doesn't know about, but the
   server may add fields to its response (e.g., an `accepted`
   boolean). The recorder only sends; it doesn't parse the
   response. The CONTEXT says: "On HTTP failure (offline,
   timeout, DNS error), appends the new event to the buffer."
   — so any non-2xx counts as failure. *Recommended:* 2xx is
   success; 4xx and 5xx are failures (append to buffer).
   3xx should be followed (default `http.Client` does).

---

*Phase: 03-observability*
*Research completed: 2026-06-11*
