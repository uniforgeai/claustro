## ADDED Requirements

### Requirement: up — create and start sandbox

The system SHALL create and start a Docker container as a sandbox for running Claude Code.

#### Scenario: Image built on first up

- **WHEN** the user runs `claustro up` and no `claustro` image exists
- **THEN** the embedded Dockerfile is used to build the image
- **AND** progress is printed to stdout during the build
- **AND** on success the container is created and started

#### Scenario: Existing image reused

- **WHEN** the user runs `claustro up` and the image already exists
- **THEN** the build step is skipped and a new container is created immediately

#### Scenario: Source bind-mounted to /workspace

- **WHEN** a sandbox starts
- **THEN** the current working directory is bind-mounted read-write to `/workspace` inside the container

#### Scenario: ~/.claude bind-mounted

- **WHEN** a sandbox starts
- **THEN** the host `~/.claude` directory is bind-mounted to `/home/sandbox/.claude` inside the container
- **AND** plans, sessions, todos, and auth state are visible inside the container

#### Scenario: ~/.claude.json bind-mounted if present

- **WHEN** a sandbox starts and `~/.claude.json` exists on the host
- **THEN** it is bind-mounted to `/home/sandbox/.claude.json` inside the container
- **AND** Claude Code finds its configuration file

#### Scenario: Host-path symlink created at startup

- **WHEN** a sandbox starts with the host project path `/Users/pepusz/code/my-saas`
- **THEN** inside the container, `/Users/pepusz/code/my-saas` is a symlink pointing to `/workspace`
- **AND** Claude Code running from that path uses the same `~/.claude/projects/` entry as the host session

#### Scenario: Container runs as non-root user

- **WHEN** a sandbox is running
- **THEN** all user processes run as `sandbox` (uid 9999)
- **AND** the container does not run with elevated privileges

#### Scenario: Container stays running in background

- **WHEN** `claustro up` completes successfully
- **THEN** the container is running in the background
- **AND** the container name is printed to stdout

#### Scenario: Already running

- **WHEN** the user runs `claustro up` and a container with that identity already exists and is running
- **THEN** an informative message is printed and the command exits cleanly (no error)

### Requirement: shell — open interactive shell in sandbox

The system SHALL open an interactive shell session inside a running sandbox.

#### Scenario: Interactive zsh session

- **GIVEN** a running sandbox
- **WHEN** the user runs `claustro shell`
- **THEN** an interactive zsh session is opened inside the container as the `sandbox` user
- **AND** the working directory inside the shell is `/workspace`

#### Scenario: Sandbox not running

- **WHEN** the user runs `claustro shell` and no running sandbox exists for the current project
- **THEN** a clear error message is printed suggesting `claustro up`

### Requirement: claude — launch Claude Code in sandbox

The system SHALL launch Claude Code with `--dangerously-skip-permissions` inside a running sandbox.

#### Scenario: Claude Code launched

- **GIVEN** a running sandbox
- **WHEN** the user runs `claustro claude`
- **THEN** `claude --dangerously-skip-permissions` is executed inside the container as the `sandbox` user
- **AND** the working directory is `/workspace`

#### Scenario: Additional arguments passed through

- **WHEN** the user runs `claustro claude -- --print "hello"`
- **THEN** the additional arguments are appended to the claude invocation

### Requirement: burn — remove sandbox container

The system SHALL stop and remove a sandbox container while preserving the image.

#### Scenario: Container stopped and removed

- **WHEN** the user runs `claustro burn`
- **THEN** the sandbox container is stopped and removed
- **AND** the Docker image is preserved
- **AND** `~/.claude` on the host is untouched

#### Scenario: Sandbox not running

- **WHEN** the user runs `claustro burn` and no sandbox exists for the current project
- **THEN** a clear message is printed and the command exits cleanly

### Requirement: ls — list sandboxes for current project

The system SHALL list all sandbox containers for the current project.

#### Scenario: List running sandboxes

- **WHEN** the user runs `claustro ls`
- **THEN** all containers with label `claustro.project={current-project}` are listed
- **AND** each row shows: name, status, uptime

#### Scenario: No sandboxes

- **WHEN** the user runs `claustro ls` and no sandboxes exist for the current project
- **THEN** an empty list or "no sandboxes" message is printed

#### Scenario: List all projects

- **WHEN** the user runs `claustro ls --all`
- **THEN** all containers with label `claustro.managed=true` are listed across all projects
- **AND** each row includes the project name
