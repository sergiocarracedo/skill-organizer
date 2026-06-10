---
wave: 2
depends_on: []
files_modified:
  - packages/cli/internal/skills/frontmatter.go
  - packages/cli/internal/skills/scanner.go
  - packages/cli/internal/skills/frontmatter_test.go
autonomous: true
single_layer_justified: "This plan is schema-only: adding risk-score fields to the data model. It is a legitimate single-layer step because the data model must exist before the check-security command (Plan 04) can write to it. No user-facing behavior changes, but verified by round-trip tests."
objective: "Add risk-score fields (risk-score, risk-evaluated-at, risk-evaluator, risk-reason) to ManagedMetadata struct, wire through SetManagedFields and mergeManagedMetadata, and add round-trip tests."
must_haves:
  - "go test ./internal/skills/... passes"
  - "TestDocumentManagedMetadataIncludesRiskScore round-trips risk-score=85"
  - "TestMergeManagedMetadataPreservesRiskFields merges risk-reason into existing metadata"
  - "A skill's frontmatter can carry risk-score, risk-evaluated-at, risk-evaluator, and risk-reason fields and survive a Marshal/ParseDocument round trip"
---

# Plan 03: Risk-score metadata schema

## Objective

Add four new fields to the `ManagedMetadata` struct — `RiskScore` (int 0-100), `RiskEvaluatedAt` (RFC3339 string), `RiskEvaluator` (string, empty = unevaluated), and `RiskReason` (one-line free text) — and wire them through `SetManagedFields`, `ManagedMetadata()`, and `mergeManagedMetadata`. The `Skill` struct in `scanner.go` is not modified directly (risk-score is metadata, not a scan concern). Round-trip tests verify the fields survive a write-read cycle.

## Context

These fields are stored in per-skill frontmatter under `metadata.skill-organizer.*` in SKILL.md files. They are read by the check-security command (Plan 04) and the enable gate (Plan 05). The `ManagedMetadata` struct and its helpers are in `internal/skills/frontmatter.go`.

The four YAML keys in the `skill-organizer` section map to:
- `risk-score` — int (0-100), stored as YAML scalar
- `risk-evaluated-at` — RFC3339 string, when the analysis was done
- `risk-evaluator` — tool ID string (e.g. `claude-code:1.2.3`); empty string means unevaluated
- `risk-reason` — one-line free text summary of why the score was assigned

Default threshold for high-risk is 70 (hardcoded for P1, not configurable yet).

## Tasks

<task id="metadata-01">
<name>Add risk-score fields to ManagedMetadata and wire through helpers</name>
<files>
  - packages/cli/internal/skills/frontmatter.go
</files>
<action>
In `frontmatter.go`:

1. Add four new fields to `ManagedMetadata` struct (after `LastUpdatedAt`):
   ```go
   RiskScore        int    // 0-100, 0 = unevaluated / no risk
   RiskEvaluatedAt  string // RFC3339 string, empty = not yet evaluated
   RiskEvaluator    string // agent tool ID that performed eval, empty = unevaluated
   RiskReason       string // one-line free-text reason
   ```

2. In `ManagedMetadata()` method (reads frontmatter → struct):
   After the `last-updated-at` block, add four new blocks:
   - `risk-score`: parse int with `strconv.Atoi(node.Value)`, or default 0
   - `risk-evaluated-at`: copy node value
   - `risk-evaluator`: copy node value
   - `risk-reason`: copy node value

3. In `SetManagedFields` method (writes struct → YAML node):
   After the `last-updated-at` block, add four new blocks using helpers:
   - `setScalar(organizerNode, "risk-score", strconv.Itoa(metadata.RiskScore))` — always write (even 0)
   - `setScalar(organizerNode, "risk-evaluated-at", metadata.RiskEvaluatedAt)` — always write
   - `setScalar(organizerNode, "risk-evaluator", metadata.RiskEvaluator)` — always write
   - `setScalar(organizerNode, "risk-reason", metadata.RiskReason)` — always write

4. In `mergeManagedMetadata` function:
   After the `LastUpdatedAt` block, add:
   - Always overwrite `target.RiskScore = updates.RiskScore`
   - If `strings.TrimSpace(updates.RiskEvaluatedAt) != ""`, set target
   - If `strings.TrimSpace(updates.RiskEvaluator) != ""`, set target
   - If `strings.TrimSpace(updates.RiskReason) != ""`, set target

5. Add `updateManagedMetadata` helper (used by Plan 04):
   ```go
   func updateManagedMetadata(skill Skill, updates ManagedMetadata) error {
       doc, err := LoadDocument(skill.SkillFile)
       if err != nil {
           return err
       }
       metadata := doc.ManagedMetadata()
       mergeManagedMetadata(&metadata, updates)
       doc.SetManagedFields(skill.FlattenedName, metadata, false)
       return doc.WriteTo(skill.SkillFile)
   }
   ```
   This selectively updates specific metadata fields without renaming the skill. Unlike `RewriteManagedFieldsWithMetadata`, it does NOT touch `OriginalName`, `SourceRelativePath`, `Disabled`, or other structural fields.
</action>
<verify>
- File compiles: `go build ./internal/skills/...`
</verify>
<done>[ ]</done>
</task>

<task id="metadata-02">
<name>Add round-trip tests for risk-score metadata</name>
<files>
  - packages/cli/internal/skills/frontmatter_test.go
</files>
<action>
Add to `frontmatter_test.go`:

1. `TestDocumentManagedMetadataIncludesRiskScore`:
   - Create a frontmatter with `risk-score: 85`, `risk-evaluated-at: "2026-06-10T12:00:00Z"`, `risk-evaluator: "claude-code"`, `risk-reason: "Contains shell execution patterns"`
   - Parse → set fields → marshal → assert each field appears in output
   - Mimics the existing `TestDocumentManagedFieldsPreserveExtraFrontmatter` pattern

2. `TestMergeManagedMetadataPreservesRiskFields`:
   - Create a base `ManagedMetadata` with source info only
   - Create an updates `ManagedMetadata` with `RiskScore: 42`, `RiskEvaluator: "opencode"`, `RiskReason: "Uses eval()"`
   - Call `mergeManagedMetadata(&base, updates)`
   - Assert `base.RiskScore == 42`, `base.RiskEvaluator == "opencode"`, `base.RiskReason == "Uses eval()"`

3. `TestUpdateManagedMetadataRoundTrip`:
   - Create a temp skill directory with SKILL.md
   - Call `updateManagedMetadata` with RiskScore=72, RiskEvaluator="claude"
   - Reload document, verify metadata matches

4. `TestManagedMetadataDefaultRiskScoreIsZero`:
   - Parse a frontmatter with NO risk-score fields
   - Assert `metadata.RiskScore == 0`, `metadata.RiskEvaluator == ""`
</action>
<verify>
- `go test ./internal/skills/... -run TestDocumentManagedMetadataIncludesRiskScore` passes
- `go test ./internal/skills/... -run TestMergeManagedMetadataPreservesRiskFields` passes
- `go test ./internal/skills/...` (all) passes
</verify>
<done>[ ]</done>
</task>

## Must-Haves

After all tasks complete, the following must be true:

- [ ] `go test ./internal/skills/...` passes
- [ ] `ManagedMetadata` has fields `RiskScore`, `RiskEvaluatedAt`, `RiskEvaluator`, `RiskReason`
- [ ] `SetManagedFields` writes `risk-score`, `risk-evaluated-at`, `risk-evaluator`, `risk-reason` yaml keys
- [ ] `ManagedMetadata()` reader parses those keys back into struct fields
- [ ] `mergeManagedMetadata` merges risk-score updates into existing metadata
- [ ] `updateManagedMetadata` helper exists and round-trips risk fields to disk

## Rollback Guide

If this plan fails:

1. Revert: `git checkout -- packages/cli/internal/skills/frontmatter.go packages/cli/internal/skills/frontmatter_test.go`
2. Verify: `go test ./internal/skills/...` passes
3. Restart with smaller scope (e.g., add fields to struct first, then helpers one at a time)

## Threat Analysis

| # | Threat | Likelihood | Impact | Mitigation |
|---|--------|-----------|--------|------------|
| 1 | `strconv.Atoi` panics on non-integer `risk-score` value in existing frontmatter | Low | Medium | Use `strconv.Atoi` with error check; on error, set to 0 (not panic). Existing skills won't have this field, so migration-safe. |
| 2 | `SetManagedFields` writes risk-score fields on every sync even when unevaluated, bloating frontmatter | Medium | Low | Always-writing risk-score=0 for unevaluated skills is intentional — it makes the field discoverable and consistent. No performance impact. |
| 3 | `mergeManagedMetadata` zeroes RiskScore when not provided in updates (value type, not pointer) | Low | High | `mergeManagedMetadata` always overwrites `RiskScore` from updates. Callers must pass the correct value. The `updateManagedMetadata` helper solves this by pre-reading existing metadata. |

## Commit Message

```
feat(cli): add risk-score fields to ManagedMetadata schema

- Add RiskScore, RiskEvaluatedAt, RiskEvaluator, RiskReason to ManagedMetadata
- Wire through SetManagedFields writer and ManagedMetadata() reader
- Wire through mergeManagedMetadata for partial updates
- Add updateManagedMetadata helper for selective field updates
- Add round-trip tests verifying read/write/merge behavior
```
