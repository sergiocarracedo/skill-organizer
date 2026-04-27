## ADDED Requirements

### Requirement: The system SHALL maintain a home watch registry of config paths
The system SHALL store watched project config paths in `~/.config/skill-organizer/skill-organizer.yml` and SHALL treat this file as the source of truth for foreground watch mode and background service execution.

#### Scenario: Watched registry stores only config file paths
- **WHEN** a project is registered for watching
- **THEN** the home registry stores the path to that project config and does not duplicate the source and target values

### Requirement: Watched project commands SHALL manage config-path entries interactively
The system SHALL provide commands to list, add, and remove watched project config paths, and `watched add` SHALL accept config paths rather than target folders.

#### Scenario: Watched add registers a config path
- **WHEN** the user runs `watched add` and provides a valid project config path
- **THEN** the system records that config path in the home watch registry

#### Scenario: Watched remove unregisters a config path
- **WHEN** the user confirms removal of a watched config path
- **THEN** the system removes that path from the home watch registry

### Requirement: Watch execution SHALL load project details from watched configs
The system SHALL read the watched config paths from the home registry and SHALL load each project config to determine the source and target paths to watch.

#### Scenario: Watch mode resolves source and target from the watched config
- **WHEN** foreground or background watch mode starts from the home registry
- **THEN** the system loads each referenced project config to obtain its `source` and `target`
