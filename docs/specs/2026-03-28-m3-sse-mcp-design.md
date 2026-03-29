# M3 SSE MCP Servers — Design Spec

> **Status:** Draft
> **Date:** 2026-03-28

## Overview

SSE MCP servers run as sibling Docker containers alongside the sandbox, attached to the sandbox's existing bridge network. This allows the sandbox (and Claude Code running inside it) to reach MCP servers by hostname. The sibling containers follow the sandbox lifecycle — they are created on `claustro up` and removed on `claustro burn` / `claustro nuke`.

---

## Config Structure

SSE MCP servers are declared in `claustro.yaml` under `mcp.sse`:

```yaml
mcp:
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      port: 8000       # optional, default 8000
      env:
        DATABASE_URI: postgresql://postgres:postgres@db:5432/devdb
    browser:
      image: example/browser-mcp:latest
      port: 3000
```

Each key under `mcp.sse` is the server name. It becomes the container hostname alias on the network and appears in the generated `mcp.json`.

### MCPSSE Struct Change

Add a `Port` field to the existing `MCPSSE` struct in `internal/config/config.go`:

```go
type MCPSSE struct {
    Image string            `yaml:"image"`
    Port  int               `yaml:"port"`
    Env   map[string]string `yaml:"env"`
}
```

`Port` defaults to `8000` when omitted (zero value). The `internal/mcp` package handles the default at runtime, not during YAML unmarshalling.

---

## Container Naming

Sibling MCP containers extend the sandbox's naming scheme:

```
claustro-{project}_{name}_mcp-{server-name}
```

Examples:
- `claustro-myapp_brave-fox_mcp-postgres`
- `claustro-myapp_brave-fox_mcp-browser`

A new helper method `MCPContainerName(serverName string) string` is added to the `Identity` struct.

---

## Labels

Sibling containers carry the sandbox's base labels plus MCP-specific labels:

| Label              | Value             |
|--------------------|-------------------|
| `claustro.managed` | `true`            |
| `claustro.project` | `{project}`       |
| `claustro.name`    | `{name}`          |
| `claustro.role`    | `mcp-sse`         |
| `claustro.mcp-server` | `{server-name}` |

A new helper method `MCPLabels(serverName string) map[string]string` is added to the `Identity` struct. It returns the base `Labels()` map plus the two MCP-specific entries.

The `claustro.role` label is used during burn/nuke to find sibling containers that need cleanup. The `claustro.mcp-server` label identifies individual servers for logging and debugging.

---

## Networking

**No new network is created.** Sibling containers attach to the sandbox's existing network (`claustro-{project}_{name}_net`) with a hostname alias equal to the server name.

```go
NetworkingConfig: &networktypes.NetworkingConfig{
    EndpointsConfig: map[string]*networktypes.EndpointSettings{
        id.NetworkName(): {
            Aliases: []string{serverName},
        },
    },
}
```

This allows the sandbox to reach the server at `http://{server-name}:{port}/sse`. For example, `http://postgres:8000/sse`.

### Prerequisite

The sandbox network must exist before sibling containers are created. Since `container.Create()` already calls `ensureNetwork()`, the sandbox container must be created first. This ordering is already the natural flow in `ensureRunning`.

---

## MCP Endpoint Injection

SSE endpoints are added to the `mcp.json` file that gets mounted into the sandbox. The same file is used for stdio MCP servers. For SSE servers, the entry uses the `url` field:

```json
{
  "mcpServers": {
    "postgres": {
      "url": "http://postgres:8000/sse"
    },
    "browser": {
      "url": "http://browser:3000/sse"
    }
  }
}
```

The URL is constructed as: `http://{server-name}:{port}/sse`

If `port` is not specified in config, `8000` is used as the default.

This `mcp.json` generation must coordinate with the stdio MCP plan — both contribute entries to the same file. The `internal/mcp` package owns `mcp.json` generation for both types.

---

## Lifecycle

### `claustro up` Flow

After the sandbox container is created and started:

1. Read `mcp.sse` from the loaded config
2. For each SSE server definition:
   a. Pull or verify the image exists (use Docker SDK `ImagePull` if not present)
   b. Create the sibling container with:
      - The server's image
      - Environment variables from config
      - Attached to the sandbox network with hostname alias
      - MCP labels applied
   c. Start the container
3. If a sibling container fails to start, log a warning and continue (do not block the sandbox)
4. Generate/update `mcp.json` with SSE endpoint entries

### `claustro burn` Flow

Before removing the sandbox container:

1. List all containers with labels matching `claustro.project={project}`, `claustro.name={name}`, and `claustro.role=mcp-sse`
2. Stop and remove each sibling container (log warnings on failure, do not abort)
3. Then stop and remove the sandbox container
4. Then remove the network (after all containers are detached)

### `claustro nuke` Flow

Same pattern as burn, applied to all sandboxes:

1. For each sandbox container found, also find its MCP siblings
2. Stop and remove siblings first, then the sandbox, then the network

---

## Error Handling

| Scenario                          | Behavior                                    |
|-----------------------------------|---------------------------------------------|
| Sibling image pull fails          | Log warning, skip this server, continue     |
| Sibling container create fails    | Log warning, skip this server, continue     |
| Sibling container start fails     | Log warning, continue (sandbox still works) |
| Sibling stop fails during burn    | Log warning, continue cleanup               |
| Sibling remove fails during burn  | Log warning, continue cleanup               |
| No SSE servers configured         | No-op (no siblings created)                 |

The principle: MCP sibling failures are non-fatal. The sandbox must always come up and be cleanable regardless of sibling status.

---

## Testing Strategy

### Unit Tests

- **Identity:** `MCPContainerName()` returns correct format, `MCPLabels()` returns correct labels
- **Config:** `MCPSSE` struct parses `port` field correctly, defaults to zero (runtime default applied elsewhere)
- **MCP SSE logic:** Container config generation produces correct `ContainerConfig`, `HostConfig`, `NetworkingConfig` with aliases
- **MCP endpoint generation:** `mcp.json` output includes correct URLs with port handling
- **Sibling listing:** Filter logic correctly identifies MCP siblings by labels

### Integration Tests (Docker-gated)

- Create a sandbox, start SSE siblings, verify they appear on the same network with correct hostnames
- Burn a sandbox, verify siblings are removed alongside it
- Sibling failure does not prevent sandbox startup
- Nuke removes all siblings across all sandboxes in a project
