# Git & GitHub Integration

> **Milestone:** M2
> **Status:** Spec — not yet implemented

## Requirement: Git Identity

The system SHALL mount `~/.gitconfig` from the host into every sandbox so that git commits produced by Claude Code carry the correct author identity.

#### Scenario: Author identity in commits

- **GIVEN** the host user has `user.name` and `user.email` set in `~/.gitconfig`
- **WHEN** a sandbox is created
- **THEN** `~/.gitconfig` is bind-mounted read-only at `/home/sandbox/.gitconfig`
- **AND** `git commit` inside the container uses the host user's name and email

#### Scenario: No gitconfig on host

- **GIVEN** `~/.gitconfig` does not exist on the host
- **WHEN** a sandbox is created
- **THEN** no mount attempt is made (no error)
- **AND** git uses its built-in defaults (or the user can set identity inside the container)

#### Scenario: Opt-out via claustro.yaml

- **WHEN** `claustro.yaml` contains `git.mount_gitconfig: false`
- **THEN** `~/.gitconfig` is not mounted
- **AND** the container starts without error

---

## Requirement: SSH Authentication via Agent Forwarding

The system SHALL forward the host SSH agent socket into the sandbox so that `git push` and `git clone` over SSH work without copying private key material into the container.

#### Scenario: Agent available on host (default)

- **GIVEN** `SSH_AUTH_SOCK` is set in the host environment
- **WHEN** a sandbox is created
- **THEN** the SSH agent socket is bind-mounted into the container
- **AND** `SSH_AUTH_SOCK` is set in the container environment to the forwarded socket path
- **AND** `git push` over SSH succeeds using the host's loaded keys
- **AND** no private key files are present inside the container

#### Scenario: No agent on host

- **GIVEN** `SSH_AUTH_SOCK` is not set in the host environment
- **WHEN** a sandbox is created
- **THEN** no SSH socket mount is attempted
- **AND** the sandbox starts normally
- **AND** SSH-based git operations will fail unless the user opts in to `mount_ssh_dir`

#### Scenario: Opt-out via claustro.yaml

- **WHEN** `claustro.yaml` contains `git.forward_agent: false`
- **THEN** `SSH_AUTH_SOCK` is not forwarded even if it is set on the host

#### Scenario: SSH directory fallback (explicit opt-in only)

- **WHEN** `claustro.yaml` contains `git.mount_ssh_dir: true`
- **THEN** `~/.ssh/` is bind-mounted read-only at `/home/sandbox/.ssh/`
- **AND** a warning is printed at `up` time: "Mounting ~/.ssh read-only into the container. Private key material will be accessible inside the sandbox."
- **AND** this option is mutually exclusive with `forward_agent: true` (agent forwarding takes precedence)

> **Note:** On macOS, the host user's uid is typically 501 while the container sandbox user is uid 1000.
> SSH refuses key files not owned by the current user (mode check). When `mount_ssh_dir: true`,
> the user is responsible for ensuring key permissions are compatible. Agent forwarding is the
> recommended path and avoids this issue entirely.

---

## Requirement: GitHub CLI Authentication

The system SHALL mount `~/.config/gh/` from the host into every sandbox so that the `gh` CLI inside the container is pre-authenticated.

#### Scenario: gh auth available

- **GIVEN** the host user has authenticated with `gh auth login`
- **AND** `~/.config/gh/` exists on the host
- **WHEN** a sandbox is created
- **THEN** `~/.config/gh/` is bind-mounted read-write at `/home/sandbox/.config/gh/`
- **AND** `gh auth status` inside the container shows the host user as authenticated
- **AND** `gh pr create` and other gh commands work without additional login

#### Scenario: gh config not present on host

- **GIVEN** `~/.config/gh/` does not exist on the host
- **WHEN** a sandbox is created
- **THEN** no mount attempt is made
- **AND** the container starts normally
- **AND** the user can run `gh auth login` inside the container to authenticate

#### Scenario: Token refresh persists to host

- **GIVEN** `~/.config/gh/` is mounted read-write
- **WHEN** `gh` refreshes its auth token inside the container
- **THEN** the refreshed token is written back to the host `~/.config/gh/`
- **AND** the host `gh` CLI remains authenticated after the sandbox is burned

#### Scenario: Opt-out via claustro.yaml

- **WHEN** `claustro.yaml` contains `git.mount_gh_config: false`
- **THEN** `~/.config/gh/` is not mounted

---

## Requirement: GitHub CLI in the Base Image

The system SHALL install the `gh` CLI in the base Docker image.

#### Scenario: gh available by default

- **WHEN** any sandbox is started using the default claustro image
- **THEN** `gh --version` succeeds inside the container
- **AND** no additional user setup is required to have the binary available

#### Scenario: gh installed via official apt repository

- **WHEN** the base image is built
- **THEN** `gh` is installed from the official GitHub CLI apt repository (`cli.github.com/packages`)
- **AND** the installation does not use curl-piped-to-sh for the key (use `gpg --dearmor` pattern)

---

## Requirement: claustro.yaml git configuration block

The system SHALL support a `git:` block in `claustro.yaml` to control all git credential forwarding behaviour.

#### Scenario: All defaults (no git block in claustro.yaml)

```yaml
# Effective defaults when no git: block is present
git:
  forward_agent: true      # if SSH_AUTH_SOCK is set
  mount_gitconfig: true    # if ~/.gitconfig exists
  mount_gh_config: true    # if ~/.config/gh/ exists
  mount_ssh_dir: false     # never by default
```

#### Scenario: Fully disabled

- **WHEN** `claustro.yaml` contains:
  ```yaml
  git:
    forward_agent: false
    mount_gitconfig: false
    mount_gh_config: false
  ```
- **THEN** no git-related mounts or env vars are added
- **AND** the sandbox behaves as if no host git config exists

#### Scenario: SSH dir fallback

- **WHEN** `claustro.yaml` contains `git.mount_ssh_dir: true`
- **THEN** a warning is printed and `~/.ssh/` is mounted read-only
- **AND** `forward_agent` is ignored for this sandbox (ssh dir takes precedence if agent not available)

---

## Security Notes

- SSH agent forwarding: the container can use loaded keys to authenticate but cannot extract the private key material. The agent socket is scoped to the container's lifetime.
- `~/.config/gh/` contains a GitHub PAT. Mounting it read-write means a compromised sandbox could overwrite or exfiltrate the token. Users running sandboxes with `--dangerously-skip-permissions` (the normal Claude Code mode) should be aware of this tradeoff.
- `~/.gitconfig` contains no credentials by default; mounting it read-only is low risk.
- `~/.ssh/` mounting is an explicit opt-in with a printed warning precisely because it exposes private key material.
