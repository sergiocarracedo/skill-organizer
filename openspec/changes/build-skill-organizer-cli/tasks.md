## 1. OpenSpec And Glossary

- [x] 1.1 Finalize the proposal, design, and capability specs for the CLI migration and new features
- [x] 1.2 Create `UBIQUITOUS_LANGUAGE.md` for the skill organizer domain terms

## 2. CLI Bootstrap

- [x] 2.1 Initialize the `cli/` Go module with Cobra, shared version metadata, and a root command
- [x] 2.2 Add the initial command tree for sync, status, project add/edit/remove, skill enable/disable/move-unmanaged, watched, watch, and service operations

## 3. Configuration And Registry

- [x] 3.1 Implement per-project config loading, saving, and nearest-config discovery for `.skill-organizer.yml`
- [x] 3.2 Implement the home watch registry at `~/.config/skill-organizer/skill-organizer.yml` with list, add, and remove operations for config paths
- [x] 3.3 Implement interactive target-folder candidate discovery for `add`

## 4. Source Skill Model And Metadata

- [x] 4.1 Implement organized source scanning with terminal-skill traversal and flatten collision detection
- [x] 4.2 Implement `SKILL.md` frontmatter parsing and rewriting that preserves unrelated fields while updating `name` and `metadata.skill-organizer`
- [x] 4.3 Implement skill enable and disable commands that update source metadata by source path

## 5. Sync And Status

- [x] 5.1 Implement managed symlink reconciliation and internal target manifest handling
- [x] 5.2 Implement status classification for synced, disabled, drifted, stale, broken, and unmanaged entries
- [x] 5.3 Implement move-unmanaged preview and confirmation flow

## 6. Watch And Service

- [x] 6.1 Implement foreground watch mode with fsnotify, debounce, and self-generated target-event suppression
- [x] 6.2 Implement service lifecycle commands using `kardianos/service` backed by the home watch registry

## 7. Verification And Documentation

- [x] 7.1 Add unit and fixture tests for config discovery, source scanning, frontmatter rewriting, sync reconciliation, and status reporting
- [x] 7.2 Document setup, sync, watch, service, and migration from the legacy shell scripts
