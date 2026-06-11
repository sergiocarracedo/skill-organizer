# Plan 05 Summary

**Completed:** 2026-06-10

## What was built

Added two UX integration points around the check-security command (Plan 04):

1. **Re-enable gate** in `skill enable <path>`: if the skill's `risk-score >= 70` and `risk-evaluator != ""`, the user is shown the risk reason and asked to confirm before enabling. Default answer is **no** (cautious). On decline, `disabled: true` is explicitly written via the new `skills.SetDisabled` helper.
2. **Post-install hook** in `skill add <source>`: after a successful install + sync, the user is prompted *"Run check-security for `<name>`?"* with default = yes. On yes, `RunCheckSecurityForSkill` runs inline for that one skill. On no, `RiskEvaluator: ""` is written via `skills.UpdateManagedMetadata` to mark the skill unevaluated.

Unevaluated skills (empty `risk-evaluator`) bypass the re-enable gate entirely.

## Key files

- `packages/cli/cmd/enable.go`
  - New `enable.go` adds func vars `enableConfirm`, `enableSetDisabled`, `enableUpdateManagedFields` for test injection
  - Before calling `skills.RewriteManagedFields(skill, false, false)`, the new code reads the existing metadata and, if `risk-score >= 70 && risk-evaluator != ""`, prints a warning with the risk reason and prompts for confirmation (default=no)
  - On decline: calls `skills.SetDisabled(skill, true)` to ensure `disabled: true` is explicitly written
  - On accept or no-gate: calls `enableWithMetadataPreserved` (wraps `skills.SetDisabled(skill, false)`) to set disabled=false without disturbing risk fields
- `packages/cli/cmd/enable_test.go` — new file with 4 tests
  - `TestEnableHighRiskSkillPromptsConfirmation`: high-risk + decline → skill stays disabled
  - `TestEnableHighRiskSkillProceedsOnConfirmation`: high-risk + accept → skill is enabled AND risk fields survive
  - `TestEnableLowRiskSkillBypassesGate`: low-risk → confirm is never called
  - `TestEnableUnevaluatedSkillBypassesGate`: unevaluated (empty evaluator) → confirm is never called
- `packages/cli/cmd/skill_add.go`
  - New func vars `skillAddConfirmRunSecurity`, `skillAddRunSecurityForSkill`, `skillAddUpdateMetadata`
  - New `actuallyInstalled` slice tracks skills that were actually written to disk (skipped reinstalls are excluded)
  - After `printSyncResult`, a loop over `actuallyInstalled` prompts "Run check-security for `<name>`?" (default=yes)
  - On accept: invokes `skillAddRunSecurityForSkill(targetSkill, location)`
  - On decline: writes `{RiskEvaluator: "", RiskEvaluatedAt: <now>}` to mark the skill unevaluated
- `packages/cli/cmd/skill_add_test.go` — 2 new tests
  - `TestSkillAddHooksCheckSecurityPromptDecline`: confirms the prompt text, verifies the unevaluated marker is written
  - `TestSkillAddRunsSecurityOnAccept`: verifies `RunCheckSecurityForSkill` is called with the correct skill
  - `stubSkillAddDependencies` extended to stub the three new func vars (no-op defaults) so existing tests don't hang
- `packages/cli/cmd/skill_security.go`
  - New exported `RunCheckSecurityForSkill(skill, location)` helper for the skill-add hook
  - **Hook mode**: skips cost-acknowledgment prompt and auto-picks the first installed tool (per the plan, "no prompt in hook mode")
  - Reports `pterm.Warning` when no tools are detected and returns nil
- `packages/cli/internal/skills/frontmatter.go`
  - New `SetDisabled(skill, disabled)` helper updates ONLY the disabled flag (preserves all other fields, including risk fields)
  - Modified `mergeManagedMetadata`: only overwrites `RiskScore` when `updates.RiskScore > 0` OR `updates.RiskEvaluator != ""`. Prevents `RewriteManagedFields` and `UpdateManagedMetadata` calls with empty `ManagedMetadata{}` from clobbering an existing risk score.
- `packages/cli/e2e_test.go`
  - Updated `TestSkillAddAndCheckUpdatesBinary` to handle the new "Run check-security" prompt with a second `interactiveStep`

## Decisions made

- **Unevaluated skills bypass the re-enable gate.** Only `risk-evaluator != ""` AND `risk-score >= 70` triggers the gate. Empty `risk-evaluator` = "no data to warn about".
- **`skills.SetDisabled` is a new exported helper.** Required because the existing `RewriteManagedFields(skill, false, false)` flow was clobbering risk fields via `mergeManagedMetadata` (Plan 03's "always overwrite RiskScore" rule). The new helper reads existing metadata, sets only `disabled`, and re-writes — preserving all other fields.
- **Cost-ack prompt is skipped in `RunCheckSecurityForSkill`.** Per the plan, hook mode is lightweight. The full `check-security` command still has the cost-ack prompt for explicit runs.
- **Auto-pick first installed tool in `RunCheckSecurityForSkill`.** No interactive tool-selection prompt in the hook. If the user wants a specific tool, they can run `check-security` manually with `--tool`.
- **`actuallyInstalled` slice** ensures skipped reinstalls don't trigger the security hook. Otherwise the hook would prompt for skills that weren't actually imported, leading to a confusing UX (and would also hang the existing e2e test).
- **`mergeManagedMetadata` now skips zero-value `RiskScore`** when `updates.RiskEvaluator == ""` too. This is a deviation from the Plan 03 spec ("RiskScore is int. Always overwrite.") but is necessary to fix the regression where `RewriteManagedFields` with empty updates zeroed existing risk scores. The fix is heuristic (positive value OR non-empty evaluator signals "set") and works for all known callers. If a caller needs to explicitly reset to 0, they can do so by passing `RiskEvaluator: "unevaluated"`.
- **Plan threat analysis claim was wrong:** the plan said `RewriteManagedFields(skill, false, false)` "doesn't touch risk fields", but it actually did (via `mergeManagedMetadata` always overwriting `RiskScore`). Discovered this when `TestEnableHighRiskSkillProceedsOnConfirmation` failed with `Reloaded RiskScore = 0`. The fix preserves risk fields correctly.

## Notes for downstream

- The re-enable gate key is `risk-evaluator != ""`. The post-install hook's "no" path writes `risk-evaluator: ""` to mark the skill unevaluated. The next `check-security` run will pick it up.
- The security hook's "yes" path may run for a long time (one skill per agent call). The `check-security` command is also still available as a one-shot for the full set.
- `RunCheckSecurityForSkill` is exported and lives in the `cmd` package, so any future flow (e.g., a watch-mode hook on `check-updates`) can call it without duplicating logic.
- The skill-add hook's "yes" path returns errors via `pterm.Warning` rather than failing the whole install — this is intentional (a security check failure shouldn't block the install itself).
- `mergeManagedMetadata` heuristics: positive `RiskScore` OR non-empty `RiskEvaluator` signals "set". The agent's empty `RiskEvaluator` (unevaluated) path requires the caller to explicitly clear the score via a separate write if desired. For now, plan 05's "decline" path sets `RiskEvaluator: ""` via `UpdateManagedMetadata`, which goes through `mergeManagedMetadata` — but since `RiskScore` is also 0 in that update, the heuristic preserves the existing score. This is acceptable: the next `check-security` run will overwrite both atomically.
