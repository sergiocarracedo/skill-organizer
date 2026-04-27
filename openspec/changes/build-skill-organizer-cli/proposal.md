## Why

The current skills sync workflow lives in shell scripts tied to one filesystem layout and a narrow symlink-only maintenance model. We need a Go CLI that preserves the current flattening behavior while adding interactive setup, source metadata management, status reporting, unmanaged target migration, and background watching for multiple configured skill projects.

## What Changes

- Add a `skill-organizer` CLI in `cli/` built with Cobra and PTerm.
- Preserve the current organized-tree-to-flat-target sync behavior, including terminal skill discovery and `/` to `--` flattening.
- Rewrite source `SKILL.md` frontmatter during sync so the canonical skill name matches the flattened target name while preserving the original name and source-relative path in `metadata.skill-organizer`.
- Add commands to enable and disable source skills by updating `metadata.skill-organizer.disabled`.
- Add per-project configuration stored at `<target-parent>/.skill-organizer.yml` and interactive setup that starts from a target skills folder.
- Add a home watch registry at `~/.config/skill-organizer/skill-organizer.yml` that stores watched config paths only.
- Add status reporting for managed skills, disabled skills, drift, broken managed links, and unmanaged target entries.
- Add a command to move unmanaged target skills into the organized source tree with interactive confirmation.
- Add foreground watch mode with `fsnotify` and background service management with `kardianos/service`.

## Capabilities

### New Capabilities
- `managed-skill-sync`: Discover organized source skills, rewrite source metadata, and reconcile managed symlinks in the flat target folder.
- `project-configuration`: Create, edit, remove, and resolve per-project sync configuration starting from a target skills folder.
- `watched-project-registry`: Register and manage watched project configs in the home registry for foreground and background watch execution.
- `skill-status-and-control`: Report sync state, toggle source skill disabled state, and move unmanaged target skills into the organized source tree.

### Modified Capabilities

None.

## Impact

- Adds a new Go module under `cli/`.
- Introduces Go dependencies for Cobra, PTerm, fsnotify, kardianos/service, and YAML parsing.
- Replaces the operational role of `/home/sergio/.agents/scripts/sync-organized-skills.sh` and `watch-organized-skills.sh`.
- Changes source `SKILL.md` files during sync by updating frontmatter `name` and `metadata.skill-organizer` fields.
- Adds per-project config files and a machine-level watch registry.
