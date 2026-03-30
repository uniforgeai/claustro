# claustro

Disposable Docker sandboxes for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Run Claude Code safely against your local source code. Source stays on the host (bind-mounted), containers are cheap to burn and respawn.

## Why

Running Claude Code directly on a host machine is risky. The `--dangerously-skip-permissions` flag — essential for autonomous agentic workflows — grants unrestricted filesystem and shell access. The community consensus is clear: never run it on bare metal.

claustro fills that gap with a lightweight, CLI-driven sandbox that understands your project and coexists peacefully with your own dev stack.

## How It Works

```
┌──────────────────────────────────────────────────┐
│  Host Machine                                    │
│                                                  │
│  ~/.claude/ ◄──── bind mount ────────────►┐      │
│  ~/projects/app/ ◄─ bind mount (/workspace)┤     │
│                                            │     │
│  ┌──────────────────────────────────────┐  │     │
│  │  claustro sandbox container          │  │     │
│  │                                      │  │     │
│  │  /workspace  ← source (rw or ro)    │  │     │
│  │  ~/.claude   ← sessions, auth       │◄─┘     │
│  │                                      │        │
│  │  Claude Code CLI (official npm pkg)  │        │
│  │  Node.js, Python, Go, Rust, tools   │        │
│  │  Optional: egress firewall           │        │
│  │  Optional: MCP servers               │        │
│  └──────────────────────────────────────┘        │
│                                                  │
│  ┌──────────────────────────────────────┐        │
│  │  Your own docker-compose stack       │        │
│  │  (db, redis, api — fully separate)   │        │
│  └──────────────────────────────────────┘        │
└──────────────────────────────────────────────────┘
```

- **Source never leaves the host** — always bind-mounted, never copied
- **Image is heavy and reusable** — runtimes and tools baked in, built once
- **Container is light and disposable** — burn in seconds, respawn instantly
- **Non-conflicting** — sandbox infra never collides with your project's docker-compose
- **Auth-transparent** — runs the real Claude Code CLI, never handles or proxies credentials

## Prerequisites

- Docker Engine or Docker Desktop
- An active [Claude Code](https://docs.anthropic.com/en/docs/claude-code) subscription

## Installation

```bash
go install github.com/uniforgeai/claustro/cmd/claustro@latest
```

Or build from source:

```bash
git clone https://github.com/uniforgeai/claustro.git
cd claustro
make build
# binary is at bin/claustro
```

## Quick Start

```bash
# Navigate to your project
cd ~/projects/my-app

# Initialize a sandbox config (interactive wizard)
claustro init

# Spin up a sandbox
claustro up

# Open a shell inside the sandbox
claustro shell

# Launch Claude Code inside the sandbox
claustro claude

# Tear it down when done
claustro burn
```

## Commands

| Command           | Description                                      |
| ----------------- | ------------------------------------------------ |
| `claustro init`   | Initialize sandbox configuration for a project   |
| `claustro up`     | Create and start a sandbox container             |
| `claustro shell`  | Open an interactive shell in a running sandbox   |
| `claustro claude` | Launch Claude Code inside the sandbox            |
| `claustro exec`   | Run a command inside the sandbox                 |
| `claustro status` | Show sandbox status                              |
| `claustro ls`     | List all sandboxes                               |
| `claustro logs`   | Show sandbox container logs                      |
| `claustro rebuild`| Rebuild the sandbox image                        |
| `claustro burn`   | Destroy a sandbox container                      |
| `claustro nuke`   | Destroy all sandboxes and cleanup                |
| `claustro config` | Show or edit sandbox configuration               |
| `claustro doctor` | Check system health and prerequisites            |

## Configuration

claustro uses a `claustro.yaml` file in your project root. Run `claustro init` to generate one interactively.

## Sandbox Isolation Options

- `--readonly` — mount source as read-only inside the container
- `--isolated-state` — use a per-sandbox Claude state directory instead of sharing `~/.claude`
- Egress firewall — restrict outbound network access from the sandbox
- MCP server support — run MCP servers inside the sandbox (stdio or SSE)

## Development

```bash
make build        # Build the binary
make test         # Run tests
make lint         # Run golangci-lint
```

## License

Business Source License 1.1 — see [LICENSE](LICENSE) for the full text.

**Licensor:** Uniforge GmbH | **Change Date:** March 30, 2030 | **Change License:** MIT

Production use is permitted for entities with annual revenue under 5,000,000 EUR. Offering claustro as a hosted or managed service that competes with the Licensor requires a separate commercial license.
