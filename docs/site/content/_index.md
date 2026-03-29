---
title: claustro
type: docs
---

# claustro

Disposable Docker sandboxes for Claude Code.

claustro manages Docker containers for running Claude Code safely against local source code. Source stays on the host (bind-mounted), containers are cheap to burn and respawn.

## Key Features

- **Configurable images** — pick your languages, tools, and MCP servers
- **Multiple sandboxes** — run parallel Claude Code sessions per project
- **Egress firewall** — optional outbound traffic restriction
- **MCP server support** — stdio and SSE-based MCP servers
- **Git integration** — SSH agent, gitconfig, and GitHub CLI forwarding
- **Zero conflict** — never touches your project's docker-compose setup

## Quick Start

```bash
brew tap uniforgeai/tap
brew install claustro

cd ~/projects/my-app
claustro init
claustro up
claustro claude
```

See [Getting Started](getting-started/) for the full guide.
