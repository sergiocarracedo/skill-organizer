# Plan 06-03 Summary

**Completed:** 2026-06-13
**Phase:** 6 — AI model visibility and security tooling

## What was built

Added security risk score display to the `status --tree` output with content-hash freshness tracking. Each skill line shows a `[risk: N]` tag colored by score, `[risk: uncheck]` for unevaluated skills, or `[risk: N (stale)]` when the skill file content has changed since the last evaluation. A `ComputeSkillHash` helper computes SHA-256 of skill content (excluding metadata frontmatter) and is stored alongside the risk score during `check-security`.

## Key files

- `packages/cli/internal/skills/frontmatter.go`: `ManagedMetadata` gains `RiskSourceHash` field with YAML read/write/merge
- `packages/cli/internal/skills/hash.go` (new): `ComputeSkillHash(skillDir)` — SHA-256 over SKILL.md body + non-metadata files sorted by name
- `packages/cli/cmd/skill_security.go`: Both the main command loop and `RunCheckSecurityForSkill` compute and store `RiskSourceHash` during evaluation
- `packages/cli/internal/status/status.go`: `SkillStatus` gains `RiskScore`, `RiskEvaluatedAt`, `RiskEvaluator`, `RiskSourceHash`; populated from metadata in `Build()`
- `packages/cli/cmd/status_render.go`: `formatRiskTag()` renders colored risk tags; `formatSkillLabel()` appends them after existing status info

## Tests added/changed

| Test | File | What it covers |
|------|------|----------------|
| `TestManagedMetadata_RiskSourceHashRoundTrip` | `frontmatter_test.go` | YAML round-trip of RiskSourceHash |
| `TestComputeSkillHash_Deterministic` | `hash_test.go` | Same dir → same hash |
| `TestComputeSkillHash_ChangesWhenContentChanges` | `hash_test.go` | Content change → hash change |
| `TestComputeSkillHash_ExcludesMetadata` | `hash_test.go` | Adding risk-score to metadata doesn't change hash |
| `TestComputeSkillHash_IncludesExtraFiles` | `hash_test.go` | Adding a README.md changes the hash |
| `TestCheckSecurity_StoresRiskSourceHash` | `skill_security_test.go` | End-to-end: real skill file → hash stored in metadata |
| `TestFormatSkillLabel_ShowsRiskForEvaluatedSkill` | `status_render_test.go` | Risk tag shown with score |
| `TestFormatSkillLabel_ShowsUncheckForUnevaluated` | `status_render_test.go` | Unevaluated shows `[risk: uncheck]` |
| `TestFormatSkillLabel_ShowsStaleWhenHashMismatch` | `status_render_test.go` | Hash mismatch shows `(stale)` |

## Decisions made

- **Hash ignores frontmatter metadata** — `ComputeSkillHash` uses `LoadDocument` to parse SKILL.md and hashes only the body. This avoids circular dependency (metadata contains the hash itself).
- **Non-SKILL.md files hashed as-is** — any file in the skill directory (README.md, configs, etc.) is included in the hash by full content.
- **Hash computation on every status run** — acceptable performance trade-off since it's local file I/O on a small number of files.
- **Stale detection is best-effort** — if `ComputeSkillHash` fails (e.g., dir doesn't exist), the hash comparison is skipped and no stale tag is shown.

## Deviations from plan

- **Existing tests show warnings** — `TestCheckSecurityStoresRiskScoreOnLowRisk` and related tests use non-existent fixture paths, causing `ComputeSkillHash` to fail with a warning. This is expected and harmless (all tests pass).

## Notes for downstream

- The `formatRiskTag` function depends on `skills.ComputeSkillHash` being importable from the `cmd` package.
- The stale detection computes the current hash on every `status` invocation. If this becomes a performance concern, the hash could be cached per status run.
