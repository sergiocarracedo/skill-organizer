## Context

The repository currently contains only OpenSpec scaffolding while the real behavior lives in external shell scripts. The existing sync script walks an organized skills tree, treats the first directory containing `SKILL.md` on each branch as a terminal skill, flattens its relative path by replacing `/` with `--`, creates top-level symlinks in the target skills folder, and removes stale managed symlinks without touching unmanaged entries.

The new CLI must preserve that behavior while adding source metadata rewriting, skill toggling, interactive configuration, watcher orchestration, and service installation. The user also wants the CLI to live in `cli/` so a future `web/` app can share the repository without coupling to the command-line module.

## Goals / Non-Goals

**Goals:**
- Preserve the existing flattening and managed symlink semantics.
- Make source `SKILL.md` the durable place for skill-organizer metadata.
- Support multiple managed projects through one config file per target parent directory.
- Make setup and management flows interactive and friendly with PTerm while keeping core commands scriptable.
- Support foreground watching and a background service driven by a home watch registry.

**Non-Goals:**
- Building the future `web/` app.
- Replacing symlinks with copied target folders.
- Supporting multiple sync definitions inside one per-project config file.
- Exposing the target manifest filename as user-configurable behavior.
- Implementing cross-platform log tailing in the first delivery.

## Decisions

### Keep symlink targets and rewrite source metadata

The target folder remains a flat set of top-level symlinks, matching the current shell workflow and avoiding content duplication. To satisfy the new requirement that the active skill name match the flattened folder name, the sync engine rewrites source `SKILL.md` frontmatter so `name` equals the flattened name while storing `original-name`, `source-relative-path`, and `disabled` under `metadata.skill-organizer`.

Alternative considered: copy full skill folders into target and rewrite a generated `SKILL.md`. Rejected because it would diverge from current behavior, complicate bundled-resource synchronization, and make manual target edits ambiguous.

### Preserve unknown frontmatter fields

Real skills already contain fields beyond `name` and `description`, such as `version` and `auto_trigger`. The frontmatter layer must preserve all unrelated fields and only edit `name` plus `metadata.skill-organizer`.

Alternative considered: normalize frontmatter to a narrow schema. Rejected because it would break existing skill metadata and create unnecessary churn.

### One config per managed project

Each managed project stores a single config file at `<target-parent>/.skill-organizer.yml` with `source` and `target`. Commands resolve the active config from `--config`, otherwise by walking upward from the current directory to the nearest `.skill-organizer.yml`.

Alternative considered: one global config containing many sync definitions. Rejected because the user wants setup to start from a target folder and treat each configured project independently.

### Separate watch registry from project config

The home file `~/.config/skill-organizer/skill-organizer.yml` stores only watched config paths. Watch execution and background service startup read this registry, then load each project config to determine source and target paths.

Alternative considered: storing full source and target values again in the home registry. Rejected because it duplicates configuration and risks drift.

### Target manifest is internal

The sync engine writes a hidden internal manifest in the target root to record managed entries and support drift detection. The manifest filename remains an internal implementation detail.

Alternative considered: user-visible or configurable manifest naming. Rejected to keep the configuration surface minimal.

### Watch both source and target, but suppress self-generated managed churn

Watching the source covers the canonical content changes, while watching the target allows the tool to surface unmanaged additions and detect manual tampering. The watcher must debounce events and suppress target activity caused by the sync engine itself.

Alternative considered: watch source only. Rejected because the user explicitly wants target visibility too.

## Risks / Trade-offs

- [Frontmatter rewrite changes formatting] -> Use a YAML/frontmatter layer that preserves field content and limit edits to targeted keys; cover with fixture tests.
- [Watcher loops on managed target updates] -> Tag in-process sync windows and ignore matching target events during reconciliation.
- [Flatten collisions make sync destructive] -> Fail fast before writing any target changes when two source skills map to the same flattened name.
- [Nearest-config discovery surprises users outside configured projects] -> Allow `--config` override and emit actionable errors that point users to `skill-organizer project add`.
- [Service behavior differs across operating systems] -> Encapsulate service integration behind one package and verify at least Linux end-to-end in this environment.
