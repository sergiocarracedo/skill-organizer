# Skill Organizer

`skill-organizer` lets you keep `SKILL.md` directories organized in nested folders while exposing the flat first-level skills folder that agent tools expect.

It uses an organized source tree such as `skills-organized/` as the source of truth and generates a flat target folder of symlinks such as `.agents/skills/`, `.claude/skills/`, or `.codex/skills/`. This makes it easy to separate your own skills, third-party skills, company skills, and experiments without manually copying folders around.

Agent-first setup and onboarding instructions are documented in [`AGENTS_README.md`](AGENTS_README.md).

Release automation and distribution notes are documented in [`docs/releasing.md`](docs/releasing.md).

[![asciicast](https://asciinema.org/a/o2D10e4PL6Qb6JvQ.svg)](https://asciinema.org/a/o2D10e4PL6Qb6JvQ)

## Table Of Contents

- [Getting Started](#getting-started)
- [Why](#why)
- [Organized Source To Flat Target](#organized-source-to-flat-target)
- [How It Works](#how-it-works)
- [Layout](#layout)
- [Commands](#commands)
- [Shell Completion](#shell-completion)
- [Disable Skills Without Deleting Them](#disable-skills-without-deleting-them)
- [Example Demo Flow](#example-demo-flow)
- [Example Status Output](#example-status-output)

## Getting Started

Install `skill-organizer` using one of the supported distribution channels:

```bash
npm i -g skill-organizer
brew tap sergiocarracedo/tap
brew install skill-organizer
```

If you prefer a direct binary download, use the GitHub Releases page:

- https://github.com/sergiocarracedo/skill-organizer/releases

Then verify the CLI is available:

```bash
skill-organizer --version
```

For a first-time setup, use the interactive onboarding flow:

```bash
skill-organizer onboard
skill-organizer status
```

If you want agent-oriented setup guidance for `.agents`, Claude Code, Codex, or Antigravity, see [`AGENTS_README.md`](AGENTS_README.md).

## Why

Most agent tools only read skill folders at the first level of their skills directory. That works for a few skills, but it becomes messy once you want to group them by source or topic.

`skill-organizer` solves that by keeping two views of the same skills:

- an organized source tree that you edit directly
- a flat generated target folder that your tools can read

The target folder is generated from the source tree using flattened names and symlinks, so your real skills stay organized and the tool-facing folder stays compatible.

## Organized Source To Flat Target

Example source tree:

```text
organized-skills/
├── generic
├── starter
├── my-skills/
│   ├── coding-skills/
│   │   ├── astro-performance-auditor/
│   │   └── typescript-error-explainer/
│   ├── text-skills/
│   │   ├── newsletter-copy-editor/
│   │   └── writing-style-harmonizer/
│   └── personal-workflows/
│       └── weekly-review-assistant/
├── company-skills/
│   ├── coding-skills/
│   │   ├── internal-api-checklist/
│   │   └── release-train-coordinator/
│   ├── compliance/
│   │   └── pii-review-helper/
│   └── onboarding/
│       └── engineering-ramp-up-guide/
├── community-skills/
│   ├── frontend/
│   │   ├── design-token-curator/
│   │   └── visual-regression-triager/
│   └── content/
│       └── markdown-link-fixer/
└── experimental/
  └── research/
    └── prompt-pattern-lab/
```

Generated target folder:

```text
.agents/skills/
├── my-skills--coding--skills--astro-performance-auditor/
├── my-skills--coding--skills--typescript-error-explainer/
├── my-skills--text--skills--newsletter-copy-editor/
├── my-skills--text--skills--writing-style-harmonizer/
├── my-skills--personal--workflows--weekly-review-assistant/
├── company-skills--coding--skills--internal-api-checklist/
├── company-skills--coding--skills--release-train-coordinator/
├── company-skills--compliance--pii-review-helper/
├── company-skills--onboarding--engineering-ramp-up-guide/
├── community-skills--frontend--design-token-curator/
├── community-skills--frontend--visual-regression-triager/
├── community-skills--content--markdown-link-fixer/
├── experimental--research--prompt-pattern-lab/
├── IMPORTANT.md
```

Flattening only replaces `/` with `--`. Existing single `-` characters are preserved.

The generated target folder is not where you should edit real skills. It is a generated view built from the source tree and typically contains symlinks plus a visible `IMPORTANT.md` notice to make that clear.

## How It Works

- Any directory containing `SKILL.md` is treated as a skill.
- Once a directory contains `SKILL.md`, it is treated as a terminal skill and child folders are not scanned.
- `sync` flattens the source-relative path into a first-level target name by replacing `/` with `--`.
- Generated target entries are symlinks pointing back to the real source skill directory.
- Unmanaged entries already present in the target folder are not deleted automatically.
- `status` shows managed skills, disabled skills, drift, broken links, and unmanaged target entries.

## Layout

For a target such as:

```text
/repo/.agents/skills
```

the default source is:

```text
/repo/.agents/skills-organized
```

and the per-project config lives at:

```text
/repo/.agents/.skill-organizer.yml
```

with contents like:

```yaml
source: /repo/.agents/skills-organized
target: /repo/.agents/skills
```

The home watch registry lives at:

```text
~/.config/skill-organizer/skill-organizer.yml
```

and stores watched project config paths and service settings:

```yaml
watched:
  - /repo/.agents/.skill-organizer.yml

service:
  log-level: info

overlap:
  default-agent-tool: claude
  acknowledged-external-tool-costs: true
```

## Commands

### Setup

```bash
skill-organizer completion bash
skill-organizer completion zsh
skill-organizer completion fish
skill-organizer completion powershell
skill-organizer onboard
skill-organizer project add
skill-organizer project edit
skill-organizer project remove
```

- `completion` prints shell-completion scripts for bash, zsh, fish, and PowerShell so you can wire command and flag completion into your shell profile.
- `onboard` guides first-time global setup for supported tools such as generic `.agents` setups, Claude Code, Codex, and Antigravity. It creates the project config, can move existing target skills into `skills-organized`, can register the project for watching, optionally installs and starts the background service, and finishes by showing `status`.
- `project add` interactively chooses a target skills folder, proposes the sibling `skills-organized` source, writes the project config, and can register it for watching.
- `project edit` updates the active project config discovered from `--config` or the nearest `.skill-organizer.yml`.
- `project remove` deletes the active project config and unregisters it from the watch registry if present.

### Sync

```bash
skill-organizer sync
skill-organizer status
skill-organizer skill enable <source-path>
skill-organizer skill disable <source-path>
skill-organizer skill move-unmanaged
skill-organizer skill check-overlap
```

- `sync` scans the source tree, rewrites source skill frontmatter, creates or repairs managed symlinks in the target, removes stale managed symlinks, and updates the hidden target manifest.
- `status` reports source skills, flattened names, disabled skills, target drift, and unmanaged target entries.
- `skill enable` and `skill disable` update `metadata.skill-organizer.disabled` in the source `SKILL.md`.
- `skill move-unmanaged` previews moves from unmanaged target entries into the source tree and applies them after confirmation, or immediately with `--yes`.
- `skill check-overlap` runs an installed agent CLI to review the current project skills and report likely overlap or duplication. By default it analyzes enabled skills only and shows only `partial` and `duplicate` overlap groups. Use `--include-disabled` to include disabled skills, `--choose-tool` to pick a different installed tool on the next run, `--tool <id>` to choose one explicitly, `--min-overlap-type` to include weaker matches, or `--no-ask-to-apply` to skip the follow-up planning prompt.
- `skill check-overlap --print-prompt` prints the generated analysis prompt without invoking any external CLI. This bypasses tool selection and the one-time cost notice.

### Overlap Analysis

```bash
skill-organizer skill check-overlap
skill-organizer skill check-overlap --choose-tool
skill-organizer skill check-overlap --tool codex
skill-organizer skill check-overlap --include-disabled
skill-organizer skill check-overlap --min-overlap-type adjacent
skill-organizer skill check-overlap --min-overlap-type 1
skill-organizer skill check-overlap --no-ask-to-apply
skill-organizer skill check-overlap --print-prompt
```

On first use, `skill check-overlap` detects installed agent tools such as Claude Code, Codex, OpenCode, Cursor, and Antigravity, then asks which one to use. The selected tool is saved in the global app config and reused on later runs unless you pass `--choose-tool`.

Before the first direct invocation, the CLI shows a one-time notice that the selected external agent tool may use a paid account, API credits, or other metered usage depending on your setup. That acknowledgment is persisted in the same global config file.

When the CLI invokes an external tool, it shows a spinner and updates it with any intermediate status lines emitted by that tool. The final overlap report is rendered with wrapped output, colored labels, colored skill names, and overlap scores from `0` to `100` where higher scores indicate stronger overlap.

After printing the report, the CLI branches based on the selected tool. When the tool supports verified interactive plan mode, the CLI asks whether it should open that tool in plan mode to prepare a plan for applying the recommendations, then warns that the user should review the worktree and consider creating a backup or commit first. The tool is opened with a plan-only prompt that asks it not to modify files or execute changes. For tools without a verified interactive plan-mode launch path, the CLI emits a capability warning, asks whether it should generate a prompt to apply the recommendations, saves that prompt to `plans/skill-overlap-fix-[YYYYDDMM]-[HHmmss].md`, and prints the absolute path.

`--min-overlap-type` accepts either text or numbers:

- `adjacent` or `1`
- `partial` or `2`
- `duplicate` or `3`

The default is `partial`, which hides `adjacent` groups unless you ask for them explicitly.

## Shell Completion

Generate shell completion scripts with:

```bash
skill-organizer completion bash
skill-organizer completion zsh
skill-organizer completion fish
skill-organizer completion powershell
```

Common usage examples:

```bash
skill-organizer completion bash > ~/.local/share/bash-completion/completions/skill-organizer
skill-organizer completion zsh > ~/.zsh/completions/_skill-organizer
skill-organizer completion fish > ~/.config/fish/completions/skill-organizer.fish
skill-organizer completion powershell > skill-organizer.ps1
```

Use `--no-descriptions` on any shell subcommand when you want a shorter completion script without command descriptions.

## Disable Skills Without Deleting Them

You can disable a skill without removing it from the organized source tree:

```bash
skill-organizer skill disable my-skills/coding-skills/astro-performance-auditor
```

This keeps the source folder in `skills-organized/`, marks the skill as disabled in its `SKILL.md`, and removes its generated target entry on the next sync.

Re-enable it later with:

```bash
skill-organizer skill enable my-skills/coding-skills/astro-performance-auditor
```

This is useful when you want to temporarily hide a skill from your agent tools without deleting the real files.

## Example Demo Flow

This is a good terminal demo flow for an asciinema recording:

```bash
cd ~/.agents

npx skills add https://github.com/terrylica/cc-skills --skill asciinema-recorder

skill-organizer status --config ~/.agents/.skill-organizer.yml

skill-organizer skill move-unmanaged --config ~/.agents/.skill-organizer.yml
```

Move `asciinema-recorder` into:

```text
thirdparty/asciinema/asciinema-recorder
```

Then show:

- the source now lives under `~/.agents/skills-organized/thirdparty/asciinema/asciinema-recorder`
- the flat target contains the generated entry
- `IMPORTANT.md` is still present in the generated folder
- `skill-organizer status` is clean after the move

Recorded example:

- Asciinema: https://asciinema.org/a/o2D10e4PL6Qb6JvQ

## Example Status Output

Relevant `status` output after installing `asciinema-recorder`, moving it into `thirdparty/asciinema/asciinema-recorder`, and then disabling it:

```text
# Project

/home/sergio/.agents/.skill-organizer.yml
Source: /home/sergio/.agents/skills-organized
Target: /home/sergio/.agents/skills

# Skills

└─┬thirdparty
  └─┬asciinema
    └──asciinema-recorder -> thirdparty--asciinema--asciinema-recorder [disabled]


# Unmanaged target entries

None

# Summary

Total skills:     39
Managed skills:   38
Unmanaged skills: 0
Synced:           38
Disabled:         1
Missing target:   0
Broken link:      0
Drifted:          0
```

### Watch Registry

```bash
skill-organizer watched list
skill-organizer watched add /path/to/.skill-organizer.yml
skill-organizer watched remove
```

- `watched list` shows watched project config paths.
- `watched add` validates a project config path and registers it.
- `watched remove` accepts a path or lets you choose one interactively.

### Foreground Watch

```bash
skill-organizer watch
```

`watch` reads the home watch registry, watches the registered project config files and their source/target trees, and reruns sync only for affected projects.

### Background Service

```bash
skill-organizer service install
skill-organizer service start
skill-organizer service stop
skill-organizer service restart
skill-organizer service status
skill-organizer service uninstall
skill-organizer service log-level
skill-organizer service log-level set debug
```

The service uses `kardianos/service` and the home watch registry as its source of watched projects.

Service log verbosity is stored in the same global config file under `service.log-level`.
Supported levels are `error`, `warn`, `info`, and `debug`.

### Service Logs

The background service writes to the system logging backend.

On Linux user services, logs are available through `journalctl`:

```bash
journalctl --user -u skill-organizer.service
journalctl --user -u skill-organizer.service -f
```

If you change the service log level, restart the service to apply it:

```bash
skill-organizer service log-level set debug
skill-organizer service restart
```

## Skill Metadata

During sync, the CLI updates source `SKILL.md` frontmatter:

```yaml
metadata:
  skill-organizer:
    original-name: example
    source-relative-path: personal/example
    disabled: false
```

The top-level `name` is rewritten during `sync` so it matches the flattened folder name used in the target.

Other frontmatter fields are preserved.

## Sync Behavior

Behavior guarantees:

- terminal skill discovery stops at the first `SKILL.md` on a branch
- flattening uses `/ -> --`
- unmanaged target entries are not deleted by sync
- deleting a source skill removes the managed target symlink on the next sync
- deleting a managed target symlink only causes it to be recreated on the next sync
