## ADDED Requirements

### Requirement: The system SHALL configure one managed project from a target folder
The system SHALL provide an interactive setup flow that starts from a target skills folder, proposes a default source sibling named `skills-organized`, allows the user to override the source path, and writes the resulting config to `<target-parent>/.skill-organizer.yml`.

#### Scenario: Setup uses sibling source by default
- **WHEN** the user selects a target folder during interactive setup
- **THEN** the system proposes a default source path that is the sibling `skills-organized` folder next to the selected target

#### Scenario: Setup writes per-project config next to the target folder
- **WHEN** the user confirms setup values
- **THEN** the system writes a config file at the parent of the selected target containing the configured `source` and `target`

### Requirement: Commands SHALL resolve the active project config predictably
The system SHALL use the config passed with `--config` when provided, otherwise it SHALL resolve the nearest `.skill-organizer.yml` by walking upward from the current working directory.

#### Scenario: Explicit config overrides discovery
- **WHEN** the user runs a command with `--config`
- **THEN** the system uses the provided config file and does not search upward for another project config

#### Scenario: Nearest config is discovered from the working tree
- **WHEN** the user runs a command without `--config` from inside a configured workspace
- **THEN** the system selects the closest ancestor `.skill-organizer.yml`

### Requirement: Target suggestions SHALL help the user choose a skills folder
The interactive setup flow SHALL search known AI tool directory patterns and offer matching target skills folders as interactive choices while also allowing a custom path.

#### Scenario: Known target folders appear as setup choices
- **WHEN** the setup flow scans for candidate target folders
- **THEN** the system offers discovered folders such as `.agents/skills`, `.claude/skills`, and `.opencode/skills` as selectable options
