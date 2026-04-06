---
title: claustro.yaml
weight: 1
---

# claustro.yaml Reference

Complete reference for the claustro configuration file.

## Top-level fields

| Field | Type | Description |
|-------|------|-------------|
| `project` | string | Project name (default: directory basename) |
| `image` | string or mapping | Custom image name, or image build config |
| `defaults` | mapping | Default settings for all sandboxes |
| `sandboxes` | mapping | Named sandbox definitions |
| `firewall` | mapping | Egress firewall configuration |
| `mcp` | mapping | MCP server configuration |
| `git` | mapping | Git integration settings |

## image

When a string, uses that image directly. When a mapping, configures the built image.

### image.languages

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `node` | bool | true | Node.js (cannot be disabled) |
| `go` | bool | true | Go |
| `rust` | bool | true | Rust |
| `python` | bool | true | Python 3 |

### image.tools

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dev` | bool | true | ripgrep, fd, fzf, jq, tree, htop, tmux |
| `build` | bool | true | gcc, make, pkg-config, libssl-dev |
| `voice` | bool | false | SoX + audio libraries for Claude Code `/voice` command (opt-in) |

### image.mcp_servers

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `filesystem` | bool | true | MCP filesystem server |
| `memory` | bool | true | MCP memory server |
| `fetch` | bool | true | MCP fetch server |

### image.extra

List of additional Dockerfile RUN steps:

```yaml
image:
  extra:
    - run: apt-get install -y ffmpeg
```

## defaults

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `firewall` | bool | false | Enable egress firewall |
| `readonly` | bool | false | Mount source read-only |
| `resources.cpus` | string | "4" | CPU limit |
| `resources.memory` | string | "8G" | Memory limit |

## sandboxes

Named sandbox definitions. Each key is a sandbox name.

| Field | Type | Description |
|-------|------|-------------|
| `workdir` | string | Working directory (relative to project root) |
| `mounts` | list | Additional bind mounts (host:container[:ro\|rw]) |
| `env` | mapping | Environment variables |

## firewall

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable egress firewall |
| `allow` | list | [] | Additional allowed domains |

## mcp.stdio

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Command to run |
| `args` | list | Command arguments |

## mcp.sse

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Docker image |
| `port` | int | SSE port |
| `env` | mapping | Environment variables |

## git

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `forward_agent` | bool | true | Forward SSH agent |
| `mount_gitconfig` | bool | true | Mount ~/.gitconfig (read-only) |
| `mount_gh_config` | bool | true | Mount ~/.config/gh/ |
| `mount_ssh_dir` | bool | false | Mount ~/.ssh/ (explicit opt-in) |
