# Phase 6 — AI model visibility and security tooling — CONTEXT

**Gathered:** 2026-06-12
**Mode:** standard
**Status:** Ready for planning

<domain>
## Phase Boundary

Three deliverables:
1. **AI tool model visibility + selection** — commands that invoke the AI tool (check-security, check-overlap) show which model is being used and let the user select a different model when the tool exposes available models.
2. **check-security fixture skills** — curated dangerous skill fixtures in testdata for deterministic regression testing of the security evaluator.
3. **Security rating in status tree** — each skill's risk score/rating appears in the `status --tree` output alongside state, version, and update info.

</domain>

<decisions>
## Implementation Decisions

### Model identification source
- Query the tool binary for available models. OpenCode exposes `opencode models`. For each tool, check if a `models` subcommand or similar exists.
- If a tool does NOT expose a model list via CLI, skip model selection entirely — don't ask the user. Use the stored or default model (or none).
- Fallback when model can't be detected: show `"unknown"`.

### Model display
- Show model name with provider prefix when available (e.g. `Anthropic/claude-sonnet-4-20250514`).

### Model selection UX
- Model selection happens during tool selection — when the user picks which AI tool to use, available models for that tool are also shown.
- `--model` flag on check-security / check-overlap to skip the interactive pick (for scripting).
- Selected model is stored in `AgentSelectionConfig` as a new `default-model` field, alongside `default-agent-tool`.

### Security rating in status tree
- Format: `[risk: 85]` tag in the right info area (alongside `[synced]`, `[installed ...]`).
- Color by score: green (0–29), yellow (30–69), red (70–100).
- Unevaluated skills (no risk score): show `[risk: uncheck]` in yellow.

### Rating freshness — file content hash
- Store a hash of the skill's files alongside the risk score and evaluation date in `ManagedMetadata`.
- If the files change (hash mismatch), the score is invalidated.
- In the status tree, show the score + date normally when hash matches; show as stale/invalid when hash differs.

### Dangerous fixture skills (for testing)
- Location: `packages/cli/internal/security/testdata/dangerous/` — dedicated directory.
- 4 fixtures, one pattern each:
  - `shell_exec/` — shell command injection patterns (e.g. `$(curl ...)`)
  - `env_exfil/` — environment variable exfiltration patterns
  - `download/` — hidden payload download patterns
  - `obfuscated/` — obfuscated base64 code patterns

### Agent's Discretion
- Exact per-tool version-query commands and output format — to be researched per tool during planning.
- Model list format parsing — varies by tool; planner chooses the most robust approach.
- Hash algorithm for skill file content — planner picks (e.g. SHA-256 of concatenated file contents).

</decisions>

<specifics>
## Specific Ideas

- `opencode models` is confirmed to list models. Claude Code does not expose model info via CLI — don't ask for it.
- The model selector should slot into the existing tool selection flow in `agenttools.ChooseAgentTool`.
- The status tree `formatSkillLabel()` function in `packages/cli/cmd/status_render.go` is the integration point for security tags.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `.planning/phases/01-skill-security-check-CONTEXT.md` — Phase 1 context (risk-score schema, threshold 70, ManagedMetadata fields)
- `.planning/REQUIREMENTS.md` — REQ-4 (skill security check) and REQ-10 (local-only telemetry) requirements
- `packages/cli/internal/agenttools/agenttools.go` — Tool struct, selection flow, supportedTools definitions
- `packages/cli/cmd/skill_security.go` — check-security command implementation
- `packages/cli/cmd/status_render.go` — status tree rendering
- `packages/cli/internal/skills/frontmatter.go` — ManagedMetadata struct and merge/update helpers
- `packages/cli/internal/config/config.go` — AgentSelectionConfig struct
- `packages/cli/internal/security/security.go` — CollectSkills, BuildPrompt, Run, ParseReport

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `agenttools.ChooseAgentTool` — the tool selection flow where model selection will be integrated.
- `agenttools.Supported()` / `DetectInstalled()` — tool registration and detection.
- `cmd/selectOption` — pterm-based interactive selector, reused for model picker.
- `skills.ManagedMetadata` — already has `RiskScore`, `RiskEvaluatedAt`, `RiskEvaluator`, `RiskReason`. New fields needed: `RiskSourceHash`.
- `cmd/status_render.go:formatSkillLabel()` — renders each skill line in the tree; integration point for risk tags.
- `cmd/status_render.go:buildStatusTreeLines()` — builds tree structure from `[]status.SkillStatus`.

### Established Patterns
- Tool detection: `exec.LookPath` in `agenttools.go`.
- Interactive selection: pterm `DefaultInteractiveSelect` callback via `selectOption`.
- Swappable function vars for testability (e.g. `ChooseAgentToolFunc`, `DetectInstalledFunc`).
- Risk score storage: inline in SKILL.md frontmatter under `metadata.skill-organizer.risk-score`.

### Integration Points
- `agenttools.InstalledTool` or `agenttools.Tool` struct needs a `Models` field or a `ModelsFunc` callback.
- `agenttools.chooseAgentToolImpl` — where model query + selection is injected into the selection flow.
- `config.AgentSelectionConfig` — needs `DefaultModel` field.
- `skills.ManagedMetadata` — needs `RiskSourceHash` field.
- `cmd/status_render.go:formatSkillLabel()` — needs risk-score + hash-check logic.
- `cmd/skill_security.go` — the `--model` flag added alongside `--tool`.

</code_context>

<deferred>
## Deferred Ideas

- Automatic periodic re-evaluation of stale scores — out of scope; manual `check-security` re-run is the path.
- Model version tracking in telemetry events — could be added later to the Event schema but not part of this phase.
- Support for remote model registries / APIs — this phase only covers local tool binary queries.

</deferred>

---

*Phase: 06-ai-model-visibility-security*
*Context gathered: 2026-06-12*
