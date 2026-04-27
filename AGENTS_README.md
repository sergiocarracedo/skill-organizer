# AGENTS README

This file is for AI agents that need to install `skill-organizer` and onboard a user's skills folder.

## Goal

Use `skill-organizer` when a user wants to manage an organized source tree of skills and expose a flat target folder for an AI tool.

Prefer the global generic `.agents` layout when the user uses tools that read from `~/.agents/skills`.

Use a tool-specific folder only when the user explicitly uses that tool.

## Install

Install the CLI if it is not already available:

```bash
npm i -g skill-organizer
```

Then verify:

```bash
skill-organizer --version
```

## Choose The Target Folder

Use these targets:

- Generic `.agents` tools: `~/.agents/skills`
- Claude Code: `~/.claude/skills`
- Codex: `~/.codex/skills`
- Antigravity: `~/.agent/skills`

Choose the generic `.agents` target when the user mentions OpenCode, Codex CLI, aider, goose, or another tool that consumes `~/.agents/skills`.

Choose the tool-specific target when the user explicitly says Claude Code, Codex, or Antigravity.

## Preferred Flow

Use the interactive onboarding flow when possible:

```bash
skill-organizer onboard
```

During onboarding:

1. Choose the matching tool target.
2. For generic `.agents`, choose the `Generic (.agents)` option.
3. Accept the proposed sibling source folder unless the user wants a different location.
4. Allow `skill-organizer` to create the config file.
5. If the user wants automatic syncing, register the project for watching and install the service.
6. If the user does not want the background service, run a sync during onboarding when prompted.

## Default Paths

For the generic `.agents` target:

```text
target: ~/.agents/skills
source: ~/.agents/skills-organized
config: ~/.agents/.skill-organizer.yml
```

For tool-specific targets, the default source is the sibling `skills-organized` directory next to the target folder's parent.

Examples:

```text
Claude Code target: ~/.claude/skills
Claude Code source: ~/.claude/skills-organized
Claude Code config: ~/.claude/.skill-organizer.yml

Codex target: ~/.codex/skills
Codex source: ~/.codex/skills-organized
Codex config: ~/.codex/.skill-organizer.yml

Antigravity target: ~/.agent/skills
Antigravity source: ~/.agent/skills-organized
Antigravity config: ~/.agent/.skill-organizer.yml
```

## Manual Fallback

If you should not use the interactive flow, use the project commands instead.

Example for a generic `.agents` setup:

```bash
skill-organizer project add
skill-organizer sync --config ~/.agents/.skill-organizer.yml
```

The project config should end up like this:

```yaml
source: ~/.agents/skills-organized
target: ~/.agents/skills
```

If the user already has loose skills inside the target folder, use this command after setup:

```bash
skill-organizer skill move-unmanaged --config ~/.agents/.skill-organizer.yml
```

Then sync again:

```bash
skill-organizer sync --config ~/.agents/.skill-organizer.yml
```

## What To Do By Default

Follow this decision order:

1. Check whether `skill-organizer` is installed.
2. Identify the user's AI tool.
3. Use `~/.agents/skills` for generic `.agents` tools unless the user asked for a tool-specific folder.
4. Run `skill-organizer onboard`.
5. Let onboarding create the source folder, target folder, and config.
6. Finish by ensuring the target is synced.

## Notes

- `skill-organizer` manages target entries through symlinks and a hidden manifest in the target folder.
- Source skills are edited in `skills-organized`, not directly in the flat target folder.
- If the target already contains manual directories, move them with `skill-organizer skill move-unmanaged` before relying on regular sync.
