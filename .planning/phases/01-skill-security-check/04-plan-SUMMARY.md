# Plan 04 Summary

**Completed:** 2026-06-10

## What was built

Implemented the `skill-organizer skill check-security` command end-to-end. It mirrors the existing `check-overlap` flow: resolve project config, collect enabled skills, build a security analysis prompt, detect installed agent tools, acknowledge external costs on first run, run the agent via spinner, parse the structured JSON response, and persist per-skill risk scores to SKILL.md frontmatter. When the score is >= 70 (hardcoded threshold), the user is prompted to disable the skill with default = yes; declining still writes the score. When no tools are detected (a new behavior, distinct from `check-overlap`'s hard error), the analysis prompt is printed to stdout and the command exits 0.

## Key files

- `packages/cli/internal/security/security.go` — new package
  - `SkillInfo` mirrors `overlap.SkillInfo` (name, path, description, flattened-name, disabled flag)
  - `SecurityReport` / `SkillResult` match the agent's JSON output schema (`results: [{name, risk-score, risk-reason}]`)
  - `BuildPrompt` instructs the agent to score each skill 0-100 with a one-line reason, with explicit risk categories (obfuscated text, binary files, dangerous instructions, undeclared side effects) and a 0-29/30-69/70-100 scoring guide
  - `ParseReport` strips code fences and clamps `risk-score` to [0, 100]
  - `Run` calls the (swappable) `commandRunner` and parses the result
  - `CollectSkills` reuses `skills.ScanSource` + `skills.LoadDocument` and filters disabled skills unless `includeDisabled` is set
  - `runCommand` (and `process_unix.go` / `process_windows.go`) duplicated from `internal/overlap` per the plan (a future cleanup can extract a shared `internal/executil`)
- `packages/cli/internal/security/security_test.go` — 6 tests: parse report, parse with code fences, build prompt, run with fake command, clamp high score, clamp negative score
- `packages/cli/cmd/skill_security.go` — new cobra command `check-security`
  - Mirrors `check-overlap` flow (resolve -> collect -> build -> detect -> cost ack -> spinner -> run -> parse)
  - `--print-prompt` flag previews the analysis prompt
  - No-tools-detected: prints prompt to stdout, returns nil
  - High-risk path: warns, then prompts "Disable skill <name>?" with default=yes
  - High-risk score is always written to metadata even when user declines disable
  - Reuses `agenttools.ChooseAgentTool`, `agenttools.StartSpinner`, `agenttools.ShowCursor` (from plan 02)
  - Uses `skills.UpdateManagedMetadata` (exported from plan 03) and `skills.RewriteManagedFields` for disable
- `packages/cli/cmd/skill.go` — registers `newCheckSecurityCommand()` under the `skill` subcommand
- `packages/cli/cmd/skill_security_test.go` — 5 tests: print-prompt bypass, no-tools-detected exit 0, low-risk score write, high-risk disable prompt, decline-keeps-score

## Decisions made

- **Exported `skills.UpdateManagedMetadata`** (was `updateManagedMetadata` lowercase). Required so the `cmd` package can use it as a test-injection function variable. The test infrastructure pattern (swappable func var per command) only works with exported symbols across packages.
- **High-risk threshold = 70**, hardcoded as `const highRiskThreshold = 70` at the top of `skill_security.go`. Not configurable in P1.
- **Risk evaluator field = tool.Tool.ID** (e.g. `claude-code`, `codex`). A future change could append a version, but P1 keeps it simple.
- **`risk-evaluator == ""` is the "unevaluated" flag** (not `risk-score == 0`). The re-enable gate in Plan 05 keys off this.
- **No-tools-detected: print prompt + exit 0**, not error. This is explicitly distinct from `check-overlap` (which hard-errors). The UX rationale: the user can still get value from the prompt to feed manually into an agent.
- **`MissingCount` for unmatched names.** If the agent returns a result for a skill that wasn't in the scanned list (race / out-of-band change), the command logs a warning and continues, rather than failing the whole run.
- **`runCommand` duplication.** Per the plan, the security package duplicates `runCommand` and the process-tree interrupt helpers from the `overlap` package. A future plan can extract them into a shared `internal/executil` (or move them to a top-level `internal/process`) without breaking the public surface. Marked as tech debt; not blocking.

## Notes for downstream

- Plan 05 (HOOKS) should key off `risk-evaluator != ""` to distinguish "evaluated" from "unevaluated" skills in the re-enable gate. Unevaluated skills bypass the gate entirely.
- Plan 05 should call `skills.UpdateManagedMetadata` (now exported) with `{RiskEvaluator: ""}` to mark a skill as unevaluated on the post-install hook's "no" path.
- The disable path in `check-security` calls `skills.RewriteManagedFields(skill, false, true)`, which sets `disabled: true` and preserves all other metadata. This is consistent with the disable command's behavior elsewhere.
- The security command's `securityWriteDisabled` function variable is wired to `skills.RewriteManagedFields` so tests can intercept the disable write. The hook in Plan 05 should NOT use this variable — it should call `skills.UpdateManagedMetadata` for the unevaluated flag.
- All `security*` function variables in `skill_security.go` follow the established pattern: original captured before test swaps, restored via `t.Cleanup`. Adding new injection points should preserve this pattern.
