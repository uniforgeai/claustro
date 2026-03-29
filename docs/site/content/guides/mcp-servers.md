---
title: MCP Servers
weight: 3
---

# MCP Servers

claustro supports stdio-based and SSE-based MCP servers.

## Pre-installed (stdio)

The default image includes:
- **filesystem** — file system access within /workspace
- **memory** — persistent memory for Claude
- **fetch** — HTTP fetching capability

Toggle in `claustro.yaml`:

```yaml
image:
  mcp_servers:
    filesystem: true
    memory: true
    fetch: false
```

## Project MCP config

Place a `.mcp.json` in your project root. It will be merged with image defaults (project config wins).

## SSE sibling containers

```yaml
mcp:
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      port: 8000
      env:
        DATABASE_URI: postgresql://postgres:postgres@db:5432/devdb
```

SSE servers run as sibling containers on the same Docker network.
