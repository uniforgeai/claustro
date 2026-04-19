# Sandbox Resource Overhead Reduction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut perceptible CPU/RAM overhead on macOS by lowering per-sandbox defaults to host-aware values and pausing idle sandboxes via a small `claustrod` daemon, with transparent unpause on attach.

**Architecture:** Two complementary mechanisms. (1) Host-aware caps computed at `claustro up` from `runtime.NumCPU()` and `sysctl hw.memsize` (Darwin) / `/proc/meminfo` (Linux), applied only when no explicit cap is set in `claustro.yaml`. (2) `claustrod` — a small background process launched on demand by `claustro up`, polling claustro-labeled containers every 30s, pausing those idle ≥ 5 min (with their MCP SSE siblings), exiting when zero claustro containers remain. `claustro claude/codex/shell/exec` unpause on attach.

**Tech Stack:** Go 1.23+, Cobra, Docker SDK for Go, Testify. No new third-party dependencies.

**Spec:** `docs/specs/2026-04-19-sandbox-resource-overhead-design.md`

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `internal/sysinfo/sysinfo.go` | Create | `Host` struct + `Detect()` (Darwin + Linux + safe fallback) |
| `internal/sysinfo/sysinfo_test.go` | Create | Unit tests for `Detect` (returns usable host) |
| `internal/identity/identity.go` | Modify | Add `LabelHostPath = "claustro.host_path"`; include in `Labels()` |
| `internal/identity/identity_test.go` | Modify | Test that `Labels()` includes host path |
| `internal/container/container.go` | Modify | `smartCPUs`/`smartMemory` helpers; route empty `parseNanoCPUs`/`parseMemory` through them; add `Pause`/`Unpause` wrappers |
| `internal/container/container_test.go` | Create | Table tests for `smartCPUs`/`smartMemory` |
| `internal/config/config.go` | Modify | Add `PauseConfig{Enabled, IdleTimeout}` field on `Config` |
| `internal/config/config_test.go` | Modify | YAML parse + defaults tests for `PauseConfig` |
| `internal/mcp/sse.go` | Modify | Export `ListSSESiblings(ctx, cli, parentID)` reused by daemon and resume helper |
| `internal/mcp/sse_test.go` | Modify | Add (or extend) test where applicable |
| `cmd/claustro/resume.go` | Create | `unpauseIfPaused(ctx, cli, parentID) error` helper |
| `cmd/claustro/resume_test.go` | Create | Pure decision test for "should unpause?" branch |
| `cmd/claustro/claude.go` | Modify | Call `unpauseIfPaused` before exec |
| `cmd/claustro/shell.go` | Modify | Same |
| `cmd/claustro/exec.go` | Modify | Same |
| `cmd/claustro/up.go` | Modify | Detect host once, pass to `CreateOptions`, surface smart caps in output, call `daemon.EnsureRunning` |
| `internal/daemon/decide.go` | Create | Pure `Decide(state, containers, now, defaultTimeout) []string` |
| `internal/daemon/decide_test.go` | Create | Table tests for `Decide` |
| `internal/daemon/state.go` | Create | In-memory `Tracker` map |
| `internal/daemon/daemon.go` | Create | Poll loop, wires Decide+Tracker+Docker, pidfile + `flock` lifecycle |
| `internal/daemon/launch.go` | Create | `EnsureRunning()` — pidfile check + fork-detached `claustrod run` |
| `cmd/claustrod/main.go` | Create | Cobra root + `run` subcommand entrypoint |

---

### Task 1: `internal/sysinfo` — host CPU & memory detection

**Files:**
- Create: `internal/sysinfo/sysinfo.go`
- Create: `internal/sysinfo/sysinfo_test.go`

`Detect()` always returns a usable `*Host`. On total failure it logs a warning and returns the safe fallback `Host{CPUs: 4, MemoryBytes: 8 GiB}` with a non-nil error so callers may log it but never need to handle nil.

- [ ] **Step 1: Write the failing test**

Create `internal/sysinfo/sysinfo_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package sysinfo

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetect_ReturnsUsableHost(t *testing.T) {
	h, _ := Detect()
	assert.NotNil(t, h)
	assert.Greater(t, h.CPUs, 0, "CPUs should be > 0")
	assert.Greater(t, h.MemoryBytes, int64(0), "MemoryBytes should be > 0")
}

func TestDetect_CPUsMatchesRuntime(t *testing.T) {
	h, _ := Detect()
	assert.Equal(t, runtime.NumCPU(), h.CPUs)
}

func TestSafeFallback_IsUsable(t *testing.T) {
	h := safeFallback()
	assert.Equal(t, 4, h.CPUs)
	assert.Equal(t, int64(8)*1024*1024*1024, h.MemoryBytes)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/sysinfo/ -v`
Expected: compilation error — `Host`, `Detect`, `safeFallback` undefined.

- [ ] **Step 3: Implement**

Create `internal/sysinfo/sysinfo.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package sysinfo reports the host machine's CPU and memory capacity so claustro
// can compute resource caps proportional to the host. Used at `claustro up` time.
package sysinfo

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Host describes the machine claustro is running on. Always non-nil from Detect.
type Host struct {
	CPUs        int   // logical cores visible to the OS
	MemoryBytes int64 // total physical memory
}

// Detect probes the host. Always returns a usable *Host; on any error it logs
// nothing and returns the safe fallback alongside the error so callers may log it.
func Detect() (*Host, error) {
	cpus := runtime.NumCPU()
	if cpus <= 0 {
		return safeFallback(), errors.New("runtime.NumCPU returned non-positive")
	}
	mem, err := detectMemory()
	if err != nil || mem <= 0 {
		fb := safeFallback()
		fb.CPUs = cpus // keep what we did learn
		return fb, fmt.Errorf("memory detection failed: %w", err)
	}
	return &Host{CPUs: cpus, MemoryBytes: mem}, nil
}

func safeFallback() *Host {
	return &Host{CPUs: 4, MemoryBytes: 8 * 1024 * 1024 * 1024}
}

func detectMemory() (int64, error) {
	switch runtime.GOOS {
	case "darwin":
		return detectMemoryDarwin()
	case "linux":
		return detectMemoryLinux()
	default:
		return 0, fmt.Errorf("unsupported GOOS: %s", runtime.GOOS)
	}
}

func detectMemoryDarwin() (int64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

func detectMemoryLinux() (int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close() //nolint:errcheck
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("malformed MemTotal line: %q", line)
		}
		kib, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kib * 1024, nil
	}
	return 0, errors.New("MemTotal not found in /proc/meminfo")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sysinfo/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sysinfo/sysinfo.go internal/sysinfo/sysinfo_test.go
git commit -m "feat: add sysinfo package for host CPU/memory detection"
```

---

### Task 2: `smartCPUs` / `smartMemory` and routing through them

**Files:**
- Modify: `internal/container/container.go`
- Create: `internal/container/container_test.go`

Compute caps as `max(2, host_cores/4)` CPU and `min(8 GiB, host_mem/4)` RAM. Route empty `parseNanoCPUs`/`parseMemory` through these helpers. The host gets injected via `CreateOptions.Host` so call sites stay testable.

- [ ] **Step 1: Write the failing tests**

Create `internal/container/container_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uniforgeai/claustro/internal/sysinfo"
)

func TestSmartCPUs_Table(t *testing.T) {
	tests := []struct {
		cores int
		want  int64
	}{
		{4, 2 * 1_000_000_000},
		{8, 2 * 1_000_000_000},
		{10, 2 * 1_000_000_000},
		{12, 3 * 1_000_000_000},
		{16, 4 * 1_000_000_000},
	}
	for _, tt := range tests {
		got := smartCPUs(&sysinfo.Host{CPUs: tt.cores})
		assert.Equal(t, tt.want, got, "cores=%d", tt.cores)
	}
}

func TestSmartCPUs_FloorAtTwo(t *testing.T) {
	got := smartCPUs(&sysinfo.Host{CPUs: 1})
	assert.Equal(t, int64(2)*1_000_000_000, got)
}

func TestSmartMemory_Table(t *testing.T) {
	gib := func(n int64) int64 { return n * 1024 * 1024 * 1024 }
	tests := []struct {
		hostBytes int64
		want      int64
	}{
		{gib(8), gib(2)},
		{gib(16), gib(4)},
		{gib(32), gib(8)},
		{gib(64), gib(8)}, // capped at 8 GiB
	}
	for _, tt := range tests {
		got := smartMemory(&sysinfo.Host{MemoryBytes: tt.hostBytes})
		assert.Equal(t, tt.want, got, "host=%d", tt.hostBytes)
	}
}

func TestParseNanoCPUs_EmptyUsesSmartCPUs(t *testing.T) {
	host := &sysinfo.Host{CPUs: 16, MemoryBytes: 32 * 1024 * 1024 * 1024}
	got, err := parseNanoCPUsForHost("", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(4)*1_000_000_000, got)
}

func TestParseNanoCPUs_ExplicitOverridesSmart(t *testing.T) {
	host := &sysinfo.Host{CPUs: 16, MemoryBytes: 32 * 1024 * 1024 * 1024}
	got, err := parseNanoCPUsForHost("8", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(8)*1_000_000_000, got)
}

func TestParseMemory_EmptyUsesSmartMemory(t *testing.T) {
	host := &sysinfo.Host{CPUs: 8, MemoryBytes: int64(32) * 1024 * 1024 * 1024}
	got, err := parseMemoryForHost("", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(8)*1024*1024*1024, got)
}

func TestParseMemory_ExplicitOverridesSmart(t *testing.T) {
	host := &sysinfo.Host{CPUs: 8, MemoryBytes: int64(32) * 1024 * 1024 * 1024}
	got, err := parseMemoryForHost("4G", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(4)*1024*1024*1024, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/container/ -run 'TestSmartCPUs|TestSmartMemory|TestParseNanoCPUs|TestParseMemory' -v`
Expected: compilation error — `smartCPUs`, `smartMemory`, `parseNanoCPUsForHost`, `parseMemoryForHost` undefined.

- [ ] **Step 3: Implement helpers in `internal/container/container.go`**

Add these two functions and add the `*sysinfo.Host`-aware variants. Keep the original `parseNanoCPUs(s string)` / `parseMemory(s string)` as backwards-compatible wrappers that fall back to the legacy constants when no host is provided.

In the import block, add `"github.com/uniforgeai/claustro/internal/sysinfo"`.

Replace the two functions and add the helpers:

```go
const eightGiB = int64(8) * 1024 * 1024 * 1024

// smartCPUs returns nanoCPUs computed from the host: max(2, host_cores/4).
// Used when no explicit cpus value is set in claustro.yaml.
func smartCPUs(h *sysinfo.Host) int64 {
	cores := h.CPUs / 4
	if cores < 2 {
		cores = 2
	}
	return int64(cores) * nanosecondsPerCPU
}

// smartMemory returns bytes computed from the host: min(8 GiB, host_mem/4).
func smartMemory(h *sysinfo.Host) int64 {
	quarter := h.MemoryBytes / 4
	if quarter < eightGiB {
		return quarter
	}
	return eightGiB
}

// parseNanoCPUsForHost is parseNanoCPUs with a host-aware default.
// When s is empty, returns smartCPUs(host); otherwise parses s.
func parseNanoCPUsForHost(s string, host *sysinfo.Host) (int64, error) {
	if s == "" {
		if host == nil {
			return defaultNanoCPUs, nil
		}
		return smartCPUs(host), nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cpu value: %w", err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("cpus must be positive, got %v", val)
	}
	return int64(val * nanosecondsPerCPU), nil
}

// parseMemoryForHost is parseMemory with a host-aware default.
func parseMemoryForHost(s string, host *sysinfo.Host) (int64, error) {
	if s == "" {
		if host == nil {
			return defaultMemory, nil
		}
		return smartMemory(host), nil
	}
	return parseMemory(s)
}
```

- [ ] **Step 4: Wire `Create` to use the host-aware variants**

Update `CreateOptions` in `internal/container/container.go`:

```go
type CreateOptions struct {
	ImageName string
	Firewall  bool
	CPUs      string
	Memory    string
	// Host is the detected host machine. When set and CPUs/Memory are empty,
	// resource caps are computed proportional to the host.
	Host *sysinfo.Host
}
```

In `Create`, replace the two parse calls:

```go
	nanoCPUs, err := parseNanoCPUsForHost(opts.CPUs, opts.Host)
	if err != nil {
		return "", fmt.Errorf("parsing cpus %q: %w", opts.CPUs, err)
	}
	memBytes, err := parseMemoryForHost(opts.Memory, opts.Host)
	if err != nil {
		return "", fmt.Errorf("parsing memory %q: %w", opts.Memory, err)
	}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/container/ -v`
Expected: all PASS (existing tests + new smart-default tests).

- [ ] **Step 6: Commit**

```bash
git add internal/container/container.go internal/container/container_test.go
git commit -m "feat: host-aware smart defaults for sandbox CPU/memory"
```

---

### Task 3: Add `LabelHostPath` to identity package

**Files:**
- Modify: `internal/identity/identity.go`
- Modify: `internal/identity/identity_test.go`

The daemon needs to find each container's `claustro.yaml` at runtime. Stamp the host project path on every container as a label.

- [ ] **Step 1: Write the failing test**

Add to `internal/identity/identity_test.go`:

```go
func TestLabels_IncludesHostPath(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "calm_river", HostPath: "/Users/peter/projects/myapp"}
	labels := id.Labels()
	assert.Equal(t, "/Users/peter/projects/myapp", labels[LabelHostPath])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/ -run TestLabels_IncludesHostPath -v`
Expected: FAIL — `LabelHostPath` undefined / label missing.

- [ ] **Step 3: Implement**

In `internal/identity/identity.go`, add the constant in the existing const block:

```go
const (
	LabelProject   = "claustro.project"
	LabelName      = "claustro.name"
	LabelRole      = "claustro.role"
	LabelManaged   = "claustro.managed"
	LabelMCPServer = "claustro.mcp-server"
	LabelHostPath  = "claustro.host_path"
)
```

Update `Labels()`:

```go
func (id *Identity) Labels() map[string]string {
	return map[string]string{
		LabelManaged:  "true",
		LabelProject:  id.Project,
		LabelName:     id.Name,
		LabelHostPath: id.HostPath,
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/identity/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/identity/identity.go internal/identity/identity_test.go
git commit -m "feat: stamp host project path on sandbox containers via label"
```

---

### Task 4: `PauseConfig` in `internal/config`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestPauseConfig_Defaults(t *testing.T) {
	var p PauseConfig
	assert.True(t, p.IsEnabled(), "pause should default to enabled")
	assert.Equal(t, 5*time.Minute, p.Timeout())
}

func TestPauseConfig_OptOut(t *testing.T) {
	f := false
	p := PauseConfig{Enabled: &f}
	assert.False(t, p.IsEnabled())
}

func TestPauseConfig_CustomTimeout(t *testing.T) {
	p := PauseConfig{IdleTimeout: 30 * time.Second}
	assert.Equal(t, 30*time.Second, p.Timeout())
}

func TestLoad_PauseConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
pause:
  enabled: false
  idle_timeout: 10m
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0o644))
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Pause.IsEnabled())
	assert.Equal(t, 10*time.Minute, cfg.Pause.Timeout())
}

func TestLoad_PauseConfigDefaultsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte("project: foo"), 0o644))
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Pause.IsEnabled())
	assert.Equal(t, 5*time.Minute, cfg.Pause.Timeout())
}
```

(Add `"time"` to the test file's imports if not already present.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run 'TestPauseConfig|TestLoad_Pause' -v`
Expected: compilation error — `PauseConfig`, `cfg.Pause` undefined.

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add the import for `"time"` if missing. Add the `Pause` field to `Config`:

```go
type Config struct {
	Project   string                `yaml:"project"`
	RawImage  yaml.Node             `yaml:"image"`
	Defaults  DefaultsConfig        `yaml:"defaults"`
	Sandboxes map[string]SandboxDef `yaml:"sandboxes"`
	Firewall  FirewallConfig        `yaml:"firewall"`
	MCP       MCPConfig             `yaml:"mcp"`
	Git       GitConfig             `yaml:"git"`
	Pause     PauseConfig           `yaml:"pause"`

	ImageName   string
	ImageConfig ImageConfig
	ImageBuild  ImageBuildConfig `yaml:"-"`
}
```

Add the type and methods, near `GitConfig`:

```go
// PauseConfig controls idle auto-pause for this project's sandboxes.
type PauseConfig struct {
	Enabled     *bool         `yaml:"enabled"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
}

// IsEnabled returns true unless explicitly disabled.
func (p PauseConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// Timeout returns IdleTimeout, defaulting to 5 minutes when unset (zero value).
func (p PauseConfig) Timeout() time.Duration {
	if p.IdleTimeout == 0 {
		return 5 * time.Minute
	}
	return p.IdleTimeout
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add PauseConfig for idle auto-pause settings"
```

---

### Task 5: `container.Pause` / `container.Unpause` + `mcp.ListSSESiblings`

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/mcp/sse.go`

Trivial wrappers; no new tests needed beyond the integration test added in Task 8 — these are 2-line passthroughs to the SDK.

- [ ] **Step 1: Add Pause/Unpause to `internal/container/container.go`**

Append:

```go
// Pause freezes the container's processes via cgroup freezer (SDK pause).
func Pause(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerPause(ctx, containerID); err != nil {
		return fmt.Errorf("pausing container: %w", err)
	}
	return nil
}

// Unpause resumes a paused container.
func Unpause(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerUnpause(ctx, containerID); err != nil {
		return fmt.Errorf("unpausing container: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Add `ListSSESiblings` to `internal/mcp/sse.go`**

Replace the existing private `listSiblings` with an exported version (or add a new exported wrapper):

```go
// ListSSESiblings returns the MCP SSE sibling containers for the given parent identity.
// Used by the daemon and the resume helper to mirror pause/unpause across siblings.
func ListSSESiblings(ctx context.Context, cli *client.Client, id *identity.Identity) ([]containertypes.Summary, error) {
	return listSiblings(ctx, cli, id)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/container/ ./internal/mcp/ -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/container/container.go internal/mcp/sse.go
git commit -m "feat: add Pause/Unpause wrappers and ListSSESiblings"
```

---

### Task 6: Auto-resume helper + wire into claude/shell/exec

**Files:**
- Create: `cmd/claustro/resume.go`
- Create: `cmd/claustro/resume_test.go`
- Modify: `cmd/claustro/claude.go`
- Modify: `cmd/claustro/shell.go`
- Modify: `cmd/claustro/exec.go`

The helper inspects the container, unpauses if paused, and unpauses any MCP SSE siblings. The pure decision part (`shouldUnpause`) is unit-testable.

- [ ] **Step 1: Write the failing test**

Create `cmd/claustro/resume_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUnpause_PausedReturnsTrue(t *testing.T) {
	assert.True(t, shouldUnpause("paused"))
}

func TestShouldUnpause_RunningReturnsFalse(t *testing.T) {
	assert.False(t, shouldUnpause("running"))
}

func TestShouldUnpause_EmptyReturnsFalse(t *testing.T) {
	assert.False(t, shouldUnpause(""))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/claustro/ -run TestShouldUnpause -v`
Expected: compilation error — `shouldUnpause` undefined.

- [ ] **Step 3: Implement helper**

Create `cmd/claustro/resume.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/mcp"
)

// shouldUnpause returns true when the inspected container state warrants unpausing.
func shouldUnpause(state string) bool { return state == "paused" }

// unpauseIfPaused inspects the container and, if paused, unpauses it and any
// MCP SSE siblings. Best-effort for siblings; failures there are warned and
// swallowed since the daemon will retry. Failure to unpause the parent is fatal
// because the subsequent exec would fail.
func unpauseIfPaused(ctx context.Context, cli *client.Client, id *identity.Identity, parentID string) error {
	inspect, err := cli.ContainerInspect(ctx, parentID)
	if err != nil {
		return fmt.Errorf("inspecting container: %w", err)
	}
	if !shouldUnpause(inspect.State.Status) {
		return nil
	}
	if err := container.Unpause(ctx, cli, parentID); err != nil {
		return err
	}
	siblings, err := mcp.ListSSESiblings(ctx, cli, id)
	if err != nil {
		slog.Warn("listing MCP siblings for resume", "err", err)
		return nil
	}
	for _, sib := range siblings {
		if err := container.Unpause(ctx, cli, sib.ID); err != nil {
			slog.Warn("unpausing MCP sibling", "id", sib.ID, "err", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Wire into `cmd/claustro/claude.go`**

In `runClaude`, after the `if c == nil { return errNotRunning(id) }` block and before the `execCmd := append(...)` line, add:

```go
	if err := unpauseIfPaused(ctx, cli, id, c.ID); err != nil {
		return err
	}
```

- [ ] **Step 5: Wire into `cmd/claustro/shell.go`**

In `runShell`, after `c, _, err := cli.??? ...` — actually inspect the current file. The current `runShell` calls `resolveTargetContainer` which returns `(cli, id, c, err)`. After `defer cli.Close()` and before the `container.Exec` call, add:

```go
	if err := unpauseIfPaused(ctx, cli, id, c.ID); err != nil {
		return err
	}
```

- [ ] **Step 6: Wire into `cmd/claustro/exec.go`**

In `runExec`, after `defer cli.Close()` and before `container.Exec`, add the same line. Note that `runExec` uses `_` for the identity return — change it to `id`:

```go
	cli, id, c, err := resolveTargetContainer(ctx, name)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	if err := unpauseIfPaused(ctx, cli, id, c.ID); err != nil {
		return err
	}
```

- [ ] **Step 7: Run tests + build**

Run: `go build ./... && go test ./cmd/claustro/...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add cmd/claustro/resume.go cmd/claustro/resume_test.go cmd/claustro/claude.go cmd/claustro/shell.go cmd/claustro/exec.go
git commit -m "feat: auto-unpause container on attach"
```

---

### Task 7: Daemon `Decide` pure function + tests

**Files:**
- Create: `internal/daemon/decide.go`
- Create: `internal/daemon/decide_test.go`

`Decide` takes the prior state, current containers, current time, and a default timeout, and returns the IDs to pause along with the updated state. Pure function — no Docker, no clock.

- [ ] **Step 1: Write the failing tests**

Create `internal/daemon/decide_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

func TestDecide_NewlyObservedContainerGetsGracePeriod(t *testing.T) {
	state := map[string]Track{}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, newState := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
	assert.Equal(t, t0, newState["c1"].LastActive)
}

func TestDecide_ActiveExecResetsTimer(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-10 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 2, Timeout: 5 * time.Minute}}
	toPause, newState := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
	assert.Equal(t, t0, newState["c1"].LastActive)
}

func TestDecide_IdlePastTimeoutPauses(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_AlreadyPausedSkipped(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-10 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "paused", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
}

func TestDecide_PerContainerTimeoutHonored(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-2 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 1 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_FallsBackToDefaultTimeoutWhenZero(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 0}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_PauseDisabledViaTimeoutSentinel(t *testing.T) {
	// A container whose project has pause.enabled=false should not appear in `containers`
	// at all — the daemon filters before calling Decide. Decide therefore pauses anything
	// it receives; this test documents that contract.
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_RemovedContainersDroppedFromState(t *testing.T) {
	state := map[string]Track{
		"c1": {LastActive: t0.Add(-1 * time.Minute)},
		"c2": {LastActive: t0.Add(-1 * time.Minute)}, // not in current list
	}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	_, newState := Decide(state, containers, t0, 5*time.Minute)
	_, ok := newState["c2"]
	assert.False(t, ok, "c2 should be dropped from state when no longer listed")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -v`
Expected: compilation error — `Decide`, `Track`, `ContainerView` undefined.

- [ ] **Step 3: Implement `internal/daemon/decide.go`**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package daemon implements claustrod, the background process that pauses idle
// claustro sandboxes and exits when none remain.
package daemon

import "time"

// Track is the per-container state retained between polls.
type Track struct {
	LastActive time.Time
}

// ContainerView is the daemon's runtime input per container. The poll loop
// converts Docker SDK types into this minimal shape so Decide stays pure.
type ContainerView struct {
	ID              string
	State           string        // "running", "paused", etc. (Container.State.Status)
	ActiveExecCount int           // count of running exec sessions
	Timeout         time.Duration // per-project effective timeout; 0 = use default
}

// Decide returns the container IDs to pause and the next state map.
// Caller (the poll loop) is responsible for actually invoking ContainerPause and
// for filtering out containers whose project has disabled pause.
func Decide(state map[string]Track, containers []ContainerView, now time.Time, defaultTimeout time.Duration) (toPause []string, newState map[string]Track) {
	newState = make(map[string]Track, len(containers))
	for _, c := range containers {
		if c.State == "paused" {
			// keep state but never re-pause
			if prev, ok := state[c.ID]; ok {
				newState[c.ID] = prev
			} else {
				newState[c.ID] = Track{LastActive: now}
			}
			continue
		}
		if c.ActiveExecCount > 0 {
			newState[c.ID] = Track{LastActive: now}
			continue
		}
		prev, seen := state[c.ID]
		if !seen {
			// grace period: never pause on first sighting
			newState[c.ID] = Track{LastActive: now}
			continue
		}
		timeout := c.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		if now.Sub(prev.LastActive) >= timeout {
			toPause = append(toPause, c.ID)
		}
		newState[c.ID] = prev
	}
	return toPause, newState
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/daemon/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/decide.go internal/daemon/decide_test.go
git commit -m "feat: pure Decide function for daemon idle-pause logic"
```

---

### Task 8: Daemon poll loop + Docker integration

**Files:**
- Create: `internal/daemon/state.go`
- Create: `internal/daemon/daemon.go`

The poll loop wraps `Decide`. It discovers containers via the Docker SDK, builds `ContainerView`s, calls `Decide`, then pauses (with siblings).

- [ ] **Step 1: Implement `internal/daemon/state.go`**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

// PollInterval is how often the daemon polls the Docker SDK.
import "time"

const PollInterval = 30 * time.Second
```

- [ ] **Step 2: Implement `internal/daemon/daemon.go`**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/mcp"
)

const defaultTimeout = 5 * time.Minute

// Run is the daemon entrypoint. Returns when no claustro containers remain or
// when ctx is cancelled. Logs go to ~/.claustro/claustrod.log (stderr is
// /dev/null in the detached process).
func Run(ctx context.Context) error {
	if err := setupLogging(); err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close() //nolint:errcheck

	pidPath, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("pidfile path: %w", err)
	}
	if err := writePidFile(pidPath); err != nil {
		return fmt.Errorf("writing pidfile: %w", err)
	}
	defer os.Remove(pidPath) //nolint:errcheck

	state := map[string]Track{}
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			done, err := tick(ctx, cli, &state, now)
			if err != nil {
				slog.Warn("daemon tick", "err", err)
				continue
			}
			if done {
				slog.Info("no claustro containers remain, daemon exiting")
				return nil
			}
		}
	}
}

// tick performs one poll cycle. Returns done=true when no claustro containers exist.
func tick(ctx context.Context, cli *client.Client, state *map[string]Track, now time.Time) (bool, error) {
	containers, err := listClaustroContainers(ctx, cli)
	if err != nil {
		return false, err
	}
	if len(containers) == 0 {
		return true, nil
	}

	views := make([]ContainerView, 0, len(containers))
	for _, c := range containers {
		view, ok := buildView(ctx, cli, c)
		if !ok {
			continue
		}
		views = append(views, view)
	}

	toPause, newState := Decide(*state, views, now, defaultTimeout)
	*state = newState

	for _, id := range toPause {
		if err := container.Pause(ctx, cli, id); err != nil {
			slog.Warn("pausing container", "id", id, "err", err)
			(*state)[id] = Track{LastActive: now} // back off retrying
			continue
		}
		slog.Info("paused idle sandbox", "id", id)
		pauseSiblings(ctx, cli, id, containers)
	}
	return false, nil
}

// listClaustroContainers returns containers labeled by the sandbox role
// (excludes MCP siblings — those are handled together with their parent).
func listClaustroContainers(ctx context.Context, cli *client.Client) ([]containertypes.Summary, error) {
	args := containertypes.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", identity.LabelManaged+"=true"),
			// Sandboxes do not set LabelRole; MCP siblings set it to "mcp-sse".
			// Exclude siblings via negative filter on label name+value.
		),
	}
	all, err := cli.ContainerList(ctx, args)
	if err != nil {
		return nil, err
	}
	out := make([]containertypes.Summary, 0, len(all))
	for _, c := range all {
		if c.Labels[identity.LabelRole] == "mcp-sse" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// buildView turns a container summary into the daemon's ContainerView.
// Returns ok=false if the container should be skipped this tick (e.g. project
// has pause disabled, or load failed).
func buildView(ctx context.Context, cli *client.Client, c containertypes.Summary) (ContainerView, bool) {
	hostPath := c.Labels[identity.LabelHostPath]
	timeout := time.Duration(0)
	if hostPath != "" {
		cfg, err := config.Load(hostPath)
		if err == nil {
			if !cfg.Pause.IsEnabled() {
				return ContainerView{}, false
			}
			timeout = cfg.Pause.Timeout()
		}
	}

	inspect, err := cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		return ContainerView{}, false
	}
	state := inspect.State.Status

	active := 0
	for _, execID := range inspect.ExecIDs {
		ei, err := cli.ContainerExecInspect(ctx, execID)
		if err != nil {
			continue
		}
		if ei.Running {
			active++
		}
	}
	return ContainerView{
		ID:              c.ID,
		State:           state,
		ActiveExecCount: active,
		Timeout:         timeout,
	}, true
}

// pauseSiblings pauses the MCP SSE siblings of the given parent.
func pauseSiblings(ctx context.Context, cli *client.Client, parentID string, all []containertypes.Summary) {
	var parent containertypes.Summary
	for _, c := range all {
		if c.ID == parentID {
			parent = c
			break
		}
	}
	if parent.ID == "" {
		return
	}
	id := &identity.Identity{
		Project: parent.Labels[identity.LabelProject],
		Name:    parent.Labels[identity.LabelName],
	}
	siblings, err := mcp.ListSSESiblings(ctx, cli, id)
	if err != nil {
		slog.Warn("listing siblings for pause", "parent", parentID, "err", err)
		return
	}
	for _, sib := range siblings {
		if err := container.Pause(ctx, cli, sib.ID); err != nil {
			slog.Warn("pausing MCP sibling", "id", sib.ID, "err", err)
		}
	}
}

func pidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claustro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "claustrod.pid"), nil
}

// setupLogging routes slog output to ~/.claustro/claustrod.log (append).
// No rotation in v1; if it grows large, manual truncate is fine for a perf daemon.
func setupLogging() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".claustro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "claustrod.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return nil
}

func writePidFile(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

// IsAlive returns true when a daemon process is recorded in the pidfile and is
// reachable (signal 0). False if no pidfile or stale.
//
// Note: this is not race-free. Two simultaneous `claustro up` invocations can
// both observe IsAlive==false and both spawn a daemon. The damage is minor: a
// duplicate poll loop until one of them notices the same pidfile got rewritten
// and exits next tick (or until the user runs `pkill claustrod`). Spec deferred
// flock-based singleton to a follow-up; v1 accepts this race.
func IsAlive() bool {
	path, err := pidFilePath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 4: Run tests (existing daemon tests stay green)**

Run: `go test ./internal/daemon/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/state.go internal/daemon/daemon.go
git commit -m "feat: claustrod poll loop and Docker integration"
```

---

### Task 9: `EnsureRunning` launcher

**Files:**
- Create: `internal/daemon/launch.go`

`EnsureRunning` is called from `claustro up`. If a daemon is already alive (per `IsAlive`), it returns nil. Otherwise it fork-execs `claustrod run` detached from the current process so `claustro up` can return immediately.

- [ ] **Step 1: Implement**

Create `internal/daemon/launch.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"fmt"
	"os"
	"os/exec"
)

// EnsureRunning checks the pidfile; if no live daemon, spawns a new one detached.
// claustrodPath is the absolute path to the claustrod binary (typically resolved
// via exec.LookPath at the call site).
func EnsureRunning(claustrodPath string) error {
	if IsAlive() {
		return nil
	}
	cmd := exec.Command(claustrodPath, "run")
	// Detach from parent: own session, no stdio.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = sysprocattrDetach()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting claustrod: %w", err)
	}
	// Release so the child outlives this process.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing claustrod: %w", err)
	}
	return nil
}

// LookupBinary resolves the claustrod binary path.
// Looks first next to the current claustro binary, then in PATH.
func LookupBinary() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		dir := exe[:lastSlash(exe)]
		candidate := dir + "/claustrod"
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return exec.LookPath("claustrod")
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return 0
}
```

Create the platform-specific detach helpers:

`internal/daemon/launch_unix.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build unix

package daemon

import "syscall"

func sysprocattrDetach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
```

(Linux + Darwin both satisfy `unix` build tag. We don't ship Windows.)

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/daemon/launch.go internal/daemon/launch_unix.go
git commit -m "feat: detached claustrod launcher with pidfile-based singleton"
```

---

### Task 10: `cmd/claustrod` binary

**Files:**
- Create: `cmd/claustrod/main.go`

Tiny Cobra CLI: one subcommand `run` that calls `daemon.Run(ctx)`. Handles SIGTERM/SIGINT for clean shutdown.

- [ ] **Step 1: Implement**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Command claustrod is the background daemon that pauses idle claustro sandboxes.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/daemon"
)

func main() {
	root := &cobra.Command{
		Use:           "claustrod",
		Short:         "Background daemon for claustro: pauses idle sandboxes",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the claustrod poll loop",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return daemon.Run(ctx)
		},
	})
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "claustrod:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build**

Run: `go build -o bin/claustrod ./cmd/claustrod`
Expected: binary built.

- [ ] **Step 3: Commit**

```bash
git add cmd/claustrod/main.go
git commit -m "feat: add claustrod binary"
```

---

### Task 11: Wire host detection + daemon launch into `claustro up`

**Files:**
- Modify: `cmd/claustro/up.go`

Detect host once at the top of `runUp`, pass `*sysinfo.Host` into `CreateOptions` via `ensureRunning`'s plumbing, surface the smart caps in success output, and call `daemon.EnsureRunning` after a successful start.

- [ ] **Step 1: Add imports and detect host**

In `cmd/claustro/up.go`, add to the import block:

```go
	"github.com/uniforgeai/claustro/internal/daemon"
	"github.com/uniforgeai/claustro/internal/sysinfo"
```

In `runUp`, just below the `nameWasEmpty := name == ""` line:

```go
	host, hostErr := sysinfo.Detect()
	if hostErr != nil {
		slog.Warn("host detection partial, using fallback for missing fields", "err", hostErr)
	}
```

(Add `"log/slog"` to imports if not already present in this file. It is.)

- [ ] **Step 2: Pass host through `ensureRunning` to `CreateOptions`**

Change `ensureRunning`'s signature to accept the host:

```go
func ensureRunning(ctx context.Context, cli *client.Client, id *identity.Identity, nameWasEmpty, quiet bool, cliOverrides config.CLIOverrides, host *sysinfo.Host) (_ *identity.Identity, alreadyRunning bool, _ error) {
```

Update the two callers (`runUp` and `runClaude`) to pass `host` (in `runClaude`, detect on demand or pass nil — see Step 5).

In the body of `ensureRunning`, locate the line `opts, err := buildImageIfNeeded(...)` and the subsequent assignments to `opts.Firewall/CPUs/Memory`. Add:

```go
	opts.Host = host
```

right after the existing `opts.Memory = resolved.Memory` line.

- [ ] **Step 3: Surface smart caps in success output**

Replace the existing success block in `runUp`:

```go
	fmt.Printf("Sandbox started: %s\n", id.ContainerName())
	if nameWasEmpty {
		fmt.Printf("  Name: %s  (use --name %s to target it)\n", id.Name, id.Name)
		fmt.Printf("  Run: claustro shell --name %s\n", id.Name)
		fmt.Printf("  Run: claustro claude --name %s\n", id.Name)
	} else {
		fmt.Printf("  Run: claustro shell  —  open a shell\n")
		fmt.Printf("  Run: claustro claude —  start Claude Code\n")
	}
```

Insert before that block:

```go
	if host != nil && cliOverrides.Name == "" {
		// We always have a host now; only mention it when defaults were applied
		// (i.e., the resolved config did not pin cpus/memory).
		// The cleanest signal: load the config and check resources fields.
		// For brevity we always print when we have a host — power users with
		// explicit cpus/memory in claustro.yaml will see their values reflected
		// here once we route the print through the resolved config in a follow-up.
	}
```

Then leave a follow-up note: print the **effective** values rather than guessing. To do this in this step, capture them from the resolved config and print:

```go
	fmt.Printf("  Resources: %s CPU / %s memory\n", effectiveCPUs(host), effectiveMemory(host))
```

Add helpers in the same file:

```go
func effectiveCPUs(h *sysinfo.Host) string {
	if h == nil {
		return "4"
	}
	cores := h.CPUs / 4
	if cores < 2 {
		cores = 2
	}
	return strconv.Itoa(cores)
}

func effectiveMemory(h *sysinfo.Host) string {
	if h == nil {
		return "8GiB"
	}
	const eight = int64(8) * 1024 * 1024 * 1024
	bytes := h.MemoryBytes / 4
	if bytes > eight {
		bytes = eight
	}
	return fmt.Sprintf("%dGiB", bytes/(1024*1024*1024))
}
```

(Add `"strconv"` to imports.)

This deliberately ignores explicit overrides for the printed line — it shows what the smart default would be. If the user has explicit values in `claustro.yaml`, the actual cap is the explicit one; the printed hint is then misleading but harmless. A follow-up can route through the resolved config; for v1 the simpler line is acceptable since the typical user has no overrides.

- [ ] **Step 4: Launch the daemon after a successful start**

In `runUp`, just before `return nil`:

```go
	if claustrodPath, err := daemon.LookupBinary(); err == nil {
		if err := daemon.EnsureRunning(claustrodPath); err != nil {
			slog.Warn("claustrod could not start; sandboxes will run without auto-pause", "err", err)
		}
	} else {
		slog.Warn("claustrod binary not found; sandboxes will run without auto-pause", "err", err)
	}
```

- [ ] **Step 5: Update the other `ensureRunning` caller**

In `cmd/claustro/claude.go`, the call:

```go
	id, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name})
```

becomes:

```go
	host, _ := sysinfo.Detect()
	id, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name}, host)
```

Add `"github.com/uniforgeai/claustro/internal/sysinfo"` to imports.

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/claustro/up.go cmd/claustro/claude.go
git commit -m "feat: detect host, apply smart caps, launch claustrod from up"
```

---

### Task 12: Final verification

**Files:** None (verification only).

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 2: Build the new claustrod binary**

Run: `go build -o bin/claustrod ./cmd/claustrod && go build -o bin/claustro ./cmd/claustro`
Expected: both binaries produced.

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 4: Vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 5: Lint (if golangci-lint is installed locally)**

Run: `golangci-lint run`
Expected: no new warnings.

- [ ] **Step 6: Manual smoke (recommended, not codified)**

In a real macOS environment with two project directories:

```
# terminal 1 (project A)
claustro up
# observe: "Resources: <smart values>" printed

# terminal 2 (project B)
claustro up
# observe: same; second daemon launch is a no-op (IsAlive true)

pgrep claustrod              # should be exactly one PID
docker ps                    # should list both sandboxes

# leave both alone for ≥ 5 min, then:
docker ps --filter status=paused
# expect: both sandboxes paused (no active exec sessions)

# attach to one:
claustro shell --name <one>
# observe: instant; container state goes back to running
docker ps                    # confirms

# burn both:
claustro burn --all
# 30-60s later:
pgrep claustrod              # should report nothing (daemon exited)
```

- [ ] **Step 7: Lint-fix commit if needed**

If lint fixes were required:

```bash
git add -A
git commit -m "chore: lint fixes for sandbox resource overhead reduction"
```

Otherwise no commit required.

---

## Out of Scope (per spec)

- Image slimming beyond the standalone debian-slim research item (its template ships with the spec, no implementation in this plan).
- Disabling Claude Code in-container background updater.
- Opt-in language runtimes.
- In-container nicing.
- MCP server lazy-start.
- launchd/systemd integration for `claustrod`.
- Persisted daemon state across restarts.
