# Plan 03 Summary

**Completed:** 2026-06-10

## What was built

Added the risk-score data model to `ManagedMetadata` so the check-security command (Plan 04) and the re-enable gate (Plan 05) can read and write per-skill risk assessments. Four new fields (`RiskScore`, `RiskEvaluatedAt`, `RiskEvaluator`, `RiskReason`) flow through the frontmatter read/write/merge helpers, and a new `updateManagedMetadata` helper lets callers apply partial updates without touching structural fields like `OriginalName` or `SourceRelativePath`.

A new `setInt` helper emits YAML integers (the existing `setScalar` always wrote strings, which quoted numeric values and broke round-trip type expectations).

## Key files

- `packages/cli/internal/skills/frontmatter.go`
  - `ManagedMetadata` struct: added `RiskScore int`, `RiskEvaluatedAt string`, `RiskEvaluator string`, `RiskReason string` after `LastUpdatedAt`
  - `ManagedMetadata()` reader: parses the four new `risk-*` keys (with `strconv.Atoi` for `risk-score`, defaulting to 0 on parse error)
  - `SetManagedFields` writer: always writes all four keys (the existing trim-check pattern was dropped intentionally to make unevaluated skills discoverable)
  - `mergeManagedMetadata`: always overwrites `RiskScore` from updates; trim-checks the three string fields
  - `updateManagedMetadata` helper: loads → reads existing metadata → merges → writes back without renaming
  - `setInt` helper: writes a YAML int scalar (with `!!int` tag)
- `packages/cli/internal/skills/frontmatter_test.go`
  - `TestDocumentManagedMetadataIncludesRiskScore`: parses frontmatter with all four risk fields, re-marshals, and verifies each key appears with the expected format
  - `TestMergeManagedMetadataPreservesRiskFields`: verifies the merge applies risk fields while leaving non-risk base fields intact
  - `TestUpdateManagedMetadataRoundTrip`: writes via the helper, reloads, and verifies on-disk values match (also confirms the skill is NOT renamed)
  - `TestManagedMetadataDefaultRiskScoreIsZero`: verifies frontmatter without risk fields parses with zero defaults

## Decisions made

- **Always-write `risk-score: 0` for unevaluated skills** (per plan). This makes the field discoverable and consistent across all skills rather than only the ones that have been analyzed. Trade-off: frontmatter grows by one line per skill. The flag for "unevaluated" is `risk-evaluator == ""`, not `risk-score == 0`.
- **`mergeManagedMetadata` always overwrites `RiskScore` from updates** (no trim-check possible for an int). Callers must pass the correct value. The `updateManagedMetadata` helper mitigates this by pre-reading existing metadata and re-writing only the relevant fields.
- **One-line free text for `risk-reason`** is the storage format. No length limit enforced at the schema level; UI/callers are expected to keep it short.
- **Default high-risk threshold = 70** (hardcoded in CONTEXT.md, referenced from Plan 04). Not stored in metadata; not configurable in P1.

## Notes for downstream

- Plan 04 (COMMAND) should call `updateManagedMetadata(skill, ManagedMetadata{RiskScore: …, RiskEvaluator: …, RiskEvaluatedAt: …, RiskReason: …})` from the analysis path. Do NOT call `RewriteManagedFieldsWithMetadata` — that helper clobbers `OriginalName`, `SourceRelativePath`, and `Disabled`.
- Plan 04 should also set `risk-evaluator` to a non-empty string (e.g. `claude-code`, `codex-1.2.3`) to mark a skill as evaluated. The re-enable gate in Plan 05 keys off this field (empty = unevaluated → bypass the gate).
- The `Skill` struct in `scanner.go` is intentionally NOT modified. Callers should read risk via `doc.ManagedMetadata()` on a loaded document, not via the `Skill` struct.
- The risk fields are always written by `SetManagedFields`, including in code paths that don't relate to security (e.g. `RewriteManagedFieldsWithMetadata`). This is by design: the schema is uniform. Callers that don't set the risk fields get `risk-score: 0` and empty strings.
