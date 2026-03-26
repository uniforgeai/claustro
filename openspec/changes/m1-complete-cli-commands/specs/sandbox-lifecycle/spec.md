## ADDED Requirements

### Requirement: nuke — remove all sandboxes for a project

The system SHALL stop, remove, and clean up all sandbox containers and their networks for the current project in a single command.

#### Scenario: All project sandboxes removed

- **WHEN** the user runs `claustro nuke`
- **THEN** all containers with label `claustro.project={current-project}` are stopped and removed
- **AND** each sandbox's Docker network is removed
- **AND** the Docker image is preserved

#### Scenario: No sandboxes to remove

- **WHEN** the user runs `claustro nuke` and no sandboxes exist for the current project
- **THEN** an informative message is printed and the command exits cleanly

#### Scenario: Nuke all projects

- **WHEN** the user runs `claustro nuke --all`
- **THEN** all containers with label `claustro.managed=true` are stopped and removed across all projects
- **AND** all associated networks are removed

### Requirement: rebuild — rebuild the sandbox image

The system SHALL force a full rebuild of the `claustro:latest` Docker image.

#### Scenario: Image rebuilt unconditionally

- **WHEN** the user runs `claustro rebuild`
- **THEN** the existing `claustro:latest` image is replaced with a fresh build from the embedded Dockerfile
- **AND** build progress is streamed to stdout

#### Scenario: Restart sandboxes after rebuild

- **WHEN** the user runs `claustro rebuild --restart`
- **THEN** all running sandboxes for the current project are stopped before the rebuild
- **AND** after the rebuild they are restarted
- **AND** if a sandbox fails to restart, an error is reported but other sandboxes continue

### Requirement: exec — run a one-off command in sandbox

The system SHALL run a single command inside a running sandbox and stream its output to the terminal.

#### Scenario: Command executed and output streamed

- **GIVEN** a running sandbox
- **WHEN** the user runs `claustro exec -- <command> [args...]`
- **THEN** the command is executed inside the container as the `sandbox` user in `/workspace`
- **AND** stdout and stderr are streamed to the terminal
- **AND** the exit code of the command is propagated as the exit code of `claustro exec`

#### Scenario: Sandbox not running

- **WHEN** the user runs `claustro exec` and no running sandbox exists for the current project
- **THEN** a clear error message is printed suggesting `claustro up`

### Requirement: status — show sandbox details

The system SHALL display detailed runtime information about a sandbox.

#### Scenario: Status shown for running sandbox

- **GIVEN** a running sandbox
- **WHEN** the user runs `claustro status`
- **THEN** the following fields are printed: container name, state, image, uptime, mounts, network, and host project path

#### Scenario: Status shown for stopped sandbox

- **WHEN** the user runs `claustro status` and the sandbox exists but is stopped
- **THEN** the status output reflects the stopped state with last exit time

#### Scenario: Sandbox not found

- **WHEN** the user runs `claustro status` and no sandbox exists for the current project
- **THEN** a clear error message is printed

### Requirement: logs — stream container logs

The system SHALL stream or tail the stdout/stderr logs of a sandbox container.

#### Scenario: Recent logs printed

- **GIVEN** a sandbox (running or stopped)
- **WHEN** the user runs `claustro logs`
- **THEN** the last 100 lines of container stdout and stderr are printed to the terminal

#### Scenario: Follow mode streams live output

- **WHEN** the user runs `claustro logs --follow`
- **THEN** logs are streamed continuously until the user presses Ctrl+C

#### Scenario: Tail flag limits output

- **WHEN** the user runs `claustro logs --tail 50`
- **THEN** only the last 50 lines are shown

#### Scenario: Sandbox not found

- **WHEN** the user runs `claustro logs` and no sandbox exists for the current project
- **THEN** a clear error message is printed
