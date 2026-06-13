---
phase: 6
status: passed
verified: 2026-06-13
---

# Phase 6: AI Model Visibility and Security Tooling — Verification

## Must-Have Results

### Plan 06-01: Tool model query infrastructure + config + fixture skills

| Plan | Must-Have | Status |
|------|-----------|--------|
| 06-01 | Tool struct has a `ListModels func(binary string) ([]string, error)` field | ✓ |
| 06-01 | Tool struct has a `ModelArgs func(model string, prompt string) []string` field | ✓ |
| 06-01 | Tool struct has a `VersionArgs []string` field | ✓ |
| 06-01 | OpenCode tool definition has `ListModels` set — runs `opencode models`, parses `provider/model` lines | ✓ |
| 06-01 | All other tools have `ListModels` set to `nil` | ✓ |
| 06-01 | `QueryToolModels(InstalledTool)` helper runs the binary with `ListModels` args and returns parsed models | ✓ |
| 06-01 | `QueryToolModels` returns `nil, nil` when `ListModels` is nil (graceful skip) | ✓ |
| 06-01 | `AgentSelectionConfig` has a `DefaultModel string` field | ✓ |
| 06-01 | `DefaultModel` round-trips through YAML serialization | ✓ |
| 06-01 | `AgentSelectionConfig` has a `KnownModels []string` field (populated by last query, not persisted) | ✓ |
| 06-01 | 4 dangerous fixture SKILL.md files exist in `security/testdata/dangerous/{pattern}/` | ✓ |
| 06-01 | Each fixture is a valid SKILL.md with name, description, and dangerous content matching its pattern | ✓ |
| 06-01 | `go build` succeeds, `go vet` is clean, `go test` passes with 0 failures | ✓ |
| 06-01 | `lefthook pre-commit` passes | ✓ |

### Plan 06-02: Model selection in tool picker + --model flag + persistence

| Plan | Must-Have | Status |
|------|-----------|--------|
| 06-02 | Model selection is integrated into the agent tool selection flow in `chooseAgentToolImpl` | ✓ |
| 06-02 | When a tool that exposes models is selected interactively, user can also pick a model from the list | ✓ |
| 06-02 | If `--model` flag is passed on `check-security` or `check-overlap`, model selection is skipped (use the flag value) | ✓ |
| 06-02 | Selected model is saved to `AgentSelectionConfig.DefaultModel` | ✓ |
| 06-02 | `telemetry status` shows the current default model (if set) | ✓ |
| 06-02 | `check-security --model <name>` uses the specified model when launching the agent | ✓ |
| 06-02 | `check-overlap --model <name>` uses the specified model when launching the agent | ✓ |
| 06-02 | If no model is available (tool doesn't expose models) and no `--model` flag, use empty string — no model info shown | ✓ |
| 06-02 | All swappable function vars for model query use the existing test-injection pattern | ✓ |
| 06-02 | `go build` succeeds, `go vet` is clean, `go test` passes with 0 failures | ✓ |
| 06-02 | `lefthook pre-commit` passes | ✓ |

### Plan 06-03: Security rating in status tree with content-hash freshness

| Plan | Must-Have | Status |
|------|-----------|--------|
| 06-03 | `ManagedMetadata` has a `RiskSourceHash string` field (YAML: `risk-source-hash`) | ✓ |
| 06-03 | `RiskSourceHash` is set during `check-security` evaluation (SHA-256 of skill file contents) | ✓ |
| 06-03 | `SkillStatus` struct carries `RiskScore`, `RiskSourceHash`, `RiskEvaluatedAt`, `RiskEvaluator` fields | ✓ |
| 06-03 | `status` tree shows `[risk: N]` tag for evaluated skills, colored by score | ✓ |
| 06-03 | `status` tree shows `[risk: uncheck]` in yellow for unevaluated skills (no `RiskEvaluator`) | ✓ |
| 06-03 | `status` tree shows `[risk: N (stale)]` when current file hash != stored `RiskSourceHash` | ✓ |
| 06-03 | Hash is computed over SKILL.md body + any non-metadata content files in the skill directory | ✓ |
| 06-03 | `go build` succeeds, `go vet` is clean, `go test` passes with 0 failures | ✓ |
| 06-03 | `lefthook pre-commit` passes | ✓ |

## Requirement Coverage

Phase 6 is a cross-cutting tooling phase (no new numbered REQ). It enhances:

| Req ID | Contribution | Status |
|--------|-------------|--------|
| REQ-4 | Dangerous fixture skills in `security/testdata/dangerous/` for regression testing | ✓ |
| REQ-4 | `check-security --model` flag for model selection | ✓ |
| REQ-4 | `RiskSourceHash` stored in `ManagedMetadata` during evaluation | ✓ |
| REQ-3 | `check-overlap --model` flag for model selection during overlap analysis | ✓ |
| REQ-4 | Risk score tags (`[risk: N]`, `[risk: uncheck]`, `[risk: N (stale)]`) in `status --tree` | ✓ |

## Integration Checks

| Import / Usage | Exists / Resolves | Status |
|---|---|---|
| `agenttools.QueryToolModels(tool)` → `queryToolModelsImpl` | Both defined in `agenttools.go` | ✓ |
| `agenttools.ChooseAgentTool(..., explicitModel)` | Signature updated, callers pass model | ✓ |
| `config.AgentSelectionConfig.DefaultModel` (YAML `default-model`) | Field exists, YAML round-trip tested | ✓ |
| `config.AgentSelectionConfig.KnownModels` (YAML `"-"`) | Field exists, never persisted (tested) | ✓ |
| `skills.ComputeSkillHash(skillDir)` → SHA-256 | Exists in `hash.go`, tested | ✓ |
| `skills.ManagedMetadata.RiskSourceHash` → YAML `risk-source-hash` | Field + marshal + unmarshal in `frontmatter.go` | ✓ |
| `status.SkillStatus.RiskScore` / `RiskEvaluatedAt` / `RiskEvaluator` / `RiskSourceHash` | All 4 fields in `status.go`, populated in `Build()` | ✓ |
| `cmd.formatRiskTag(entry)` → colored `[risk: N]` | Defined in `status_render.go`, called from `formatSkillLabel` | ✓ |
| `cmd.defaultSecurityRunAnalysis` → uses `ModelArgs` when model set | Defined in `skill_security.go` | ✓ |
| `cmd.defaultOverlapRunAnalysis` → uses `ModelArgs` when model set | Defined in `skill_overlap.go` | ✓ |
| `cmd.telemetry status` → third line `Default model:` | Reads `agentCfg.DefaultModel` in `telemetry.go` | ✓ |
| `agenttools.QueryToolModelsFunc` (swappable var) | Package-level var, tests override | ✓ |
| `agenttools.ChooseAgentToolFunc` (swappable var) | Package-level var, tests override | ✓ |

## Test Coverage

### Plan 06-01 tests verified
- `TestQueryToolModels_ReturnsNilWhenListModelsNil` — `agenttools_test.go`
- `TestQueryToolModels_ReturnsModelsForOpenCode` — `agenttools_test.go`
- `TestQueryToolModels_ReturnsErrorOnFailedCommand` — `agenttools_test.go`
- `TestAgentSelectionConfigDefaultModelRoundTrip` — `agent_selection_test.go`
- `TestAgentSelectionConfigKnownModelsNotPersisted` — `agent_selection_test.go`

### Plan 06-02 tests verified
- `TestChooseAgentTool_ExplicitModelFlag` — `agenttools_test.go`
- `TestChooseAgentTool_NoModelPromptWhenToolHasNoModels` — `agenttools_test.go`
- `TestChooseAgentTool_SelectsModelWhenToolExposesModels` — `agenttools_test.go`
- `TestChooseAgentTool_UsesDefaultModel` — `agenttools_test.go`
- `TestChooseAgentTool_ModelQueryErrorShowsToolWithoutModel` — `agenttools_test.go`
- `TestCheckSecurity_ModelFlagParsed` — `skill_security_test.go`
- `TestCheckOverlap_ModelFlagParsed` — `skill_overlap_test.go`

### Plan 06-03 tests verified
- `TestManagedMetadata_RiskSourceHashRoundTrip` — `frontmatter_test.go`
- `TestComputeSkillHash_Deterministic` — `hash_test.go`
- `TestComputeSkillHash_ChangesWhenContentChanges` — `hash_test.go`
- `TestComputeSkillHash_ExcludesMetadata` — `hash_test.go`
- `TestComputeSkillHash_IncludesExtraFiles` — `hash_test.go`
- `TestCheckSecurity_StoresRiskSourceHash` — `skill_security_test.go`
- `TestFormatSkillLabel_ShowsRiskForEvaluatedSkill` — `status_render_test.go`
- `TestFormatSkillLabel_ShowsUncheckForUnevaluated` — `status_render_test.go`
- `TestFormatSkillLabel_ShowsStaleWhenHashMismatch` — `status_render_test.go`

## Build & Static Analysis

- `go build ./...`: ✓ Success (no output)
- `go vet ./...`: ✓ Success (no output)
- `go test ./... -count=1`: ✓ 17 packages, all pass
- `lefthook run pre-commit --all-files --force`: ✓ Passed (cli-e2e run, 8.58s)

## Summary

**Score:** 34/34 must-haves verified

All automated checks passed. Phase goal achieved:

1. **AI tool model visibility** — Tool struct gains `ListModels`/`ModelArgs`/`VersionArgs`; OpenCode model query wired; `--model` flags on `check-security` and `check-overlap`; model selection flow in tool picker; default model saved to config and displayed in `telemetry status`.

2. **Dangerous fixture skills** — 4 curated dangerous SKILL.md fixtures under `security/testdata/dangerous/` (`shell_exec`, `env_exfil`, `download`, `obfuscated`) for security evaluator regression tests.

3. **Security rating in status tree** — `ManagedMetadata.RiskSourceHash` tracks content freshness; `SkillStatus` carries 4 risk fields; `status --tree` shows `[risk: N]` (color-coded by score), `[risk: uncheck]` (unevaluated, yellow), and `[risk: N (stale)]` (content changed, yellow).

All 34 must-haves across 3 plans verified. Build, vet, test suite (17 packages), and lefthook pre-commit all green.
