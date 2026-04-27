## ADDED Requirements

### Requirement: Organized source skills SHALL sync into a flat managed target
The system SHALL discover terminal skills under the configured source tree, derive a flattened target name by replacing `/` in the source-relative path with `--`, and reconcile a managed top-level symlink in the configured target for each enabled source skill.

#### Scenario: Sync creates managed symlinks for enabled source skills
- **WHEN** a configured source tree contains terminal skill folders with `SKILL.md` files and the user runs sync
- **THEN** the system creates or repairs top-level symlinks in the configured target using the flattened source-relative path as the link name

#### Scenario: Sync removes stale managed target entries
- **WHEN** a previously managed source skill no longer exists or becomes disabled
- **THEN** the system removes the corresponding managed target symlink and leaves unmanaged target entries untouched

### Requirement: Terminal skill discovery SHALL stop at the first skill on a branch
The system SHALL treat the first directory containing `SKILL.md` on a source branch as a terminal skill and SHALL NOT scan child folders of that directory as part of the same branch traversal.

#### Scenario: Nested child folders under a terminal skill are ignored
- **WHEN** a source directory contains `SKILL.md` and also contains child directories
- **THEN** the system treats that directory as the branch terminal skill and does not continue scanning its descendants for additional skills on that branch

### Requirement: Sync SHALL rewrite source skill metadata for flattened naming
The system SHALL update each discovered source `SKILL.md` frontmatter so the top-level `name` equals the flattened target name and `metadata.skill-organizer` contains `original-name`, `source-relative-path`, and `disabled`, while preserving unrelated frontmatter fields.

#### Scenario: Sync preserves unrelated frontmatter while rewriting managed fields
- **WHEN** a source `SKILL.md` contains frontmatter fields outside `name` and `metadata.skill-organizer`
- **THEN** sync updates only the managed fields and keeps the unrelated frontmatter fields present in the file

#### Scenario: Sync stores original source metadata for renamed skills
- **WHEN** a source skill name differs from the flattened target name
- **THEN** sync writes the flattened name to the top-level `name` field and stores the pre-flattened name and source-relative path under `metadata.skill-organizer`

### Requirement: Flatten collisions SHALL fail before target mutation
The system SHALL detect when two source skills map to the same flattened target name and SHALL stop sync with an error before mutating the target.

#### Scenario: Colliding source skills block sync
- **WHEN** two discovered source skills produce the same flattened target name
- **THEN** the system aborts sync and reports both source paths and the colliding target name
