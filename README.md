# Skill Organizer

`skill-organizer` migrates the legacy shell-based organized-skills workflow into a Go CLI.

It keeps an organized source tree of skills in one folder, exposes a flat target folder of first-level symlinks for AI tools, and stores tool metadata in each source `SKILL.md`.

Agent-first setup and onboarding instructions are documented in [`AGENTS_README.md`](AGENTS_README.md).

Release automation and distribution notes are documented in [`docs/releasing.md`](docs/releasing.md).

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

## Migration From Legacy Scripts

Legacy scripts:

- `/home/sergio/.agents/scripts/sync-organized-skills.sh`
- `/home/sergio/.agents/scripts/watch-organized-skills.sh`

Equivalent CLI flow:

1. Create a project config with `skill-organizer project add`.
2. Run `skill-organizer sync` to replace the one-off sync script.
3. Register the config with `watched add` or choose watch registration during `project add`.
4. Run `skill-organizer watch` for foreground watching or `skill-organizer service install` plus `start` for background watching.

Behavior preserved from the shell implementation:

- terminal skill discovery stops at the first `SKILL.md` on a branch
- flattening uses `/ -> --`
- unmanaged target entries are not deleted by sync
- deleting a source skill removes the managed target symlink on the next sync
- deleting a managed target symlink only causes it to be recreated on the next sync
