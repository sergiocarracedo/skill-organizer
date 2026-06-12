# Phase 6 — AI model visibility and security tooling — DISCUSSION-LOG

> Audit trail only. Downstream agents read CONTEXT.md, not this file.

**Gathered:** 2026-06-12
**Mode:** standard

---

## Model identification source

**Question:** How does the system determine which model a tool is using?

Options considered:
1. User-configured per tool (Recommended)
2. Hardcoded per tool in source
3. **Query the tool binary** ← SELECTED
4. Combination: config + hardcoded default

**Chosen:** Query the tool binary.
**Rationale:** Most accurate, reflects user's actual setup.

**Follow-up — fallback:**
- Show "unknown" when model can't be detected ← SELECTED
- Alternatives: hardcoded default, error + ask to configure

**Follow-up — query mechanism:**
- OpenCode: `opencode models` gives the list of models.
- Claude Code: no CLI model list — don't ask, use configured model.
- Other tools: check if they expose a model list; if not, skip.

**Chosen:** Per-tool detection using available model-list commands. If none, show only "unknown".

---

## Model display

**Question:** What exactly should be displayed?

Options considered:
1. Model name only (e.g. `claude-sonnet-4`)
2. **Name with provider prefix** ← SELECTED (e.g. `Anthropic/claude-sonnet-4`)
3. Tool name + model

**Chosen:** Provider prefix + model name.

---

## Model selection UX

**Question:** When should the model selector appear?

Options considered:
1. **During tool selection** ← SELECTED
2. `--model` flag on each command
3. Config-only, prompt on `--choose-model`
4. Dedicated `models` subcommand

**Chosen:** During tool selection. Models for the selected tool are shown alongside tool options. Also support `--model` flag for scripting.

**Follow-up — storage:**
- **Store in AgentSelectionConfig** ← SELECTED (new `default-model` field)
- Alternative: per-command flag only, no persistence.

---

## Security rating in status tree

**Question:** How should the security rating appear?

Options considered:
1. Emoji badge prefix (e.g. 🟢)
2. Text annotation `[score]`
3. **Risk tag in right info area** ← SELECTED
4. Color-coded skill name

**Chosen:** `[risk: 85]` tag alongside `[synced]`, `[installed ...]` in the right info area. Colored by score: green (0–29), yellow (30–69), red (70–100).

**Follow-up — unevaluated:**
- **Show `[risk: uncheck]` in yellow** ← SELECTED
- Alternatives: show nothing, show `[risk: ?]` in gray.

---

## Rating freshness

**Question:** How to handle stale risk scores?

Options considered:
1. **Show stored score + evaluation date** ← SELECTED
2. Staleness indicator after 30 days
3. Show score only, no staleness

**User added requirement:** Also store a hash of the skill files alongside the score. If files change, the score is invalidated.

**Chosen:** Store `RiskSourceHash` (hash of skill files at evaluation time). In status tree, compare current hash vs stored. If mismatch, score is shown as stale.

---

## Dangerous fixture skills

**Question:** What patterns should the test fixtures cover?

Options considered:
1. **Multiple fixtures, one pattern each** ← SELECTED
2. Single fixture with all patterns
3. One high-risk + one mid-risk

**Chosen:** 4 fixtures — shell_exec, env_exfil, download, obfuscated.

**Follow-up — location:**
- **Dedicated `security/testdata/dangerous/` dir** ← SELECTED
- Alternatives: shared overlap testdata, inline t.TempDir() only.

---

## Agent's Discretion

- Exact per-tool version-query commands and output format.
- Model list format parsing per tool.
- Hash algorithm for skill file content hash.

## Deferred Ideas

- Automatic periodic re-evaluation of stale scores.
- Model version in telemetry event schema.
- Remote model registries / APIs.
