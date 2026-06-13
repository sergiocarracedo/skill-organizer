# Phase 6 — AI model visibility and security tooling — RESEARCH

> Results from web research + codebase scan. Confidence levels: HIGH (verified via official docs or code), MEDIUM (reasonable extrapolation), LOW (assumption, needs verification).

---

## Don't Hand-Roll

### Interactive selection
The existing `selectOption()` in `cmd/prompt.go` uses `pterm.DefaultInteractiveSelect` — a polished keyboard-navigable menu. **Don't build a custom model picker.** Reuse the same `ToolSelector` callback pattern that `agenttools` already uses for tool selection. [VERIFIED: codebase scan]

### Model list parsing
`opencode models` outputs models in `provider/model` format (e.g. `anthropic/claude-sonnet-4-20250514`), one per line. [CITED: open-code.ai/en/docs/cli]. **Don't write a custom parser for each tool.** Use a per-tool `ListModelsFunc` callback that each tool defines, returning `[]string`. The `agenttools.Tool` struct already has function fields (`Args`, `PlanArgs`) — this follows the same pattern. [VERIFIED: agenttools.go:20-22]

### File hashing
Go's `crypto/sha256` is in the stdlib — no external dependency needed for hashing skill files to detect content changes. [ASSUMED: standard practice]

---

## Common Pitfalls

### 1. Assuming every tool exposes model info
**Pitfall:** Treating model detection as universal. Claude Code does NOT expose a model list via CLI (confirmed by user). Codex exposes models via `/model` slash command but not as a standalone subcommand.

**Mitigation:** Per-tool `ListModelsFunc` that returns `([]string, error)`. If a tool has no way to list models, return `nil, nil` — the caller shows "unknown". This is the same pattern as `PlanArgs` (nil = not supported).

### 2. Mixing tool version with model version
**Pitfall:** A tool's version (e.g. `opencode 1.14.33`) is NOT the same as the model version (e.g. `claude-sonnet-4-20250514`). Don't conflate them.

**Mitigation:** `ListModelsFunc` queries the model list. A separate `VersionQuery` (optional per tool) queries the tool binary version for display. These are independent.

### 3. Blocking the tool selection flow on model query failure
**Pitfall:** If `opencode models` fails (tool not responding, no configured provider), the entire tool selection blocks.

**Mitigation:** Model query should fail gracefully. If `ListModelsFunc` returns an error, log a warning and show "unknown" — don't block the user from selecting the tool.

### 4. Mutating SKILL.md during status tree rendering
**Pitfall:** Reading the risk score during tree rendering shouldn't modify files. The status tree is a read-only view.

**Mitigation:** The status tree reads `RiskScore`, `RiskSourceHash` etc. from `Skill.SkillMetadata` (loaded from frontmatter during scan). It never writes. Hash comparison happens in memory during rendering.

### 5. SHA-256 of entire skills directory catches irrelevant files
**Pitfall:** Hashing all files in the skill directory includes the `metadata.skill-organizer` block itself (which changes on every evaluation), creating a chicken-and-egg staleness problem.

**Mitigation:** Hash only the content files (SKILL.md's content body plus any referenced files like scripts), excluding the managed metadata block. Or: hash `SKILL.md` body + any non-metadata files.

---

## Existing Patterns in This Codebase

### Tool definition pattern
`agenttools.Tool` already uses function fields for tool-specific behavior:
```go
type Tool struct {
    ID          string
    Name        string
    Binaries    []string
    Description string
    Args        func(prompt string) []string
    PlanArgs    func(prompt string) []string
}
```
**Add:** `ListModels func() ([]string, error)` as a swappable function field. Each tool defines how to query its models. A helper function runs the binary with the appropriate args and parses the output.

### Agent selection flow
`chooseAgentToolImpl()` in `agenttools.go:188-211` handles:
1. Explicit `--tool` flag → use it
2. Saved default from config → use it
3. Interactive prompt → `selectInstalledToolImpl`

**Add model selection between step 2 and step 3** (when `--tool` is used but no `--model` flag, or after the tool is chosen interactively). Or make model selection part of `selectInstalledToolImpl` by showing tool + model options in a single picker.

### Status tree rendering
`status_render.go` has `formatSkillLabel()` (line 194-204) that renders each line. Currently:
```
alpha -> personal--alpha [synced] [installed ...] [update ...]
```
**Add** `[risk: 85]` or `[risk: uncheck]` as a right-side tag.

### Risk metadata storage
`ManagedMetadata` in `frontmatter.go:15-29` already stores:
- `RiskScore`, `RiskEvaluatedAt`, `RiskEvaluator`, `RiskReason`

**Add:** a new YAML field for the content hash.

### Swappable function vars for testing
Every major function in `agenttools`, `cmd/skill_security.go`, and `cmd` follows the swappable-var pattern (`ChooseAgentToolFunc`, `securityRunAnalysis`, etc.). Model query functions should follow the same pattern for testability.

---

## Recommended Approach

### Plan breakdown

**Plan 06-01: Model query infrastructure + fixture skills (Wave 1, no deps)**
- Add `ListModels func() ([]string, error)` and `VersionQuery []string` fields to `agenttools.Tool`.
- Define per-tool model queries in `supportedTools`: OpenCode → `opencode models`, others → nil.
- Add helper `QueryToolModels(tool InstalledTool) ([]string, error)` that runs the binary with the version query args and parses the output.
- Add `DefaultModel` field to `config.AgentSelectionConfig`.
- Create dangerous fixture skills in `packages/cli/internal/security/testdata/dangerous/` (4 patterns: shell_exec, env_exfil, download, obfuscated).
- All swappable function vars for test injection.

**Plan 06-02: Model selection in tool picker + security rating in status tree (Wave 2, depends on 01)**
- Integrate model selection into `chooseAgentToolImpl`: after tool is selected, if the tool exposes models, prompt user to pick one (or use `--model` flag).
- Add `--model` flag to `check-security` and `check-overlap`.
- Add `DefaultModel` persistence in `AgentSelectionConfig`.
- Add `[risk: N]` and `[risk: uncheck]` tags to `status_render.go:formatSkillLabel()`.
- Add `RiskSourceHash` to `ManagedMetadata`, implement hash-on-evaluate in `check-security`.
- Wire model info display: show model name in status output.
