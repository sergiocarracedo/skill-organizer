# Phase 2: Overlap refactor (REQ-3) — Research

**Researched:** 2026-06-11
**Phase goal:** `skill-organizer check-overlap` keeps its existing
agent-driven detection, but exits with a meaningful code, supports a
`--allow-overlap` flag, and gains curated-fixture test coverage.

---

## TL;DR for the planner

- The whole phase is **two production-code edits** + **fixtures** +
  **tests**. No new packages, no new exported types, no new agent
  prompts, no JSON schema change.
- Production code lives entirely in
  `packages/cli/cmd/skill_overlap.go`: one new `bool` var, one new
  flag registration, and one new 3-line `if` block in `RunE`. The
  rest is tests and fixture YAML.
- The cobra exit-code plumbing is **already wired** by `main.go` —
  just `return` a non-nil error from `RunE` to get a non-zero exit.
  The only subtlety is keeping the error message friendly and
  suppressing it with `--allow-overlap`.
- Fixtures: 6 hand-curated `SKILL.md` files in
  `packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/`.
  They are **input data** for `skills.ScanSource`, **not** parsed
  report JSON. The agent is not exercised against them.
- Agent smoke test: **already exists** as `TestRunParsesStructuredReport`
  in `overlap_test.go:83-132`. The CONTEXT asks for "a single agent
  smoke test that stubs `commandRunner` to return canned JSON" — that
  test satisfies it. We do **not** need to write a new one; we can
  add a second variant that exercises the **filter + report** path
  end-to-end (canned JSON in, `filterOverlapGroups` applied, exit
  asserted).

---

## Don't Hand-Roll

| Problem | Recommended solution | Why |
|---------|---------------------|-----|
| Custom `os.Exit(1)` / `os.Exit(2)` in `RunE` | Return a non-nil `fmt.Errorf("…")` from `RunE` | `main.go:13` already calls `os.Exit(1)` on any error returned by `cmd.Execute()`. Cobra's `SilenceUsage=true; SilenceErrors=true` (set in `cmd/root.go:91-92`) suppress cobra's own prints; `main.go:12` prints via `pterm.Error` once. No need to call `os.Exit` from inside the command. |
| A new sentinel error type `ErrOverlap` to distinguish "config error" from "overlap found" | Just `fmt.Errorf("overlap detected: %d group(s) (use --allow-overlap to suppress)", len(groups))` | The codebase has only **one** sentinel (`configpkg.ErrConfigNotFound`, `internal/config/discovery.go:15`) and uses it only for `errors.Is` checks. The "found overlap" case is a normal command outcome, not an exceptional error — a plain `fmt.Errorf` message is what the user reads. The error text is the contract. |
| A new helper function for the "has overlap after filter?" check | Inline 3-line `if` after `printOverlapReport` | The check is one expression against already-filtered `report.Groups`. Wrapping it in a named function would be ceremony. |
| A new test fixture helper that reads YAML files and parses them into `SkillInfo` | Reuse `skills.ScanSource` + `skills.LoadDocument` against a `t.TempDir()` copy of the curated `testdata/overlap/` files | `skills.ScanSource` (`internal/skills/scanner.go:22-40`) and `CollectSkills` (`internal/overlap/overlap.go:47-88`) are the production codepath. Exercising them in tests gives real coverage, not a parallel-reality test. |
| Mocking the agent for the smoke test | Swap the package-level `commandRunner` var with a fake, restore in `t.Cleanup` | Established pattern across the codebase (`security/security_test.go:71-94`, `overlap/overlap_test.go:84-108`). The project does not use `gomock` or `mockery` — function-variable injection is the convention (`CONVENTIONS.md:88-93`, `TESTING.md:177-220`). |
| Building a brand-new "exit code 2 vs 1" taxonomy | Just use `1` for all overlap-found exits | The `main.go` path treats all non-nil errors as exit 1. Distinguishing "overlap" vs "error" in the exit code would require changes to `main.go` and to the `Execute()` error-handling dance in `cmd/root.go:32-48`. Out of scope. |

---

## Common Pitfalls

### Pitfall 1 — `--allow-overlap` bypasses the apply-plan flow
**What goes wrong:** A naive "early-return" for `--allow-overlap`
(placed before the `if overlapNoAskToApply` block) skips the
"ask the agent to prepare an apply plan" prompt. The user has to
re-run the command to get a plan.

**Why:** The CONTEXT's wording — "flag variable + a single
early-return before the exit-code check" — is ambiguous between
"the flag itself early-returns" and "the flag's implementation
*is* the exit-code check, which is an early-return". The CONTEXT
also says "The report is still printed", which is consistent only
with the second reading.

**How to avoid:** Put the check **after** `printOverlapReport(...)`
and **before** the `if overlapNoAskToApply { return nil }` branch:

```go
printOverlapReport(tool, len(items), overlapAllSkills, report)

// P2: exit-code check (--allow-overlap suppresses the non-zero exit)
if len(report.Groups) > 0 && !overlapAllowOverlap {
    return fmt.Errorf(
        "overlap detected: %d group(s) (use --allow-overlap to ignore)",
        len(report.Groups),
    )
}

if overlapNoAskToApply {
    return nil
}
// ... rest of apply-plan flow
```

This way `--allow-overlap` skips the non-zero exit but still goes
through the apply-plan prompt flow. `--no-ask-to-apply` is
independent — it skips the apply-plan flow but does **not**
suppress the exit code.

[VERIFIED: packages/cli/cmd/skill_overlap.go:143-147 — current
`overlapNoAskToApply` early-return is at line 146, after
`printOverlapReport` at line 143. The exit-code check must slot
in between, exactly as drawn above.]

### Pitfall 2 — pterm.Error noise on "successful" overlap exits
**What goes wrong:** Returning `fmt.Errorf("overlap detected: …")`
from `RunE` causes `main.go:12` to call `pterm.Error.Printfln`,
which prints a red "ERROR  …" banner to stderr. That's noisy for
a routine, expected outcome (the user just wants CI to fail).

**Why:** `main.go:11-14` calls `pterm.Error.Printfln("%v", err)`
on any error from `Execute()`. There's no path to "silent
non-zero exit".

**How to avoid:** **Accept the noise.** It is what every other
cobra command in this repo does on error (e.g. `return
fmt.Errorf("aborted")` in `skill_check_updates.go`, `skill_delete.go`,
`onboard.go`, etc. — all surface the same red banner).
The error message itself should be terse and actionable:
`"overlap detected: 1 group(s) (use --allow-overlap to ignore)"`
not `"FATAL: overlap analysis returned 1 result"`. The user / CI
log will see the message, and `os.Exit(1)` is what gates CI.

**Optional polish (only if asked):** Introduce a sentinel
`overlapFoundErr` and special-case it in `main.go` to skip the
`pterm.Error` call. **Don't do this in P2** — it's a refactor
that affects every other command's exit path and is out of
scope.

[VERIFIED: packages/cli/main.go:11-14; packages/cli/cmd/root.go:91-92
(SilenceUsage + SilenceErrors).]
[CITED: https://context7.com/spf13/cobra/llms.txt — Cobra's
`Execute()` returns the RunE error; the caller chooses what
to print and whether to call `os.Exit(1)`.]

### Pitfall 3 — Test fixture files end up in the binary build
**What goes wrong:** Placing `SKILL.md` files inside the package
directory without the `testdata/` prefix means `go build` will try
to embed them, or `go test` will refuse to compile.

**Why:** Go's package model reserves the `testdata/` directory as
the canonical place for test inputs. Files at the package root
that don't end in `.go` cause `go build` warnings.

**How to avoid:** Use the path exactly as the CONTEXT specifies —
`packages/cli/internal/overlap/testdata/overlap/{conflicting,clean,partial}/`.
That path is **outside** the package's source compilation set
(Go skips `testdata/`).

[VERIFIED: Go spec; `.planning/phases/02-overlap-refactor/02-CONTEXT.md:86`.]

### Pitfall 4 — Fixture files not committed / not loadable in CI
**What goes wrong:** The test references
`testdata/overlap/conflicting/alpha/SKILL.md` via
`os.ReadFile(filepath.Join("testdata", ...))` from inside the
test, but the working directory in `go test` is the package
directory, not the repo root. The path resolution works, but the
test fails if anyone ever runs `go test ./...` from the repo
root expecting fixtures to live somewhere else.

**Why:** `go test ./packages/cli/internal/overlap/...` sets
`pwd` to `packages/cli/internal/overlap/` before running the
test, so relative `testdata/...` paths resolve correctly.

**How to avoid:** Use **bare relative paths** like
`filepath.Join("testdata", "overlap", "conflicting", "alpha", "SKILL.md")`
in the test. The project's `e2e_test.go` uses the same idiom
with `testdata/fake-skills-cli.go` (`e2e_test.go:418-421`).

[VERIFIED: packages/cli/e2e_test.go:418-421 — the e2e tests
reference `testdata/fake-skills-cli.go` via a relative path
that resolves from the package's working directory.]
[HIGH confidence — this is standard Go test behavior.]

### Pitfall 5 — Func-var swap leaks across tests
**What goes wrong:** A test reassigns `commandRunner` (or any
other `var`) but forgets `t.Cleanup`. The next test gets the fake
and panics on a nil-binary call, or worse, silently produces
wrong results.

**Why:** Package-level vars are global. The repo's pattern is
"assign in test, restore in t.Cleanup" — easy to forget when
the test is small.

**How to avoid:** In **every** new test that touches a func var:
```go
original := commandRunner
commandRunner = func(_ context.Context, _ string, _ []string, _ func(string)) (string, error) {
    return cannedJSON, nil
}
t.Cleanup(func() { commandRunner = original })
```
For the `overlapAllowOverlap` `bool` var (which is **not** a
func var), the existing pattern is to swap the bool and reset
in `t.Cleanup`:
```go
original := overlapAllowOverlap
overlapAllowOverlap = true
t.Cleanup(func() { overlapAllowOverlap = original })
```
The bool is plain package state and is mutated by the cobra
flag-binding machinery; tests must reset it.

[VERIFIED: packages/cli/internal/overlap/overlap_test.go:84-108 —
the `TestRunParsesStructuredReport` pattern. packages/cli/cmd/skill_overlap_test.go:130-139 — the
`overlapPrintPrompt` swap-and-cleanup pattern.]

### Pitfall 6 — `auto_trigger` is not parsed by the CLI
**What goes wrong:** A test fixture includes `auto_trigger:` and
the test asserts the field round-trips into a `SkillInfo` struct.
The CLI does not parse `auto_trigger` — the frontmatter layer
treats it as opaque YAML.

**Why:** `Document.ManagedMetadata()` (`internal/skills/frontmatter.go:126-175`)
only reads the `metadata.skill-organizer` block. `Document.Name()`
and `Document.Description()` only read the top-level `name` and
`description`. `auto_trigger` is mentioned in
`.planning/phases/02-overlap-refactor/02-DISCUSSION-LOG.md:53-54` and
in `openspec/changes/build-skill-organizer-cli/design.md:33`, but
is **not** read by the CLI. It is purely an input to the agent.

**How to avoid:** Fixture YAML can include `auto_trigger:` (it
is preserved verbatim by the frontmatter layer, see
`frontmatter_test.go:11, 39`), but the test must **not** assert
on it being lifted into a struct field. Assert on
`Name()`, `Description()`, and on the prompt's serialized output
(which contains the full text).

[VERIFIED: packages/cli/internal/skills/frontmatter_test.go:11, 39 —
`auto_trigger` is preserved but not exposed.]
[VERIFIED: packages/cli/internal/skills/frontmatter.go:94-106 —
`Name()` and `Description()` are the only top-level fields read.]

### Pitfall 7 — Disabling `overlapPrintPrompt` collides with the
                  `--print-prompt` early-return
**What goes wrong:** A test that exercises the new exit-code
path sets `overlapPrintPrompt = true` for convenience, but that
short-circuits the whole command at line 77 (`return nil`) before
the exit-code check ever runs.

**Why:** The `--print-prompt` early-return is unconditional and
sits **above** the overlap-run / report-print / exit-code
sections. There is no overlap between the two — `--print-prompt`
never produces a report, so it never produces a non-zero exit.

**How to avoid:** Tests that exercise the new exit-code logic
must keep `overlapPrintPrompt = false` and stub `runOverlapAnalysis`
to return canned reports directly. The stubbed
`runOverlapAnalysis` already exists in the test file
(`skill_overlap_test.go:315-317`); the new tests just reuse it
with different return values and assert the err from `cmd.RunE`.

[VERIFIED: packages/cli/cmd/skill_overlap.go:75-78 — the
`--print-prompt` early-return.]
[VERIFIED: packages/cli/cmd/skill_overlap_test.go:315-317 — the
`runOverlapAnalysis` stub.]

### Pitfall 8 — Skipping the spinner/header for the unit tests
**What goes wrong:** Calling `cmd.RunE` exercises
`agenttools.StartSpinner` (line 127) and the full `printOverlapReport`
function with pterm output. Tests get ANSI color codes in their
captured output, or — worse — a nil-pointer panic if the stub
spinner returns `nil`.

**Why:** The test file's `stubSpinner` (`skill_overlap_test.go:445-449`)
implements `agenttools.SpinnerHandle` but the test still calls
`agenttools.StartSpinner` through the `agenttools.StartSpinnerFunc`
func var.

**How to avoid:** In the new tests, stub `agenttools.StartSpinnerFunc`
to return `stubSpinner{}` (the same helper that already exists at
`skill_overlap_test.go:445-449`). Reuse the helper, do not
redefine a new one. Also stub `printInfoMessage`, `printDebugMessage`,
`printWarningMessage` to record instead of print — the
`TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
test at `skill_overlap_test.go:336-348` shows the exact pattern.

[VERIFIED: packages/cli/cmd/skill_overlap_test.go:292, 346-348 —
the spinner stub pattern.]

---

## Existing Patterns in This Codebase

### Pattern 1: `--bool-flag` + early-return-after-render
**Where:** `packages/cli/cmd/skill_overlap.go:197-198`
(`overlapMinType` / `--min-overlap-type`) and the apply-plan flow
at `:145-147` (`overlapNoAskToApply`).
**How it works:** A package-level `bool` (or `string`) var, a
`cmd.Flags().BoolVar` (or `StringVar`) call near the other flag
registrations, and a read of the var at the right point in
`RunE`.
**When to reuse:** **The new `--allow-overlap` flag.** Mirror the
declaration style of `overlapNoAskToApply` exactly:
```go
var (
    ...
    overlapAllowOverlap bool
)
// in newCheckOverlapCommand:
cmd.Flags().BoolVar(&overlapAllowOverlap, "allow-overlap", false,
    "Exit 0 even when overlap groups are found (the report is still printed)")
```

### Pattern 2: `fmt.Errorf` returned from `RunE` for non-zero exit
**Where:** `skill_check_updates.go`, `skill_delete.go`, `onboard.go`,
`watched.go`, `remove.go` — all use `return fmt.Errorf("aborted")`
or `return fmt.Errorf("interrupted")` to exit non-zero.
**How it works:** `main.go:11-14` reads the error and calls
`os.Exit(1)`. `pterm.Error.Printfln` formats it. The user sees a
red `ERROR   <msg>` line in the terminal.
**When to reuse:** The new "overlap detected, exit 1" case. Use
the exact same shape:
```go
return fmt.Errorf(
    "overlap detected: %d group(s) (use --allow-overlap to ignore)",
    len(report.Groups),
)
```

### Pattern 3: `TestRunParsesStructuredReport` — the agent smoke test
**Where:** `packages/cli/internal/overlap/overlap_test.go:83-132`.
**How it works:**
1. Stash the original `commandRunner` package var.
2. Replace it with a closure that returns a hard-coded JSON
   string and optionally invokes the `onStatus` callback.
3. `t.Cleanup` restores the original.
4. Call `overlap.Run(ctx, tool, prompt, onStatus)` and assert on
   the returned `Report`.
**When to reuse:** This **already** satisfies the CONTEXT's "single
agent smoke test that stubs `commandRunner` to return canned JSON".
The planner should **not** write a new smoke test; it should add
**one** additional test (or extend this one with a `t.Run` subtest)
that exercises the **filter** path — i.e. canned JSON with both
`partial` and `adjacent` groups, then assert that
`filterOverlapGroups(groups, 2)` returns only the `partial` ones.
That is the test that ties `Run` to `filterOverlapGroups` and
proves the integration works.

[VERIFIED: packages/cli/internal/overlap/overlap_test.go:83-132 —
the exact pattern; modeled on `security_test.go:71-94`.]

### Pattern 4: Fixture helper `createSkill(t, root, relPath, name, description, disabled)`
**Where:** `packages/cli/internal/overlap/overlap_test.go:231-249`.
**How it works:** A `t.Helper()`-decorated function that creates a
directory and writes a minimal `SKILL.md` with the given
frontmatter.
**When to reuse:** **For the new fixture-based tests, do NOT use
this helper.** It creates files *inline in the test* — the
"curated fixtures on disk" decision in the CONTEXT
(`02-CONTEXT.md:86`) explicitly asks for fixture files committed
to the repo. The helper is the right tool for synthetic
single-skill tests (which already exist); the **new** tests
should:
1. Read the curated `SKILL.md` content from
   `testdata/overlap/<scenario>/<skill>/SKILL.md` via
   `os.ReadFile`.
2. Write them into a `t.TempDir()` to keep `ScanSource`'s contract
   (it expects a real directory tree).
3. Call `overlap.CollectSkills` (or `skills.ScanSource` +
   `LoadDocument`) the same way `CommandRunner` does at runtime.

This is the **only** place in the test plan where a small
`copyFixture(t, srcDir, destDir)` helper is worth introducing —
it eliminates 6× `os.ReadFile` + `os.WriteFile` lines per
scenario. Define it once in `overlap_test.go` next to the
existing `createSkill` helper.

### Pattern 5: `TestCheckOverlapUnsupportedToolSavesPrompt…` — the
              full-command test with all stubs
**Where:** `packages/cli/cmd/skill_overlap_test.go:274-394`.
**How it works:** Stubs every func var (`detectInstalledTools`,
`loadAgentSelectionConfigFunc`, `runOverlapAnalysis`,
`confirmApplyPlan`, `confirmExternalCosts`, `saveApplyPlanPrompt`,
`agenttools.StartSpinnerFunc`, `agenttools.LaunchSessionFunc`,
`printInfoMessage`, `printDebugMessage`, `printWarningMessage`),
stubs `loadResolvedLocationFunc` to return a temp config path,
stubs `collectOverlapSkills` to return a canned `[]SkillInfo`,
then calls `cmd.RunE(cmd, nil)` and asserts on the captured
side effects.
**When to reuse:** **The new exit-code + `--allow-overlap` tests.**
Copy the stub-set from `TestCheckOverlapUnsupportedToolSavesPrompt…`
verbatim — every var is the same. Add `overlapAllowOverlap` to the
stash+set+restore pattern. The only change is the canned report
returned by `runOverlapAnalysis` (the existing test returns a
single `partial` group; the new tests need one with groups for
"exit 1" and one empty for "exit 0") and the assertion (instead
of `err == nil`, assert `err != nil` and contains `"overlap
detected"`; for the `--allow-overlap` case, assert `err == nil`).

[VERIFIED: packages/cli/cmd/skill_overlap_test.go:274-394 — the
canonical "stub everything, call RunE, assert side effects"
pattern.]

### Pattern 6: `parseMinOverlapType` table-driven test
**Where:** `packages/cli/cmd/skill_overlap_test.go:175-204`.
**How it works:** Slice of `{input, wantRank, wantLabel}` structs
plus a `t.Fatalf` inside the loop. Not the `tests := []struct{...}`
table idiom — the repo's `TESTING.md:114-119` notes that simple
`for _, want := range []string{…}` is preferred over
`tests := []struct{…}` tables.
**When to reuse:** **No** — the new flag is a `bool`, not a
parseable string. Skip this pattern. The `--min-overlap-type`
test stays untouched.

### Pattern 7: Fixture load via `os.ReadFile` + `t.TempDir` copy
**Where:** Not yet present in the CLI; the closest analog is
`packages/cli/testdata/fake-skills-cli.go` (a Go source file
read via `os/exec`) and the e2e tests' `e2e_test.go:418-421`
which reference `testdata/fake-skills-cli.go` from a temp
binary.
**How it works:** Reads the file from the test's CWD
(`packages/cli/internal/overlap/`) into a byte buffer, then
writes it to a `t.TempDir()` path that mirrors the directory
layout, so `skills.ScanSource` can find it.
**When to reuse:** **Yes — the new tests in `overlap_test.go`.**
The pattern is straightforward but worth pulling into a single
helper:

```go
func loadFixtureRoot(t *testing.T, scenario string) string {
    t.Helper()
    root := t.TempDir()
    src := filepath.Join("testdata", "overlap", scenario)
    entries, err := os.ReadDir(src)
    if err != nil {
        t.Fatalf("read fixture dir %q: %v", src, err)
    }
    for _, entry := range entries {
        copyDir(t,
            filepath.Join(src, entry.Name()),
            filepath.Join(root, entry.Name()),
        )
    }
    return root
}

func copyDir(t *testing.T, src, dst string) {
    t.Helper()
    if err := os.MkdirAll(dst, 0o755); err != nil {
        t.Fatalf("MkdirAll(%q): %v", dst, err)
    }
    skillFile := filepath.Join(src, skills.SkillFileName)
    data, err := os.ReadFile(skillFile)
    if err != nil {
        t.Fatalf("ReadFile(%q): %v", skillFile, err)
    }
    if err := os.WriteFile(filepath.Join(dst, skills.SkillFileName), data, 0o644); err != nil {
        t.Fatalf("WriteFile: %v", err)
    }
}
```

[VERIFIED: internal/skills/scanner.go:13 — `SkillFileName = "SKILL.md"`.]

---

## Recommended Approach

### Production code (one commit, two files)

In `packages/cli/cmd/skill_overlap.go`:

1. **Add the package var** alongside the existing
   `overlapNoAskToApply` (line 19-26):
   ```go
   var (
       ...
       overlapAllowOverlap bool
   )
   ```

2. **Register the flag** alongside the others (line 193-198), in
   the same kebab-case / camelCase style:
   ```go
   cmd.Flags().BoolVar(&overlapAllowOverlap, "allow-overlap", false,
       "Exit 0 even when overlap groups are found (the report is still printed)")
   ```

3. **Add the exit-code check** in `RunE` **after** the call to
   `printOverlapReport` (line 143) and **before** the
   `if overlapNoAskToApply` block (line 145). Do not move the
   apply-plan flow:
   ```go
   printOverlapReport(tool, len(items), overlapAllSkills, report)

   // P2: non-zero exit on overlap (--allow-overlap suppresses it)
   if len(report.Groups) > 0 && !overlapAllowOverlap {
       return fmt.Errorf(
           "overlap detected: %d group(s) (use --allow-overlap to ignore)",
           len(report.Groups),
       )
   }

   if overlapNoAskToApply {
       return nil
   }
   // ... existing apply-plan flow unchanged
   ```

That is the **entire** production diff. No other file changes
needed.

### Fixtures (one commit, six files)

Create `packages/cli/internal/overlap/testdata/overlap/` with three
subdirectories. Each subdirectory is a scenario; each scenario
contains 2-3 `SKILL.md`-bearing skill directories.

```
testdata/overlap/
├── conflicting/
│   ├── alpha/SKILL.md      # same trigger / same description
│   └── beta/SKILL.md       # same trigger / same description
├── clean/
│   ├── alpha/SKILL.md      # unrelated description
│   └── beta/SKILL.md       # unrelated description
└── partial/
    ├── alpha/SKILL.md      # shares one trigger
    ├── beta/SKILL.md       # shares one trigger
    └── gamma/SKILL.md      # shares the OTHER trigger with beta only
```

**Fixture YAML shape** (the frontmatter layer treats `name`,
`description`, and `metadata.skill-organizer` as structured; any
other keys — including `auto_trigger` — are preserved verbatim
and ignored by the CLI but read by the agent):

```yaml
---
name: alpha
description: Run when the user asks to draft a release announcement for a new software release.
auto_trigger:
  - keywords: ["release announcement", "release notes"]
---

# Alpha skill body
```

For the **conflicting** pair, use the *same* `auto_trigger` and
*overlapping* `description`. For the **clean** pair, use
*disjoint* `auto_trigger` and *disjoint* `description`. For
**partial**, the middle skill shares a trigger with each neighbor
but the neighbors share nothing.

**Do not** add a `.gitignore` for `testdata/` — Go's tooling
expects the directory to be present in the working tree.

### Tests (one commit, two files)

In `packages/cli/internal/overlap/overlap_test.go`:

1. **Add the fixture helpers** (`loadFixtureRoot`, `copyDir`) at
   the bottom of the file, next to `createSkill` and
   `mockInstalledTool`.

2. **Add `TestCollectSkillsOnConflictingFixture`** — calls
   `loadFixtureRoot(t, "conflicting")`, runs
   `overlap.CollectSkills(location, false)`, asserts the two
   expected `SkillInfo` rows are present and have the expected
   `Name` / `Description` / `RelativePath` / `FlattenedName`.

3. **Add `TestCollectSkillsOnCleanFixture`** — same shape,
   asserts the two skills are scanned and `Disabled == false`.

4. **Add `TestRunParsesFilteredReport`** — wraps
   `TestRunParsesStructuredReport`'s pattern, returns canned JSON
   with both `partial` and `adjacent` groups, asserts that
   `overlap.filterOverlapGroups(report.Groups, 2)` returns the
   `partial` group only. (This is the test the CONTEXT asks for
   as the "agent smoke test" — it already exists, this is the
   filter+integration variant.)

In `packages/cli/cmd/skill_overlap_test.go`:

5. **Add `TestCheckOverlapExitsNonZeroOnGroups`** — copy the
   stub-set from
   `TestCheckOverlapUnsupportedToolSavesPromptInsteadOfLaunchingPlanMode`
   (lines 274-394). Set `runOverlapAnalysis` to return one
   `partial` group. Set `overlapNoAskToApply = true` and
   `overlapAllowOverlap = false`. Call `cmd.RunE(cmd, nil)`.
   Assert `err != nil` and `strings.Contains(err.Error(),
   "overlap detected")`. Cleanup restores all vars.

6. **Add `TestCheckOverlapAllowOverlapForcesExitZero`** — same
   stub-set, same canned report, but `overlapAllowOverlap = true`
   and `overlapNoAskToApply = true` (so we don't get tangled up
   in the apply-plan flow in the unit test). Assert `err == nil`.

7. **Add `TestCheckOverlapExitsZeroOnEmptyReport`** — same
   stub-set, `runOverlapAnalysis` returns `overlap.Report{}` (no
   groups), `overlapAllowOverlap = false`. Assert `err == nil`.

### What NOT to do

- Do **not** add a `--min-score` flag. The CONTEXT explicitly
  defers it (`.planning/phases/02-overlap-refactor/02-CONTEXT.md:216-218`).
- Do **not** add a per-tool smoke test against the real
  Claude/Codex/OpenCode/Cursor/Antigravity binaries. The CONTEXT
  drops it (`02-CONTEXT.md:213-215`).
- Do **not** introduce a local deterministic overlap rule. The
  CONTEXT defers it (`02-CONTEXT.md:208-211`).
- Do **not** change `Report.Groups` shape. The CONTEXT locks it
  (`02-CONTEXT.md:44-65`).
- Do **not** change the default `--min-overlap-type` value. The
  CONTEXT locks it as `partial` (`02-CONTEXT.md:76-79`).
- Do **not** move the exit-code check below the `--no-ask-to-apply`
  early-return; the CONTEXT's "single early-return" wording
  means the flag is a guard around the exit-code check, not a
  wholesale skip of the post-report flow.

### Confidence summary

- **HIGH** — production code change (the `bool` var, the flag
  registration, the 3-line `if` check). Cobra's exit-code
  semantics via `RunE` → `Execute()` → `main.go` →
  `os.Exit(1)` are documented and observable in the current
  source. [VERIFIED: `main.go:11-14`; `cmd/root.go:32-48`;
  context7.com/spf13/cobra/llms.txt.]
- **HIGH** — fixture directory location and Go's
  `testdata/`-is-skipped-by-build behavior. Standard Go
  toolchain.
- **HIGH** — `TestRunParsesStructuredReport` already satisfies
  the "agent smoke test" requirement. [VERIFIED:
  `overlap_test.go:83-132`.]
- **HIGH** — `TestCheckOverlapUnsupportedToolSavesPrompt…` is
  the right template for the new command-level tests.
  [VERIFIED: `skill_overlap_test.go:274-394`.]
- **MEDIUM** — exact wording of the `pterm.Error` banner on
  overlap-exit. The behavior is observable but the planner
  should manually run `go run ./main.go skill check-overlap`
  once with a stubbed agent to confirm the message is
  acceptable. No code change is required regardless of
  outcome.

---

*Phase: 02-overlap-refactor*
*Research complete: 2026-06-11*
