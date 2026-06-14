# Plan 06-01 Summary

**Completed:** 2026-06-13
**Phase:** 6 — AI model visibility and security tooling

## What was built

Added model-query infrastructure to the `agenttools.Tool` struct so each tool can describe how to list its available models. Extended `AgentSelectionConfig` with `DefaultModel` (persisted) and `KnownModels` (runtime only) fields. Created 4 dangerous fixture SKILL.md files in `security/testdata/dangerous/` for regression testing the security evaluator.

## Key files

- `packages/cli/internal/agenttools/agenttools.go`: Tool struct now has `ListModels`, `ModelArgs`, `VersionArgs` fields; new `QueryToolModels` helper; OpenCode tool definition sets `ListModels` (runs `opencode models`, parses provider/model lines); all other tools leave `ListModels: nil`; package-level `execCommand` var for test injection
- `packages/cli/internal/agenttools/agenttools_test.go`: 3 new tests — nil ListModels returns nil/nil, fake OpenCode returns parsed model list, failed command returns error
- `packages/cli/internal/config/config.go`: `AgentSelectionConfig` gains `DefaultModel` (YAML persisted, omitempty) and `KnownModels` (runtime-only, `yaml:"-"`)
- `packages/cli/internal/config/agent_selection_test.go`: 2 new tests — `TestAgentSelectionConfigDefaultModelRoundTrip` (YAML round-trip), `TestAgentSelectionConfigKnownModelsNotPersisted` (runtime-only field)
- `packages/cli/internal/security/testdata/dangerous/shell_exec/SKILL.md` (new): dangerous shell injection pattern
- `packages/cli/internal/security/testdata/dangerous/env_exfil/SKILL.md` (new): env var exfiltration pattern
- `packages/cli/internal/security/testdata/dangerous/download/SKILL.md` (new): remote payload download pattern
- `packages/cli/internal/security/testdata/dangerous/obfuscated/SKILL.md` (new): base64-obfuscated command pattern

## Decisions made

- `KnownModels` uses `yaml:"-"` tag (not persisted) as specified in must-haves but not in the action code block — the must-haves are authoritative
- `execCommand` follows the existing `lookPath` pattern for test injection
- Tests use `printf` (not `echo`) to produce newline-separated model output for the fake OpenCode command
- Test file for AgentSelectionConfig tests is `agent_selection_test.go` (existing pattern), not `config_test.go` (which doesn't exist)

## Deviations from plan

- None

## Notes for downstream

- Plan 06-02 will use `QueryToolModels` and `DefaultModel` for the model-selection UI
- Plan 06-03 will use the dangerous fixtures for `check-security` evaluator tests
