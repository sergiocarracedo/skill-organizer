# Skill Organizer

`skill-organizer` lets you keep `SKILL.md` directories organized in nested folders while exposing the flat first-level skills folder that agent tools expect.

It uses an organized source tree such as `skills-organized/` as the source of truth and generates a flat target folder of symlinks such as `.agents/skills/`, `.claude/skills/`, or `.codex/skills/`. This makes it easy to separate your own skills, third-party skills, company skills, and experiments without manually copying folders around.

Agent-first setup and onboarding instructions are documented in [`AGENTS_README.md`](AGENTS_README.md).

Release automation and distribution notes are documented in [`docs/releasing.md`](docs/releasing.md).

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
```

## Commands

### Setup

```bash
skill-organizer onboard
skill-organizer project add
skill-organizer project edit
skill-organizer project remove
```

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
```

- `sync` scans the source tree, rewrites source skill frontmatter, creates or repairs managed symlinks in the target, removes stale managed symlinks, and updates the hidden target manifest.
- `status` reports source skills, flattened names, disabled skills, target drift, and unmanaged target entries.
- `skill enable` and `skill disable` update `metadata.skill-organizer.disabled` in the source `SKILL.md`.
- `skill move-unmanaged` previews moves from unmanaged target entries into the source tree and applies them after confirmation, or immediately with `--yes`.

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

Real `status` output after installing `asciinema-recorder`, moving it into `thirdparty/asciinema/asciinema-recorder`, and then disabling it:

```text
# Project

/home/sergio/.agents/.skill-organizer.yml
Source: /home/sergio/.agents/skills-organized
Target: /home/sergio/.agents/skills

# Skills

├─┬3rdparty
│ ├─┬agent-tools
│ │ ├──agent-browser -> 3rdparty--agent-tools--agent-browser [synced]
│ │ ├──mcp-builder -> 3rdparty--agent-tools--mcp-builder [synced]
│ │ └──skill-judge -> 3rdparty--agent-tools--skill-judge [synced]
│ ├──brainstorming -> 3rdparty--brainstorming [synced]
│ ├─┬coding
│ │ ├──ai-sdk -> 3rdparty--coding--ai-sdk [synced]
│ │ ├──allium -> 3rdparty--coding--allium [synced]
│ │ ├──create-pr -> 3rdparty--coding--create-pr [synced]
│ │ ├──frontend-design -> 3rdparty--coding--frontend-design [synced]
│ │ ├──mermaid-diagrams -> 3rdparty--coding--mermaid-diagrams [synced]
│ │ ├──nanostack -> 3rdparty--coding--nanostack [synced]
│ │ ├──pterm-go-cli -> 3rdparty--coding--pterm-go-cli [synced]
│ │ ├──requesting-code-review -> 3rdparty--coding--requesting-code-review [synced]
│ │ └──shaders-com -> 3rdparty--coding--shaders-com [synced]
│ ├─┬coral
│ │ ├──coral -> 3rdparty--coral--coral [synced]
│ │ └──coral-create-source-spec -> 3rdparty--coral--coral-create-source-spec [synced]
│ ├──hevy-api -> 3rdparty--hevy-api [synced]
│ ├─┬mattpocock
│ │ ├──design-an-interface -> 3rdparty--mattpocock--design-an-interface [synced]
│ │ ├──github-triage -> 3rdparty--mattpocock--github-triage [synced]
│ │ ├──grill-me -> 3rdparty--mattpocock--grill-me [synced]
│ │ ├──improve-codebase-architecture -> 3rdparty--mattpocock--improve-codebase-architecture [synced]
│ │ ├──prd-to-issues -> 3rdparty--mattpocock--prd-to-issues [synced]
│ │ ├──prd-to-plan -> 3rdparty--mattpocock--prd-to-plan [synced]
│ │ ├──qa -> 3rdparty--mattpocock--qa [synced]
│ │ ├──request-refactor-plan -> 3rdparty--mattpocock--request-refactor-plan [synced]
│ │ ├──tdd -> 3rdparty--mattpocock--tdd [synced]
│ │ ├──ubiquitous-language -> 3rdparty--mattpocock--ubiquitous-language [synced]
│ │ └──write-a-prd -> 3rdparty--mattpocock--write-a-prd [synced]
│ └─┬react
│   ├──react-useeffect -> 3rdparty--react--react-useeffect [synced]
│   └──vercel-react-best-practices -> 3rdparty--react--vercel-react-best-practices [synced]
├─┬personal
│ ├──agent-tasks -> personal--agent-tasks [synced]
│ ├─┬apis
│ │ ├──hevy-api -> personal--apis--hevy-api [synced]
│ │ └──home-assistant-api -> personal--apis--home-assistant-api [synced]
│ ├─┬coding
│ │ ├──frontend-project-bootstrap -> personal--coding--frontend-project-bootstrap [synced]
│ │ └──shaders-com -> personal--coding--shaders-com [synced]
│ ├─┬react
│ │ ├──react-component -> personal--react--react-component [synced]
│ │ └──react-project-bootstrap -> personal--react--react-project-bootstrap [synced]
│ ├──skill-creator -> personal--skill-creator [synced]
│ └──text-correction -> personal--text-correction [synced]
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
