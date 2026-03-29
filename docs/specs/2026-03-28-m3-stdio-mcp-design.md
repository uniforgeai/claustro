# M3 Stdio MCP Servers — Design Spec

> **Status:** Draft
> **Date:** 2026-03-28

## Overview

Claustro sandboxes ship three pre-installed stdio MCP servers (filesystem, memory, fetch) baked into the Docker image. On container startup, claustro builds a merged MCP config from built-in defaults and project-level `claustro.yaml` overrides, then writes the result as `/home/sandbox/.claude/mcp.json` inside the container via Docker SDK exec.

---

## Pre-installed MCP Servers

Three `@anthropic-ai` MCP server packages are installed globally in the Docker image via npm:

| Server | npm Package | Purpose |
|--------|-------------|---------|
| filesystem | `@anthropic-ai/mcp-server-filesystem` | Local filesystem access scoped to `/workspace` |
| memory | `@anthropic-ai/mcp-server-memory` | Working memory (key-value store) |
| fetch | `@anthropic-ai/mcp-server-fetch` | HTTP fetch for external resources |

These are the same servers Claude Code users commonly configure. Pre-installing them avoids npx download delays on first use and ensures offline availability inside the sandbox.

---

## Dockerfile Changes

Add a single `RUN` layer after the Claude Code install step to globally install the three MCP server packages:

```dockerfile
# Install MCP servers
RUN npm install -g \
    @anthropic-ai/mcp-server-filesystem \
    @anthropic-ai/mcp-server-memory \
    @anthropic-ai/mcp-server-fetch
```

The `npx -y` invocation in the MCP config will resolve to the globally installed version without network access.

---

## MCP Config Format

Claude Code reads MCP server configuration from `~/.claude/mcp.json`. The file follows this structure:

```json
{
  "mcpServers": {
    "<server-name>": {
      "command": "<executable>",
      "args": ["<arg1>", "<arg2>"]
    }
  }
}
```

### Default Config

The three built-in servers produce this default:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"]
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@anthropic-ai/mcp-server-memory"]
    },
    "fetch": {
      "command": "npx",
      "args": ["-y", "@anthropic-ai/mcp-server-fetch"]
    }
  }
}
```

---

## Config Merging Logic

The merged MCP config is built from three layers, with later layers winning on key collision:

1. **Built-in defaults** — the three pre-installed servers above.
2. **Existing host config** — entries from the host's `~/.claude/mcp.json` (if bind-mounted and present).
3. **Project overrides** — entries from `claustro.yaml` under `mcp.stdio`.

### Merge rules

- Each layer is a map of `serverName -> {command, args}`.
- Merge is a shallow key-level merge: if two layers define the same server name, the later layer's entry replaces the earlier one entirely (no field-level merge within a server entry).
- Servers from earlier layers that are not overridden are preserved.

### Project config in `claustro.yaml`

```yaml
mcp:
  stdio:
    my-custom-server:
      command: node
      args: ["/workspace/tools/mcp-server.js"]
    filesystem:
      command: npx
      args: ["-y", "@anthropic-ai/mcp-server-filesystem", "/workspace", "/data"]
```

This would add `my-custom-server` and override the default `filesystem` entry with a broader path list.

---

## Where the Merged Config Is Written

The merged config is written to `/home/sandbox/.claude/mcp.json` inside the container using Docker SDK `ContainerExecCreate` + `ContainerExecAttach` after the container starts.

### Interaction with `~/.claude` mount modes

**Bind-mounted (default, `--isolated-state` not used):**
- `/home/sandbox/.claude` is the host's `~/.claude` directory.
- Claustro reads the existing `/home/sandbox/.claude/mcp.json` (if present), merges with defaults and project config, and writes the result back.
- This modifies the host file. The merge preserves all existing entries that are not overridden by defaults or project config. Host entries take precedence over built-in defaults but are overridden by project config.
- On container teardown, the host file retains the merged state. Users who want to avoid host modification should use `--isolated-state`.

**Volume-backed (`--isolated-state`):**
- `/home/sandbox/.claude` is a Docker volume, not the host directory.
- No host file is read or modified.
- Claustro writes the merged config (defaults + project overrides only) to the volume.
- The volume persists across container restarts within the same project.

### Write mechanism

A new non-interactive exec writes the JSON content via a shell command:

```
["sh", "-c", "cat > /home/sandbox/.claude/mcp.json << 'CLAUSTRO_EOF'\n<json-content>\nCLAUSTRO_EOF"]
```

This is executed via the Docker SDK (`ContainerExecCreate` / `ContainerExecAttach`), consistent with the project's hard constraint against shelling out to docker.

---

## New Package: `internal/mcp`

A new `internal/mcp` package encapsulates all MCP config logic:

### Types

```go
// StdioServer represents a single stdio MCP server entry.
type StdioServer struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}

// Config represents the full mcp.json structure.
type Config struct {
    MCPServers map[string]StdioServer `json:"mcpServers"`
}
```

### Functions

```go
// DefaultConfig returns the built-in default MCP server config.
func DefaultConfig() Config

// FromProjectConfig converts claustro.yaml MCPConfig.Stdio entries into an mcp.Config.
func FromProjectConfig(stdio map[string]config.MCPStdio) Config

// Merge combines multiple configs, with later configs taking precedence.
func Merge(configs ...Config) Config

// MarshalJSON returns the JSON representation suitable for mcp.json.
func (c Config) MarshalJSON() ([]byte, error)

// ParseJSON parses an existing mcp.json file's contents.
func ParseJSON(data []byte) (Config, error)
```

---

## Integration Point

In `cmd/claustro/up.go`, the `ensureRunning` function is modified to call MCP config setup after `container.Start`:

```
container.Start(...)

// Write MCP config into the running container.
mcpCfg := mcp.DefaultConfig()
if hostMCPJSON exists and !isolatedState {
    hostCfg := mcp.ParseJSON(hostMCPJSON)
    mcpCfg = mcp.Merge(mcpCfg, hostCfg)
}
if projectMCPStdio is non-empty {
    projectCfg := mcp.FromProjectConfig(cfg.MCP.Stdio)
    mcpCfg = mcp.Merge(mcpCfg, projectCfg)
}
writeMCPConfig(ctx, cli, containerID, mcpCfg)
```

The `writeMCPConfig` helper performs the Docker SDK exec to write the file.

---

## Error Handling

- **Dockerfile build failure** (npm install fails): image build fails, `claustro up` reports the error. No partial state.
- **Existing mcp.json parse failure**: log a warning via `slog.Warn`, fall back to defaults + project config only (do not crash).
- **Exec write failure**: return error from `ensureRunning`, container is left running but without MCP config. User can manually fix or `claustro burn` and retry.
- **Empty project MCP config**: no error; only defaults (and host config if applicable) are written.

---

## Testing Strategy

### Unit tests (no Docker required)

All in `internal/mcp/`:

- `TestDefaultConfig` — returns exactly the three expected servers with correct commands and args.
- `TestFromProjectConfig` — converts `map[string]MCPStdio` to `Config` correctly.
- `TestMerge_DefaultsOnly` — single config passes through.
- `TestMerge_ProjectOverridesDefault` — project entry replaces default entry with same key.
- `TestMerge_ProjectAddsNew` — project entry with new key is added alongside defaults.
- `TestMerge_ThreeLayers` — defaults + host + project merges correctly with last-wins semantics.
- `TestMerge_EmptyConfigs` — merging empty configs produces empty result.
- `TestMarshalJSON` — output matches expected JSON structure.
- `TestParseJSON` — round-trip: marshal then parse produces identical config.
- `TestParseJSON_Invalid` — malformed JSON returns error.

### Integration tests (Docker required, `//go:build integration`)

- `TestWriteMCPConfig_Integration` — start a container, write config via exec, exec `cat` to read it back, verify contents.

---

## Out of Scope

- SSE-based MCP servers (`mcp.sse` in `claustro.yaml`) — separate M3 feature.
- Custom MCP server installation (user-provided npm packages) — future enhancement.
- MCP server health checks or lifecycle management.
- Removing or disabling default servers (could be added later with `enabled: false` syntax).
