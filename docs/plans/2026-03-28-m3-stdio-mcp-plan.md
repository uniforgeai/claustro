# M3 Stdio MCP Servers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pre-install three stdio MCP servers in the Docker image and auto-configure Claude Code's `mcp.json` on container startup by merging defaults, host config, and project overrides.

**Architecture:** A new `internal/mcp` package handles config types, default generation, merging, and JSON serialization. The `ensureRunning` function in `cmd/claustro/up.go` calls into this package after `container.Start` and writes the merged config into the container via Docker SDK exec. The Dockerfile gains one `RUN` layer to npm-install the three server packages.

**Tech Stack:** Go, Docker SDK for Go, Testify, `encoding/json`

**Design Spec:** `docs/specs/2026-03-28-m3-stdio-mcp-design.md`

---

### Task 1: Add MCP server npm packages to Dockerfile

**Files:**
- Modify: `internal/image/Dockerfile`

**Dependencies:** None

- [ ] **Step 1: Add npm install layer**

In `internal/image/Dockerfile`, add after the Claude Code install block (after the `RUN npm install -g @anthropic-ai/claude-code` line and its related commands, before the ccstatusline install):

```dockerfile
# Install MCP servers (filesystem, memory, fetch)
RUN npm install -g \
    @anthropic-ai/mcp-server-filesystem \
    @anthropic-ai/mcp-server-memory \
    @anthropic-ai/mcp-server-fetch
```

- [ ] **Step 2: Verify Dockerfile syntax**

Run: `go build ./...`
Expected: Passes (Dockerfile is not validated by Go build, but ensures nothing else broke).

- [ ] **Step 3: Commit**

```bash
git add internal/image/Dockerfile
git commit -m "feat: pre-install MCP server packages in sandbox image"
```

---

### Task 2: Create `internal/mcp` package — types and default config

**Files:**
- Create: `internal/mcp/mcp.go`
- Create: `internal/mcp/mcp_test.go`

**Dependencies:** None (can run in parallel with Task 1)

- [ ] **Step 1: Write failing tests**

Create `internal/mcp/mcp_test.go`:

```go
package mcp

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
    cfg := DefaultConfig()

    require.Len(t, cfg.MCPServers, 3)

    fs, ok := cfg.MCPServers["filesystem"]
    require.True(t, ok, "filesystem server must exist")
    assert.Equal(t, "npx", fs.Command)
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"}, fs.Args)

    mem, ok := cfg.MCPServers["memory"]
    require.True(t, ok, "memory server must exist")
    assert.Equal(t, "npx", mem.Command)
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-memory"}, mem.Args)

    fetch, ok := cfg.MCPServers["fetch"]
    require.True(t, ok, "fetch server must exist")
    assert.Equal(t, "npx", fetch.Command)
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-fetch"}, fetch.Args)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestDefaultConfig -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write implementation**

Create `internal/mcp/mcp.go`:

```go
// Package mcp builds and merges MCP server configurations for Claude Code sandboxes.
package mcp

// StdioServer represents a single stdio MCP server entry in mcp.json.
type StdioServer struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
}

// Config represents the full mcp.json structure that Claude Code reads.
type Config struct {
    MCPServers map[string]StdioServer `json:"mcpServers"`
}

// DefaultConfig returns the built-in MCP server config with the three pre-installed servers.
func DefaultConfig() Config {
    return Config{
        MCPServers: map[string]StdioServer{
            "filesystem": {
                Command: "npx",
                Args:    []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"},
            },
            "memory": {
                Command: "npx",
                Args:    []string{"-y", "@anthropic-ai/mcp-server-memory"},
            },
            "fetch": {
                Command: "npx",
                Args:    []string{"-y", "@anthropic-ai/mcp-server-fetch"},
            },
        },
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestDefaultConfig -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "feat: add internal/mcp package with default MCP server config"
```

---

### Task 3: Add config merging, project conversion, and JSON serialization

**Files:**
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`

**Dependencies:** Task 2

- [ ] **Step 1: Write failing tests for Merge**

Add to `internal/mcp/mcp_test.go`:

```go
func TestMerge_SingleConfig(t *testing.T) {
    cfg := DefaultConfig()
    merged := Merge(cfg)
    assert.Equal(t, cfg, merged)
}

func TestMerge_LaterOverridesEarlier(t *testing.T) {
    base := Config{
        MCPServers: map[string]StdioServer{
            "filesystem": {Command: "npx", Args: []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"}},
        },
    }
    override := Config{
        MCPServers: map[string]StdioServer{
            "filesystem": {Command: "npx", Args: []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace", "/data"}},
        },
    }
    merged := Merge(base, override)
    require.Len(t, merged.MCPServers, 1)
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace", "/data"}, merged.MCPServers["filesystem"].Args)
}

func TestMerge_AddsNewServers(t *testing.T) {
    base := DefaultConfig()
    extra := Config{
        MCPServers: map[string]StdioServer{
            "custom": {Command: "node", Args: []string{"/workspace/server.js"}},
        },
    }
    merged := Merge(base, extra)
    assert.Len(t, merged.MCPServers, 4)
    assert.Contains(t, merged.MCPServers, "custom")
    assert.Contains(t, merged.MCPServers, "filesystem")
}

func TestMerge_ThreeLayers(t *testing.T) {
    defaults := DefaultConfig()
    host := Config{
        MCPServers: map[string]StdioServer{
            "host-tool": {Command: "host-cmd", Args: []string{"a"}},
            "filesystem": {Command: "npx", Args: []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/host-path"}},
        },
    }
    project := Config{
        MCPServers: map[string]StdioServer{
            "filesystem": {Command: "npx", Args: []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/project-path"}},
        },
    }
    merged := Merge(defaults, host, project)

    // Project wins for filesystem.
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/project-path"}, merged.MCPServers["filesystem"].Args)
    // Host tool preserved.
    assert.Contains(t, merged.MCPServers, "host-tool")
    // Defaults preserved.
    assert.Contains(t, merged.MCPServers, "memory")
    assert.Contains(t, merged.MCPServers, "fetch")
}

func TestMerge_EmptyConfigs(t *testing.T) {
    merged := Merge(Config{}, Config{})
    assert.Empty(t, merged.MCPServers)
}

func TestMerge_NoArgs(t *testing.T) {
    merged := Merge()
    assert.Empty(t, merged.MCPServers)
}
```

- [ ] **Step 2: Write failing tests for FromProjectConfig**

Add to `internal/mcp/mcp_test.go`:

```go
import "github.com/uniforgeai/claustro/internal/config"

func TestFromProjectConfig(t *testing.T) {
    stdio := map[string]config.MCPStdio{
        "custom": {Command: "node", Args: []string{"/workspace/server.js"}},
        "filesystem": {Command: "npx", Args: []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace", "/extra"}},
    }
    cfg := FromProjectConfig(stdio)

    require.Len(t, cfg.MCPServers, 2)
    assert.Equal(t, "node", cfg.MCPServers["custom"].Command)
    assert.Equal(t, []string{"/workspace/server.js"}, cfg.MCPServers["custom"].Args)
    assert.Equal(t, []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace", "/extra"}, cfg.MCPServers["filesystem"].Args)
}

func TestFromProjectConfig_Empty(t *testing.T) {
    cfg := FromProjectConfig(nil)
    assert.Empty(t, cfg.MCPServers)
}
```

- [ ] **Step 3: Write failing tests for JSON round-trip**

Add to `internal/mcp/mcp_test.go`:

```go
func TestMarshalJSON(t *testing.T) {
    cfg := Config{
        MCPServers: map[string]StdioServer{
            "test": {Command: "echo", Args: []string{"hello"}},
        },
    }
    data, err := cfg.JSON()
    require.NoError(t, err)
    assert.Contains(t, string(data), `"mcpServers"`)
    assert.Contains(t, string(data), `"test"`)
    assert.Contains(t, string(data), `"command"`)
}

func TestParseJSON(t *testing.T) {
    original := DefaultConfig()
    data, err := original.JSON()
    require.NoError(t, err)

    parsed, err := ParseJSON(data)
    require.NoError(t, err)
    assert.Equal(t, original, parsed)
}

func TestParseJSON_Invalid(t *testing.T) {
    _, err := ParseJSON([]byte(`not json`))
    assert.Error(t, err)
}

func TestParseJSON_Empty(t *testing.T) {
    cfg, err := ParseJSON([]byte(`{}`))
    require.NoError(t, err)
    assert.Empty(t, cfg.MCPServers)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -v`
Expected: FAIL — `Merge`, `FromProjectConfig`, `JSON`, `ParseJSON` undefined.

- [ ] **Step 5: Write implementation**

Add to `internal/mcp/mcp.go`:

```go
import (
    "encoding/json"
    "fmt"

    "github.com/uniforgeai/claustro/internal/config"
)

// FromProjectConfig converts claustro.yaml MCP stdio entries into a Config.
func FromProjectConfig(stdio map[string]config.MCPStdio) Config {
    if len(stdio) == 0 {
        return Config{}
    }
    servers := make(map[string]StdioServer, len(stdio))
    for name, s := range stdio {
        servers[name] = StdioServer{
            Command: s.Command,
            Args:    s.Args,
        }
    }
    return Config{MCPServers: servers}
}

// Merge combines multiple configs. Later configs take precedence on key collision.
func Merge(configs ...Config) Config {
    merged := Config{MCPServers: make(map[string]StdioServer)}
    for _, cfg := range configs {
        for name, server := range cfg.MCPServers {
            merged.MCPServers[name] = server
        }
    }
    return merged
}

// JSON returns the indented JSON representation suitable for writing to mcp.json.
func (c Config) JSON() ([]byte, error) {
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("marshaling mcp config: %w", err)
    }
    return data, nil
}

// ParseJSON parses the contents of an mcp.json file.
func ParseJSON(data []byte) (Config, error) {
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return Config{}, fmt.Errorf("parsing mcp config: %w", err)
    }
    return cfg, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/mcp/ -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "feat: add MCP config merging, project conversion, and JSON serialization"
```

---

### Task 4: Add `WriteMCPConfig` function for writing config into a container

**Files:**
- Modify: `internal/mcp/mcp.go`
- Modify: `internal/mcp/mcp_test.go`

**Dependencies:** Task 3

- [ ] **Step 1: Write failing test for write command generation**

The actual Docker exec is an integration concern. Unit-test the command construction instead.

Add to `internal/mcp/mcp_test.go`:

```go
func TestWriteCommand(t *testing.T) {
    cfg := Config{
        MCPServers: map[string]StdioServer{
            "test": {Command: "echo", Args: []string{"hello"}},
        },
    }
    cmd, err := WriteCommand(cfg, "/home/sandbox/.claude/mcp.json")
    require.NoError(t, err)
    require.Len(t, cmd, 3)
    assert.Equal(t, "sh", cmd[0])
    assert.Equal(t, "-c", cmd[1])
    assert.Contains(t, cmd[2], `"mcpServers"`)
    assert.Contains(t, cmd[2], "/home/sandbox/.claude/mcp.json")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestWriteCommand -v`
Expected: FAIL — `WriteCommand` undefined.

- [ ] **Step 3: Write implementation**

Add to `internal/mcp/mcp.go`:

```go
// MCPConfigPath is the path inside the container where Claude Code reads MCP config.
const MCPConfigPath = "/home/sandbox/.claude/mcp.json"

// WriteCommand returns the shell command (for Docker exec) that writes the given
// config to the specified path inside a container.
func WriteCommand(cfg Config, path string) ([]string, error) {
    data, err := cfg.JSON()
    if err != nil {
        return nil, err
    }
    script := fmt.Sprintf("cat > %s << 'CLAUSTRO_MCP_EOF'\n%s\nCLAUSTRO_MCP_EOF", path, string(data))
    return []string{"sh", "-c", script}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestWriteCommand -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/mcp.go internal/mcp/mcp_test.go
git commit -m "feat: add WriteCommand for writing MCP config into containers"
```

---

### Task 5: Integrate MCP config writing into `ensureRunning`

**Files:**
- Modify: `cmd/claustro/up.go`

**Dependencies:** Tasks 1-4

- [ ] **Step 1: Add import**

Add `"github.com/uniforgeai/claustro/internal/mcp"` to the import block in `cmd/claustro/up.go`.

- [ ] **Step 2: Add MCP config setup after container.Start**

In `ensureRunning`, after the `container.Start(ctx, cli, containerID)` call and before the `return id, false, nil`, add:

```go
    // Build and write MCP config into the container.
    if err := writeMCPConfig(ctx, cli, containerID, cfg, resolved.IsolatedState); err != nil {
        slog.Warn("failed to write MCP config", "err", err)
    }
```

- [ ] **Step 3: Add `writeMCPConfig` helper function**

Add to `cmd/claustro/up.go`:

```go
// writeMCPConfig builds the merged MCP config and writes it into the container.
// When isolatedState is false, existing host mcp.json entries are preserved in the merge.
func writeMCPConfig(ctx context.Context, cli *client.Client, containerID string, cfg *config.Config, isolatedState bool) error {
    mcpCfg := mcp.DefaultConfig()

    // When bind-mounted, read existing host mcp.json and merge it (host entries
    // override defaults, but are themselves overridden by project config).
    if !isolatedState {
        home, err := os.UserHomeDir()
        if err == nil {
            hostMCPPath := filepath.Join(home, ".claude", "mcp.json")
            if data, err := os.ReadFile(hostMCPPath); err == nil {
                hostCfg, err := mcp.ParseJSON(data)
                if err != nil {
                    slog.Warn("could not parse host mcp.json, using defaults only", "err", err)
                } else {
                    mcpCfg = mcp.Merge(mcpCfg, hostCfg)
                }
            }
        }
    }

    // Merge project-level MCP stdio overrides.
    if len(cfg.MCP.Stdio) > 0 {
        projectCfg := mcp.FromProjectConfig(cfg.MCP.Stdio)
        mcpCfg = mcp.Merge(mcpCfg, projectCfg)
    }

    cmd, err := mcp.WriteCommand(mcpCfg, mcp.MCPConfigPath)
    if err != nil {
        return fmt.Errorf("building mcp write command: %w", err)
    }

    return container.Exec(ctx, cli, containerID, cmd, container.ExecOptions{})
}
```

- [ ] **Step 4: Build to verify compilation**

Run: `go build ./...`
Expected: No errors.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 6: Run linter**

Run: `golangci-lint run`
Expected: No new warnings.

- [ ] **Step 7: Commit**

```bash
git add cmd/claustro/up.go
git commit -m "feat: write merged MCP config into container on startup"
```

---

### Task 6: End-to-end verification

**Dependencies:** All previous tasks

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 2: Run linter**

Run: `golangci-lint run`
Expected: No new warnings.

- [ ] **Step 3: Verify Dockerfile content**

Visually confirm `internal/image/Dockerfile` contains the MCP server install line in the correct position (after Claude Code, before ccstatusline).

- [ ] **Step 4: Verify mcp package test coverage**

Run: `go test ./internal/mcp/ -v -cover`
Expected: All tests pass, coverage of key functions (DefaultConfig, Merge, FromProjectConfig, JSON, ParseJSON, WriteCommand) is high.

- [ ] **Step 5: Smoke test (if Docker is available)**

Build the image and start a container manually to confirm mcp.json is written:

```bash
go build -o bin/claustro ./cmd/claustro
# In a test project directory:
bin/claustro up --name mcp-test
bin/claustro shell --name mcp-test -- cat /home/sandbox/.claude/mcp.json
bin/claustro burn --name mcp-test
```

Expected: The cat output shows the merged JSON with filesystem, memory, and fetch servers.

- [ ] **Step 6: Final commit if any fixes needed**

Only if verification steps revealed issues. Otherwise this step is a no-op.
