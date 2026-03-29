# M3 SSE MCP Servers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run SSE-based MCP servers as sibling Docker containers alongside the sandbox, attached to the sandbox's network with hostname aliases, and inject their endpoints into `mcp.json`.

**Architecture:** Config defines SSE servers in `mcp.sse`. On `claustro up`, sibling containers are created after the sandbox starts. On `claustro burn`/`nuke`, siblings are cleaned up before the sandbox is removed. A new `internal/mcp` package owns sibling lifecycle and `mcp.json` generation. The identity package gains MCP naming/labeling helpers.

**Tech Stack:** Go, Cobra, Docker SDK for Go, Testify

**Design Spec:** `docs/specs/2026-03-28-m3-sse-mcp-design.md`

---

### Task 1: Add `Port` field to `MCPSSE` struct

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Dependencies:** None

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go` a test that parses a `claustro.yaml` with `mcp.sse` entries including `port`:

```go
func TestLoad_MCPSSEPort(t *testing.T) {
	dir := t.TempDir()
	yaml := `
mcp:
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      port: 8000
      env:
        DATABASE_URI: postgresql://localhost/db
    browser:
      image: example/browser-mcp:latest
      port: 3000
    noport:
      image: example/noport:latest
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(yaml), 0644))
	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, 8000, cfg.MCP.SSE["postgres"].Port)
	assert.Equal(t, 3000, cfg.MCP.SSE["browser"].Port)
	assert.Equal(t, 0, cfg.MCP.SSE["noport"].Port) // zero means "use default at runtime"
	assert.Equal(t, "crystaldba/postgres-mcp-server:latest", cfg.MCP.SSE["postgres"].Image)
	assert.Equal(t, "postgresql://localhost/db", cfg.MCP.SSE["postgres"].Env["DATABASE_URI"])
}
```

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/config/ -run TestLoad_MCPSSEPort -v`
Expected: FAIL — `Port` field does not exist on `MCPSSE`.

- [ ] **Step 3: Add `Port` field to `MCPSSE`**

In `internal/config/config.go`, modify the `MCPSSE` struct:

```go
type MCPSSE struct {
	Image string            `yaml:"image"`
	Port  int               `yaml:"port"`
	Env   map[string]string `yaml:"env"`
}
```

- [ ] **Step 4: Run test — verify it passes**

Run: `go test ./internal/config/ -run TestLoad_MCPSSEPort -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... && golangci-lint run`

---

### Task 2: Add MCP naming and labeling helpers to identity package

**Files:**
- Modify: `internal/identity/identity.go`
- Modify: `internal/identity/identity_test.go`

**Dependencies:** None (can run in parallel with Task 1)

- [ ] **Step 1: Write failing tests**

Add to `internal/identity/identity_test.go`:

```go
func TestIdentity_MCPContainerName(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "brave-fox"}
	tests := []struct {
		server string
		want   string
	}{
		{"postgres", "claustro-myapp_brave-fox_mcp-postgres"},
		{"browser", "claustro-myapp_brave-fox_mcp-browser"},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			assert.Equal(t, tt.want, id.MCPContainerName(tt.server))
		})
	}
}

func TestIdentity_MCPLabels(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "brave-fox"}
	labels := id.MCPLabels("postgres")

	assert.Equal(t, "true", labels["claustro.managed"])
	assert.Equal(t, "myapp", labels["claustro.project"])
	assert.Equal(t, "brave-fox", labels["claustro.name"])
	assert.Equal(t, "mcp-sse", labels["claustro.role"])
	assert.Equal(t, "postgres", labels["claustro.mcp-server"])
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `go test ./internal/identity/ -run "TestIdentity_MCP" -v`
Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement `MCPContainerName` and `MCPLabels`**

Add to `internal/identity/identity.go`:

```go
// MCPContainerName returns the Docker container name for an MCP sibling server.
// Format: claustro-{project}_{name}_mcp-{serverName}
func (id *Identity) MCPContainerName(serverName string) string {
	return fmt.Sprintf("claustro-%s_%s_mcp-%s", id.Project, id.Name, serverName)
}

// MCPLabels returns Docker labels for an MCP sibling container.
// Includes the base sandbox labels plus MCP-specific role and server name.
func (id *Identity) MCPLabels(serverName string) map[string]string {
	labels := id.Labels()
	labels["claustro.role"] = "mcp-sse"
	labels["claustro.mcp-server"] = serverName
	return labels
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `go test ./internal/identity/ -run "TestIdentity_MCP" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... && golangci-lint run`

---

### Task 3: Create `internal/mcp/sse.go` — sibling container lifecycle

**Files:**
- Create: `internal/mcp/sse.go`
- Create: `internal/mcp/sse_test.go`

**Dependencies:** Tasks 1 and 2

This is the core package. It handles creating, starting, stopping, and removing MCP sibling containers using the Docker SDK.

- [ ] **Step 1: Write failing tests**

Create `internal/mcp/sse_test.go` with unit tests. Use a Docker client mock/interface or test the config-generation functions directly. Focus on:

```go
func TestSSEContainerConfig(t *testing.T) {
	// Test that buildContainerConfig produces correct Config, HostConfig, NetworkingConfig
	// for a given MCPSSE definition, identity, and server name.
}

func TestDefaultPort(t *testing.T) {
	// Port=0 in config -> 8000 at runtime
	assert.Equal(t, 8000, effectivePort(0))
	assert.Equal(t, 3000, effectivePort(3000))
}

func TestSSEEndpointURL(t *testing.T) {
	assert.Equal(t, "http://postgres:8000/sse", EndpointURL("postgres", 0))
	assert.Equal(t, "http://browser:3000/sse", EndpointURL("browser", 3000))
}
```

- [ ] **Step 2: Implement `internal/mcp/sse.go`**

```go
package mcp

import (
	"context"
	"fmt"
	"log/slog"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/identity"
)

const defaultSSEPort = 8000

// effectivePort returns the port to use, defaulting to 8000 if zero.
func effectivePort(port int) int {
	if port == 0 {
		return defaultSSEPort
	}
	return port
}

// EndpointURL returns the SSE endpoint URL for a server name and port.
func EndpointURL(serverName string, port int) string {
	return fmt.Sprintf("http://%s:%d/sse", serverName, effectivePort(port))
}

// StartSSESiblings creates and starts sibling containers for all configured SSE MCP servers.
// Failures are logged as warnings but do not prevent the sandbox from running.
func StartSSESiblings(ctx context.Context, cli *client.Client, id *identity.Identity, servers map[string]config.MCPSSE) error {
	for name, srv := range servers {
		if err := startOneSibling(ctx, cli, id, name, srv); err != nil {
			slog.Warn("MCP SSE sibling failed to start", "server", name, "err", err)
			// Non-fatal: continue with remaining servers
		}
	}
	return nil
}

func startOneSibling(ctx context.Context, cli *client.Client, id *identity.Identity, serverName string, srv config.MCPSSE) error {
	containerName := id.MCPContainerName(serverName)

	env := make([]string, 0, len(srv.Env))
	for k, v := range srv.Env {
		env = append(env, k+"="+v)
	}

	cfg := &containertypes.Config{
		Image:  srv.Image,
		Labels: id.MCPLabels(serverName),
		Env:    env,
	}

	hostCfg := &containertypes.HostConfig{}

	netCfg := &networktypes.NetworkingConfig{
		EndpointsConfig: map[string]*networktypes.EndpointSettings{
			id.NetworkName(): {
				Aliases: []string{serverName},
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		return fmt.Errorf("creating MCP container %q: %w", containerName, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, containertypes.StartOptions{}); err != nil {
		return fmt.Errorf("starting MCP container %q: %w", containerName, err)
	}

	slog.Info("MCP SSE sibling started", "server", serverName, "container", containerName)
	return nil
}

// StopSSESiblings finds and removes all MCP SSE sibling containers for the given identity.
func StopSSESiblings(ctx context.Context, cli *client.Client, id *identity.Identity) error {
	// List containers with matching project, name, and role labels
	// Stop and remove each one
	// Log warnings on failure, do not abort
}
```

Key design points:
- `StartSSESiblings` iterates over `map[string]config.MCPSSE` from the loaded config
- `StopSSESiblings` uses `container.ListByProject` filtered further by role label, or uses its own label-based query
- All failures are logged but non-fatal

- [ ] **Step 3: Implement `StopSSESiblings` with label filtering**

Use Docker SDK `ContainerList` with filters for `claustro.project`, `claustro.name`, and `claustro.role=mcp-sse`.

- [ ] **Step 4: Run tests — verify they pass**

Run: `go test ./internal/mcp/ -v`

- [ ] **Step 5: Run full test suite**

Run: `go test ./... && golangci-lint run`

---

### Task 4: Add `ListMCPSiblings` to container package

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/container/container_test.go` (if unit-testable) or add integration test

**Dependencies:** Task 2 (needs MCP labels defined)

- [ ] **Step 1: Write test for `ListMCPSiblings`**

```go
func TestListMCPSiblings_FiltersCorrectly(t *testing.T) {
	// Integration test (Docker-gated) that creates a sandbox + MCP sibling,
	// then verifies ListMCPSiblings returns only the sibling.
}
```

- [ ] **Step 2: Implement `ListMCPSiblings`**

Add to `internal/container/container.go`:

```go
// ListMCPSiblings returns all MCP SSE sibling containers for the given sandbox identity.
func ListMCPSiblings(ctx context.Context, cli *client.Client, id *identity.Identity) ([]containertypes.Summary, error) {
	args := filters.NewArgs(
		filters.Arg("label", "claustro.project="+id.Project),
		filters.Arg("label", "claustro.name="+id.Name),
		filters.Arg("label", "claustro.role=mcp-sse"),
	)
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("listing MCP siblings: %w", err)
	}
	return containers, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/container/ -v`

---

### Task 5: Integrate SSE startup into `ensureRunning` (up.go)

**Files:**
- Modify: `cmd/claustro/up.go`

**Dependencies:** Tasks 1, 2, 3

After the sandbox container is created and started (line ~234 in up.go), add SSE sibling startup.

- [ ] **Step 1: Add import for `internal/mcp`**

- [ ] **Step 2: After `container.Start()`, add sibling startup**

Insert after the `container.Start` call and before the return:

```go
// Start MCP SSE sibling containers (non-fatal on failure).
if len(cfg.MCP.SSE) > 0 {
	if err := mcp.StartSSESiblings(ctx, cli, id, cfg.MCP.SSE); err != nil {
		slog.Warn("error starting MCP SSE siblings", "err", err)
	}
}
```

- [ ] **Step 3: Verify build compiles**

Run: `go build ./cmd/claustro/`

- [ ] **Step 4: Manual test**

Create a test `claustro.yaml` with an SSE server, run `claustro up`, verify sibling container appears on the same network with the correct hostname alias.

---

### Task 6: Integrate SSE cleanup into burn.go

**Files:**
- Modify: `cmd/claustro/burn.go`

**Dependencies:** Tasks 3, 4

Before stopping and removing the sandbox container, find and remove MCP siblings.

- [ ] **Step 1: Add sibling cleanup to single-sandbox burn**

In `runBurn`, before the sandbox `container.Stop` call, add:

```go
// Clean up MCP SSE siblings before the sandbox.
mcp.StopSSESiblings(ctx, cli, id)
```

- [ ] **Step 2: Add sibling cleanup to `--all` burn path**

In the `--all` loop, for each container check if it's a sandbox (not a sibling) and clean up its siblings first. Alternatively, the label-based listing in `--all` already picks up siblings since they have `claustro.project` labels. Need to ensure siblings are stopped before the sandbox. Strategy: process containers with `claustro.role=mcp-sse` first, then sandboxes.

- [ ] **Step 3: Verify build and test**

Run: `go build ./cmd/claustro/ && go test ./...`

---

### Task 7: Integrate SSE cleanup into nuke.go / lifecycle.go

**Files:**
- Modify: `internal/container/lifecycle.go`

**Dependencies:** Tasks 3, 4

The `NukeContainers` function already lists all containers by project label. MCP sibling containers will be included in this list because they carry `claustro.project` labels. However, the current code assumes each container is a sandbox and tries to clean up its network/volumes.

- [ ] **Step 1: Update `NukeContainers` to handle sibling containers**

Siblings should be stopped/removed but do not have their own network or volumes. Strategy:

1. Partition the container list: siblings (role=mcp-sse) vs sandboxes (no role label or role != mcp-sse)
2. Stop and remove all siblings first
3. Then stop and remove sandboxes with their networks and volumes

```go
// Partition into siblings and sandboxes
var siblings, sandboxes []containertypes.Summary
for _, c := range containers {
	if c.Labels["claustro.role"] == "mcp-sse" {
		siblings = append(siblings, c)
	} else {
		sandboxes = append(sandboxes, c)
	}
}

// Remove siblings first
for _, c := range siblings {
	name := strings.TrimPrefix(c.Names[0], "/")
	fmt.Fprintf(w, "Removing MCP sibling %s...\n", name)
	if err := Stop(ctx, cli, c.ID); err != nil {
		fmt.Fprintf(w, "  (stop: %v — continuing)\n", err)
	}
	if err := Remove(ctx, cli, c.ID); err != nil {
		fmt.Fprintf(w, "  error removing: %v\n", err)
	}
}

// Then remove sandboxes (existing logic)
for _, c := range sandboxes {
	// ... existing nuke logic ...
}
```

- [ ] **Step 2: Write test for partitioned nuke behavior**

Integration test (Docker-gated): create a sandbox + 2 siblings, nuke, verify all three are removed and the network is cleaned up.

- [ ] **Step 3: Run full test suite**

Run: `go test ./... && golangci-lint run`

---

### Task 8: SSE endpoint injection into mcp.json

**Files:**
- Create or modify: `internal/mcp/mcpjson.go`
- Create or modify: `internal/mcp/mcpjson_test.go`

**Dependencies:** Task 3

This task handles generating the `mcp.json` file that gets mounted into the sandbox. This file is shared with stdio MCP servers, so the generation must merge both types.

- [ ] **Step 1: Write failing tests**

```go
func TestGenerateMCPJSON_SSEOnly(t *testing.T) {
	servers := map[string]config.MCPSSE{
		"postgres": {Image: "pg:latest", Port: 8000},
		"browser":  {Image: "br:latest", Port: 3000},
	}
	data, err := GenerateMCPJSON(nil, servers) // nil stdio, SSE servers
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	mcpServers := result["mcpServers"].(map[string]interface{})
	pg := mcpServers["postgres"].(map[string]interface{})
	assert.Equal(t, "http://postgres:8000/sse", pg["url"])

	br := mcpServers["browser"].(map[string]interface{})
	assert.Equal(t, "http://browser:3000/sse", br["url"])
}

func TestGenerateMCPJSON_DefaultPort(t *testing.T) {
	servers := map[string]config.MCPSSE{
		"noport": {Image: "img:latest", Port: 0},
	}
	data, err := GenerateMCPJSON(nil, servers)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	mcpServers := result["mcpServers"].(map[string]interface{})
	np := mcpServers["noport"].(map[string]interface{})
	assert.Equal(t, "http://noport:8000/sse", np["url"])
}
```

- [ ] **Step 2: Implement `GenerateMCPJSON`**

```go
// MCPJSON represents the mcp.json structure mounted into the sandbox.
type MCPJSON struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// MCPServerEntry is a single entry in mcp.json.
// For SSE servers, only URL is set. For stdio servers, Command and Args are set.
type MCPServerEntry struct {
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// GenerateMCPJSON produces the mcp.json content for both stdio and SSE servers.
func GenerateMCPJSON(stdio map[string]config.MCPStdio, sse map[string]config.MCPSSE) ([]byte, error) {
	mcpJSON := MCPJSON{
		MCPServers: make(map[string]MCPServerEntry),
	}
	for name, srv := range stdio {
		mcpJSON.MCPServers[name] = MCPServerEntry{
			Command: srv.Command,
			Args:    srv.Args,
		}
	}
	for name, srv := range sse {
		mcpJSON.MCPServers[name] = MCPServerEntry{
			URL: EndpointURL(name, srv.Port),
		}
	}
	return json.MarshalIndent(mcpJSON, "", "  ")
}
```

- [ ] **Step 3: Integrate mcp.json generation into `ensureRunning`**

Write the generated `mcp.json` to a temp file and add it to the sandbox's bind mounts (or write it into a volume). The exact mount target depends on where Claude Code expects `mcp.json` — coordinate with the existing mount assembly in `internal/mount`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mcp/ -v && go test ./... && golangci-lint run`

---

### Task 9: End-to-end verification

**Dependencies:** All previous tasks

- [ ] **Step 1: Build and lint**

```bash
go build ./cmd/claustro/
go test ./...
golangci-lint run
```

- [ ] **Step 2: Manual integration test**

1. Create a test project with `claustro.yaml` containing an SSE MCP server:
   ```yaml
   mcp:
     sse:
       testserver:
         image: python:3.12-slim
         port: 8000
   ```
2. Run `claustro up`
3. Verify sibling container exists: check Docker containers list for `claustro-*_mcp-testserver`
4. Verify sibling is on the same network as the sandbox
5. Run `claustro burn` — verify sibling and sandbox are both removed
6. Run `claustro up` then `claustro nuke` — verify all containers and network cleaned up

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add SSE MCP sibling container support (M3)"
```
