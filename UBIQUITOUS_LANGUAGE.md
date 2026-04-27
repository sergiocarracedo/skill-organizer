# Ubiquitous Language

## Core Concepts

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Skill Organizer** | The CLI tool that synchronizes organized skill trees into flat tool-readable target folders. | Sync script, watcher script |
| **Source Tree** | The organized hierarchy that contains the canonical skill folders. | Organized repo, source folder set |
| **Target Folder** | The flat first-level skills folder consumed by AI tooling. | Output folder, generated skills folder |
| **Project Config** | The per-project `.skill-organizer.yml` file that declares one `source` and one `target`. | Definition file, local config |
| **Watch Registry** | The home file that stores watched project config paths for foreground and background watching. | Global config, watch config |
| **Terminal Skill** | The first directory containing `SKILL.md` on a traversal branch. | Leaf skill, final skill |
| **Flattened Name** | The managed skill name produced from the source-relative path by replacing `/` with `--`. | Generated name, target name |
| **Managed Target Entry** | A target symlink created and owned by Skill Organizer. | Generated link, synced skill |
| **Unmanaged Target Entry** | A target entry not owned by Skill Organizer. | Manual target skill, foreign target entry |
| **Internal Manifest** | The hidden file in the target root used internally to record managed sync state. | Public manifest, config manifest |

## Skill Metadata

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Skill Organizer Metadata** | The metadata stored under `metadata.skill-organizer` in a source `SKILL.md`. | Sync metadata, generated metadata |
| **Original Name** | The pre-flattened source skill name preserved in `metadata.skill-organizer.original-name`. | Legacy name, old name |
| **Source-Relative Path** | The source skill path relative to the project config source root. | Skill path, original path |
| **Disabled Skill** | A source skill whose `metadata.skill-organizer.disabled` is `true`. | Hidden skill, inactive skill |

## Operations

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Sync** | The reconciliation process that rewrites source metadata and aligns managed target symlinks with the source tree. | Publish, flatten only |
| **Status** | The read-only inspection of source skills, managed target state, and unmanaged target entries. | Health check, diff |
| **Move Unmanaged** | The confirmed operation that moves unmanaged target skills into the organized source tree. | Import target, absorb skills |
| **Watch Mode** | The long-running foreground or background process that reacts to registered project changes. | Poller, daemon only |

## Relationships

- A **Project Config** declares exactly one **Source Tree** and one **Target Folder**.
- A **Source Tree** contains zero or more **Terminal Skills**.
- Each **Terminal Skill** maps to one **Flattened Name** unless a collision blocks sync.
- Each enabled **Terminal Skill** should correspond to one **Managed Target Entry**.
- A **Watch Registry** contains paths to one or more **Project Configs**.
- A **Disabled Skill** has no expected **Managed Target Entry** after sync.

## Example Dialogue

> **Dev:** "If I add a new skill under the source tree, what does sync change?"
>
> **Domain expert:** "Sync discovers the new **Terminal Skill**, rewrites its **Flattened Name** into the source `SKILL.md`, and creates a **Managed Target Entry** in the **Target Folder**."
>
> **Dev:** "What if somebody drops a folder directly into the target?"
>
> **Domain expert:** "That becomes an **Unmanaged Target Entry**. `status` reports it, and `move-unmanaged` can move it into the **Source Tree**."
>
> **Dev:** "And the watcher only needs the home registry?"
>
> **Domain expert:** "Yes. The **Watch Registry** stores **Project Config** paths, and each config points to its own **Source Tree** and **Target Folder**."

## Flagged Ambiguities

- "config" was overloaded between the per-project file and the machine-level watch file. Use **Project Config** for `<target-parent>/.skill-organizer.yml` and **Watch Registry** for `~/.config/skill-organizer/skill-organizer.yml`.
- "definition" was used temporarily for a sync unit, but the final model uses **Project Config** instead.
