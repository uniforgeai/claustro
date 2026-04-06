# Claude Sandbox — Project Specification

> **Status:** Draft
> **Version:** 0.1.0
> **Author:** Peter (Uniforge GmbH)
> **Date:** 2026-03-24

## Why

Running Claude Code directly on a host machine is risky. The `--dangerously-skip-permissions` flag — essential for autonomous agentic workflows — grants unrestricted filesystem and shell access. The community consensus is clear: never run it on bare metal.

Existing solutions (Docker Desktop Sandboxes, devcontainers, NVIDIA OpenShell) each solve a piece of the puzzle but none deliver a lightweight, CLI-driven, docker-compose-compatible sandbox that understands multi-language monorepos and coexists peacefully with a project's own dev stack.

Claude Sandbox fills that gap.

## What

A Go CLI tool that manages disposable Docker containers for running Claude Code against local source code. Source stays on the host (bind-mounted). Containers are cheap to burn and respawn. The image holds the heavy parts (runtimes, tools, Claude Code). The user's `~/.claude` directory is bind-mounted so plans, sessions, and auth persist across container lifecycles.

## Principles

- Source never leaves the host — always bind-mounted, never copied
- Image is heavy and reusable — all runtimes and tools baked in, built once
- Container is light and disposable — burn in seconds, respawn instantly
- Non-conflicting — sandbox infrastructure never collides with the project's own docker-compose setup
- Auth-transparent — runs the real Claude Code CLI, never handles or proxies credentials
- Anthropic TOS compliant — no OAuth spoofing, no harness impersonation, no credential proxying

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Host Machine                                       │
│                                                     │
│  ~/.claude/ ◄──────── bind mount ──────────────►┐   │
│  ~/projects/my-app/ ◄─ bind mount (/workspace)──┤   │
│                                                  │   │
│  ┌───────────────────────────────────────────┐   │   │
│  │  claude-sandbox container                 │   │   │
│  │                                           │   │   │
│  │  /workspace  ← source (rw or ro)         │   │   │
│  │  ~/.claude   ← plans, sessions, auth     │◄──┘   │
│  │                                           │       │
│  │  Claude Code CLI (official npm package)   │       │
│  │  Node.js, Python, Go, Rust, dev tools     │       │
│  │                                           │       │
│  │  Optional: egress firewall (iptables)     │       │
│  │  Optional: MCP servers (stdio / SSE)      │       │
│  └───────────────────────────────────────────┘       │
│                                                      │
│  ┌───────────────────────────────────────────┐       │
│  │  Project's own docker-compose stack       │       │
│  │  (db, redis, api — completely separate)   │       │
│  └───────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────┘
```

---

## Requirements

### Requirement: CLI Tool

The system SHALL provide a single Go binary (`claude-sandbox` or a shorter name TBD) as the primary user interface.

#### Scenario: Installation

- **GIVEN** a machine with Docker Engine or Docker Desktop installed
- **WHEN** the user installs the binary (via `go install`, Homebrew, or direct download)
- **THEN** the binary is available in PATH with no additional dependencies

#### Scenario: Context awareness

- **GIVEN** the user runs the CLI from a project directory
- **WHEN** any command is invoked without explicit path arguments
- **THEN** the current working directory is treated as the project root
- **AND** the project is identified by its directory basename (or `claustro.yaml` override)

#### Scenario: Help and discoverability

- **WHEN** the user runs `claude-sandbox help`
- **THEN** all commands are listed with brief descriptions and usage examples

---

### Requirement: Sandbox Identity

The system SHALL derive a unique sandbox identity from the project path and an optional sandbox name.

#### Scenario: Default sandbox

- **GIVEN** the user is in `~/projects/my-saas`
- **WHEN** they run `claude-sandbox up`
- **THEN** a sandbox is created with identity `my-saas_default`

#### Scenario: Named sandbox

- **GIVEN** the user is in `~/projects/my-saas`
- **WHEN** they run `claude-sandbox up --name backend`
- **THEN** a sandbox is created with identity `my-saas_backend`

#### Scenario: Isolation between projects

- **GIVEN** sandboxes exist for `my-saas` and `other-project`
- **WHEN** the user runs `claude-sandbox ls` from `~/projects/my-saas`
- **THEN** only sandboxes for `my-saas` are shown

#### Scenario: Global listing

- **WHEN** the user runs `claude-sandbox ls --all`
- **THEN** all sandboxes across all projects are shown with their project and status

---

### Requirement: Sandbox Lifecycle

The system SHALL support a full container lifecycle through CLI commands.

#### Scenario: Up (create and start)

- **WHEN** the user runs `claude-sandbox up`
- **THEN** the image is built if not present
- **AND** a new container is created and started
- **AND** the source directory is bind-mounted to `/workspace`
- **AND** `~/.claude` is bind-mounted to the container's home `.claude`
- **AND** the container stays running in the background

#### Scenario: Shell

- **GIVEN** a running sandbox
- **WHEN** the user runs `claude-sandbox shell [name]`
- **THEN** an interactive shell (zsh) is opened inside the container

#### Scenario: Claude

- **GIVEN** a running sandbox
- **WHEN** the user runs `claude-sandbox claude [name] [-- claude-args...]`
- **THEN** `claude --dangerously-skip-permissions` is executed inside the container
- **AND** any additional arguments are passed through to Claude Code

#### Scenario: Exec

- **GIVEN** a running sandbox
- **WHEN** the user runs `claude-sandbox exec [name] -- cmd args...`
- **THEN** the command is executed inside the container

#### Scenario: Burn (remove container)

- **WHEN** the user runs `claude-sandbox burn [name]`
- **THEN** the container is stopped and removed
- **AND** the image is preserved
- **AND** `~/.claude` on the host is untouched
- **AND** cache volumes (npm, pip) are preserved

#### Scenario: Nuke (remove container and cache volumes)

- **WHEN** the user runs `claude-sandbox nuke [name]`
- **THEN** the container is stopped and removed
- **AND** all associated cache volumes are removed
- **AND** the image is preserved
- **AND** `~/.claude` on the host is untouched

#### Scenario: Rebuild (rebuild image)

- **WHEN** the user runs `claude-sandbox rebuild`
- **THEN** the Docker image is rebuilt with `--no-cache`
- **AND** existing containers are not affected until next `up`

#### Scenario: Status

- **WHEN** the user runs `claude-sandbox status [name]`
- **THEN** the container state, uptime, resource usage, and mounted paths are shown

#### Scenario: Logs

- **WHEN** the user runs `claude-sandbox logs [name] [-f]`
- **THEN** container logs are displayed, optionally in follow mode

---

### Requirement: Multiple Concurrent Sandboxes

The system SHALL support running multiple sandboxes simultaneously for the same project.

#### Scenario: Parallel sandboxes

- **GIVEN** the user is in `~/projects/my-saas`
- **WHEN** they run `claude-sandbox up --name api` and `claude-sandbox up --name web`
- **THEN** two separate containers are running concurrently
- **AND** each has its own network, cache volumes, and container name
- **AND** both share the same `~/.claude` bind mount from the host

#### Scenario: Targeting a specific sandbox

- **GIVEN** multiple sandboxes are running for the current project
- **WHEN** the user runs `claude-sandbox shell api`
- **THEN** the shell opens in the `api` sandbox specifically

#### Scenario: Burn all

- **WHEN** the user runs `claude-sandbox burn --all`
- **THEN** all sandboxes for the current project are stopped and removed

---

### Requirement: Non-Conflicting Docker Setup

The system SHALL never conflict with the project's own docker-compose infrastructure.

#### Scenario: Separate compose project

- **WHEN** a sandbox is created
- **THEN** it uses a Docker compose project name prefixed with `claude-sandbox-` (or uses Docker API directly)
- **AND** the project's own `docker-compose.yml` is never read, parsed, or modified

#### Scenario: Separate network

- **WHEN** a sandbox is created
- **THEN** it creates its own Docker bridge network
- **AND** it does not attach to any existing project network
- **AND** it does not bind any host ports by default

#### Scenario: No naming collisions

- **WHEN** a sandbox container is created
- **THEN** its container name follows the pattern `claude-sandbox-{project}_{name}`
- **AND** volume names follow the pattern `claude-sandbox-{project}_{name}_{purpose}`
- **AND** network names follow the pattern `claude-sandbox-{project}_{name}_net`

---

### Requirement: Source Mounting

The system SHALL bind-mount the local source code into the container.

#### Scenario: Default mount (whole project)

- **GIVEN** the user is in `~/projects/my-saas`
- **WHEN** they run `claude-sandbox up`
- **THEN** `~/projects/my-saas` is mounted to `/workspace` in the container

#### Scenario: Subdirectory mount (monorepo)

- **WHEN** the user runs `claude-sandbox up --name api --workdir ./services/api`
- **THEN** `~/projects/my-saas/services/api` is mounted to `/workspace`

#### Scenario: Additional mounts

- **WHEN** the user runs `claude-sandbox up --mount ./libs:/workspace/libs:ro`
- **THEN** the additional path is bind-mounted with the specified mode
- **AND** multiple `--mount` flags are supported

#### Scenario: Read-only option

- **WHEN** the user runs `claude-sandbox up --readonly`
- **THEN** the source is mounted as read-only (`:ro`)

---

### Requirement: Claude State Persistence

The system SHALL bind-mount `~/.claude` from the host into every sandbox container.

> **⚠️ TESTING REQUIRED:** Docker Desktop Sandboxes are known to ignore user-level Claude
> config even when `~/.claude` is mounted, because the container's `HOME` points elsewhere
> or Claude Code resolves config paths differently inside containers. We MUST verify that:
>
> 1. The container user's `HOME` is set so that `~/.claude` resolves to the mounted path
> 2. Claude Code inside the container actually reads plans, todos, settings, MCP config,
     >    plugins, skills, commands, rules, and hooks from the mounted directory
> 3. Session history written inside the container appears on the host and vice versa
> 4. Auth tokens (`claude login`) created inside the container work on next container start
> 5. Project-scoped config (`~/.claude/projects/`) maps correctly when `/workspace`
     >    is a bind-mounted host path (Claude Code indexes by absolute path)
>
> If Claude Code uses hardcoded or resolved absolute paths internally to index projects,
> the path difference between host (`/home/peter/projects/my-app`) and container
> (`/workspace`) may cause it to treat them as different projects. This needs investigation
> and possibly a symlink or `CLAUDE_CONFIG_DIR` override.

#### Scenario: Plans and sessions persist

- **GIVEN** a Claude Code session created plans and todos in a sandbox
- **WHEN** the sandbox is burned and a new one is created
- **THEN** the plans and todos are still available in the new sandbox

#### Scenario: Auth persists

- **GIVEN** the user authenticated with `claude login` inside a sandbox
- **WHEN** the sandbox is burned and a new one is created
- **THEN** the user is still authenticated

#### Scenario: Shared across sandboxes

- **GIVEN** multiple sandboxes are running for the same project
- **THEN** they all share the same `~/.claude` from the host

#### Scenario: HOME and config path alignment

- **WHEN** the container starts
- **THEN** the container user's `HOME` environment variable points to a directory where `~/.claude` resolves to the bind-mounted host `~/.claude`
- **AND** `CLAUDE_CONFIG_DIR` is set if needed to override any path resolution issues
- **AND** Claude Code recognizes all user-level config: plugins, skills, commands, rules, hooks, MCP servers

#### Scenario: Project path mapping

- **GIVEN** the host project path is `/home/user/projects/my-app`
- **AND** the container workspace path is `/workspace`
- **WHEN** Claude Code writes project-scoped state to `~/.claude/projects/`
- **THEN** the project is correctly identified regardless of the path difference
- **OR** a documented workaround exists (symlink, env var, config override)

#### Scenario: Isolated state option

- **WHEN** the user runs `claude-sandbox up --isolated-state`
- **THEN** a named Docker volume is used instead of the host bind mount
- **AND** the host `~/.claude` is not affected

---

### Requirement: Multi-Language Support

The system SHALL provide a container image with runtimes for multiple programming languages.

#### Scenario: Default image (polyglot)

- **WHEN** the default image is built
- **THEN** it includes Node.js (LTS), Python 3.12+, Go (latest stable), Rust (latest stable)
- **AND** common dev tools: git, ripgrep, fd, fzf, jq, tree, tmux, htop, zsh, make
- **AND** build essentials: gcc, pkg-config, libssl-dev

#### Scenario: Monorepo with mixed languages

- **GIVEN** a monorepo with a Go backend and a React frontend
- **WHEN** the user runs `claude-sandbox up`
- **THEN** Claude Code can build and test both the Go and React code within the same container

#### Scenario: Custom image

- **WHEN** the user specifies `image:` in `claustro.yaml` or uses `--image` flag
- **THEN** that image is used instead of the default
- **AND** it must have Claude Code pre-installed or the user accepts responsibility

---

### Requirement: MCP Server Support

The system SHALL support both stdio-based and SSE-based MCP servers.

#### Scenario: Stdio MCP servers (pre-installed)

- **GIVEN** stdio MCP servers are installed in the image (filesystem, memory, fetch)
- **WHEN** Claude Code starts inside the container
- **THEN** the pre-configured MCP servers are available via Claude's MCP config

#### Scenario: Project MCP config

- **GIVEN** the project root contains a `.mcp.json` file
- **WHEN** a sandbox starts
- **THEN** the project MCP config is merged with the image defaults
- **AND** the project config takes precedence over defaults

#### Scenario: SSE MCP servers as compose siblings

- **WHEN** the user configures SSE MCP servers in `claustro.yaml`
- **THEN** the CLI starts the MCP server containers alongside the sandbox
- **AND** they share a Docker network so the sandbox can reach them
- **AND** the MCP endpoints are injected into Claude's config

---

### Requirement: Egress Firewall

The system SHALL optionally restrict outbound network access from the sandbox.

#### Scenario: Firewall disabled (default)

- **WHEN** a sandbox is created without firewall configuration
- **THEN** outbound network access is unrestricted

#### Scenario: Firewall enabled

- **WHEN** the user enables the firewall via `--firewall` flag or `claustro.yaml`
- **THEN** outbound traffic is blocked by default (iptables DROP)
- **AND** the following domains are whitelisted: Anthropic API, npm registry, PyPI, GitHub, Ubuntu repos
- **AND** Docker internal networks (172.16.0.0/12, 192.168.0.0/16, 10.0.0.0/8) are allowed for compose-sibling services

#### Scenario: Custom domain whitelist

- **WHEN** the user specifies additional allowed domains in `claustro.yaml`
- **THEN** those domains are added to the firewall whitelist

---

### Requirement: Security

The system SHALL enforce security best practices by default.

#### Scenario: No Docker socket

- **THEN** the host Docker socket (`/var/run/docker.sock`) is NEVER mounted into the sandbox

#### Scenario: Non-root user

- **THEN** all processes inside the container run as a non-root user (uid 1000)

#### Scenario: No privilege escalation

- **THEN** the container runs with `no-new-privileges` security option

#### Scenario: Resource limits

- **THEN** the container has configurable CPU and memory limits
- **AND** defaults to 4 CPUs and 8 GB memory

---

### Requirement: Project Configuration

The system SHALL support a `claustro.yaml` configuration file in the project root.

#### Scenario: Config discovery

- **WHEN** the CLI is invoked from a directory containing `claustro.yaml`
- **THEN** the configuration is loaded and applied

#### Scenario: Config structure

```yaml
# claustro.yaml
project: my-saas                    # override project slug (default: dirname)
image: claude-sandbox:latest        # custom image

defaults:
  firewall: false
  readonly: false
  resources:
    cpus: "4"
    memory: 8G

sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
      - ./proto:/workspace/proto:ro
    env:
      DATABASE_URL: postgresql://host.docker.internal:5432/dev

  web:
    workdir: ./packages/frontend
    env:
      API_URL: http://localhost:3000

firewall:
  enabled: false
  allow:
    - custom-registry.company.com
    - api.openai.com

mcp:
  stdio:
    filesystem:
      command: npx
      args: ["-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"]
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      env:
        DATABASE_URI: postgresql://postgres:postgres@db:5432/devdb
```

#### Scenario: CLI flags override config

- **WHEN** the user provides CLI flags that conflict with `claustro.yaml`
- **THEN** CLI flags take precedence

---

### Requirement: Environment Variables

The system SHALL support passing environment variables into the sandbox.

#### Scenario: API key via environment

- **GIVEN** `ANTHROPIC_API_KEY` is set in the host environment or `.env` file
- **WHEN** a sandbox starts
- **THEN** the API key is forwarded to the container

#### Scenario: .env file

- **GIVEN** a `.env` file exists alongside `claustro.yaml`
- **WHEN** a sandbox starts
- **THEN** variables from `.env` are loaded into the container environment

#### Scenario: Per-sandbox env

- **GIVEN** `claustro.yaml` defines env vars for a named sandbox
- **WHEN** that sandbox starts
- **THEN** the per-sandbox env vars are applied

---

### Requirement: Anthropic TOS Compliance

The system SHALL remain fully compliant with Anthropic's Terms of Service.

#### Scenario: Official CLI only

- **THEN** the system runs the official `@anthropic-ai/claude-code` npm package
- **AND** it NEVER impersonates, spoofs, or wraps the Claude Code harness

#### Scenario: No credential proxying

- **THEN** the system NEVER handles, stores, intercepts, or proxies Anthropic credentials
- **AND** auth is the user's responsibility via `ANTHROPIC_API_KEY` env var or `claude login`

#### Scenario: No OAuth routing

- **THEN** the system NEVER uses OAuth tokens from Claude Free/Pro/Max subscriptions programmatically
- **AND** if the user authenticates via `claude login` inside the container, that is their direct action

---

### Requirement: Version and Update

The system SHALL provide version information and self-update capabilities.

#### Scenario: Version command

- **WHEN** the user runs `claustro version`
- **THEN** the version, commit hash, and build date are displayed
- **AND** version info is injected at build time via ldflags

#### Scenario: Update command

- **WHEN** the user runs `claustro update`
- **THEN** the install method is auto-detected (Homebrew, go install, or binary download)
- **AND** the appropriate update mechanism is invoked
- **AND** if the method is unknown, a link to GitHub releases is shown

#### Scenario: Background update reminder

- **WHEN** any claustro command is executed
- **THEN** a non-blocking background check for newer versions runs
- **AND** the check result is cached for 24 hours in `~/.config/claustro/update-check.json`
- **AND** if a newer version is available, a reminder is printed to stderr after command completion
- **AND** development builds are never nagged

---

### Requirement: Voice Mode Support

The system SHALL optionally support Claude Code's voice mode in sandbox containers.

#### Scenario: Voice mode disabled (default)

- **WHEN** `tools.voice` is not set or set to `false` in `claustro.yaml`
- **THEN** SoX and audio libraries are not installed in the image

#### Scenario: Voice mode enabled

- **WHEN** `tools.voice` is set to `true` in `claustro.yaml`
- **THEN** the image includes SoX, libsox-fmt-all, alsa-utils, and pulseaudio-utils
- **AND** Claude Code's `/voice` command can function inside the container
- **AND** the audio bridge streams host microphone audio into the container via socket
- **AND** container-side `rec`/`arecord` shims connect to the bridge for recording

---

## Implementation

### Requirement: Go Implementation

The system SHALL be implemented in Go.

#### Scenario: Docker SDK

- **THEN** the CLI uses the Docker SDK for Go (`github.com/docker/docker/client`) to manage containers
- **AND** it does NOT shell out to `docker` or `docker compose` CLI commands

#### Scenario: CLI framework

- **THEN** the CLI uses `cobra` for command parsing and `viper` for configuration

#### Scenario: Build targets

- **THEN** the binary cross-compiles for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- **AND** releases are published as static binaries and via Homebrew

---

## Non-Requirements

- The system is NOT an IDE integration (no VS Code, no Cursor)
- The system is NOT a CI/CD tool (no GitHub Actions integration)
- The system is NOT a multi-tenant platform (single user, local machine)
- The system is NOT a security auditing tool (no compliance reporting)
- The system does NOT manage Anthropic billing or subscriptions
- The system does NOT provide a web UI or terminal multiplexer

---

## Competitive Landscape

| Solution | Approach | Gap for us |
|---|---|---|
| Docker Desktop Sandboxes | MicroVM, `docker sandbox` CLI | No compose compat, requires Desktop, no monorepo |
| Trail of Bits devcontainer | Devcontainer spec + `devc` CLI | VS Code-centric, no compose isolation |
| FoamoftheSea sandbox | Docker Compose + iptables | Single-project, no multi-sandbox, no CLI |
| NVIDIA OpenShell | K3s-in-Docker, YAML policy engine | Heavyweight, alpha, no dev workflow awareness |
| textcortex (archived → Spritz) | K8s control plane | Overkill, requires K8s |

---

## Open Questions

1. **Name** — `claude-sandbox` is descriptive but long. `csb`? `cbox`? `sandclaude`? Something entirely different?
2. **Dockerfile authoring** — should the CLI generate the Dockerfile on the fly, or ship a static one the user can extend?
3. **Plugin/extension model** — should `claustro.yaml` support custom Dockerfile snippets or just image overrides?
4. **Connect to project network** — optional flag to attach the sandbox to the project's compose network so Claude can talk to the project's dev services?
5. **GPU passthrough** — should we support `--gpu` for local model inference via Docker Model Runner?
6. ~~**Update mechanism**~~ — resolved: `claustro update` auto-detects install method (Homebrew, go install, binary) with background 24h update reminders

---

## Milestones

### M1: Core CLI + Container Lifecycle
- `up`, `burn`, `nuke`, `rebuild`, `shell`, `claude`, `exec`, `status`, `logs`, `ls`
- Sandbox identity from CWD + name
- Source bind mount + `~/.claude` bind mount
- Default polyglot Dockerfile

### M2: Multi-Sandbox + Config
- Multiple concurrent sandboxes per project
- `claustro.yaml` config file
- `--workdir`, `--mount`, `--name` flags
- `.env` file support

### M3: Firewall + MCP
- Optional egress firewall
- Stdio MCP server pre-install
- SSE MCP servers as sibling containers
- Project `.mcp.json` merging

### M4: Polish + Distribution
- Homebrew formula
- Cross-platform binaries
- Documentation site
- `claustro.yaml` validation and schema
- GitHub Actions: CI test pipeline
- GitHub Actions: release automation and versioning

### M5: Version, Update + Voice Mode
- `claustro version` command with build-time ldflags (version, commit, date)
- `claustro update` command with install method detection (Homebrew, go install, binary)
- Background update reminder with 24h check interval (non-blocking, cached in `~/.config/claustro/`)
- Voice mode support: optional SoX installation in sandbox image for Claude Code `/voice` command
