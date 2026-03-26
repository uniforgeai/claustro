## Requirements

### Requirement: Sandbox identity derived from project path and name

The system SHALL derive a unique sandbox identity from the current working directory and an optional name flag. This identity is used to name all Docker resources (container, network, volumes).

#### Scenario: Default identity from CWD

- **WHEN** the user runs any command from `/Users/pepusz/code/my-saas` without `--name`
- **THEN** the project slug is `my-saas` (directory basename, lowercased)
- **AND** the sandbox name is `default`
- **AND** the full identity is `my-saas_default`

#### Scenario: Named sandbox identity

- **WHEN** the user runs a command with `--name backend`
- **THEN** the full identity is `my-saas_backend`

#### Scenario: Project slug sanitization

- **WHEN** the directory name contains uppercase letters or non-alphanumeric characters (e.g., `My.Project`)
- **THEN** the slug is lowercased and non-alphanumeric characters replaced with `-` (e.g., `my-project`)

### Requirement: Docker resource names derived from identity

The system SHALL name all Docker resources with a `claustro-` prefix and the sandbox identity to avoid collisions with the project's own Docker infrastructure.

#### Scenario: Container naming

- **WHEN** a sandbox with project `my-saas` and name `default` is created
- **THEN** the container name is `claustro-my-saas-default`

#### Scenario: Network naming

- **WHEN** a sandbox is created
- **THEN** the Docker bridge network is named `claustro-my-saas-default-net`

#### Scenario: Label applied to container

- **WHEN** a sandbox container is created
- **THEN** it receives the Docker label `claustro.project=my-saas`
- **AND** the label `claustro.name=default`
- **AND** the label `claustro.managed=true`
