# `claustro codex` Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a first-class `claustro codex` subcommand symmetric to `claustro claude`, with a shared `runAgent` helper that consolidates identity resolution, auto-up, and a new disabled-agent pre-flight check.

**Architecture:** Extract a shared command-layer helper in `cmd/claustro/agent.go` that both `claude.go` and the new `codex.go` delegate to. Surface the resolved `*config.Config` from `ensureRunning` (already loaded internally) so the helper can pre-check `IsAgentEnabled("codex")` without re-loading. Parallel `claude` + `codex` sessions reuse the existing single container via independent `docker exec` sessions — falls out of the existing architecture for free.

**Tech Stack:** Go 1.23+, Cobra (CLI), Docker SDK for Go, Testify.

**Spec:** `docs/specs/2026-04-17-codex-command-design.md`

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `cmd/claustro/agent.go` | Create | `AgentSpec` type, `runAgent`, `buildAgentCmd`, `checkAgentEnabled`, `claudeSpec`, `codexSpec` |
| `cmd/claustro/agent_test.go` | Create | Pure-function tests for `buildAgentCmd`, `checkAgentEnabled`, spec contents |
| `cmd/claustro/claude.go` | Modify | `runClaude` collapses to a thin call into `runAgent(claudeSpec)` |
| `cmd/claustro/codex.go` | Create | `newCodexCmd`, mirrors `newClaudeCmd` but uses `codexSpec` |
| `cmd/claustro/commands.go` | Modify | Register `newCodexCmd()` |
| `cmd/claustro/commands_test.go` | Modify | Add `codex` to `TestSetupCommands_RegistersAllCommands` and add `TestCodexCmd_Defaults` |
| `cmd/claustro/up.go` | Modify | `ensureRunning` returns `*config.Config`; add `claustro codex` hint to success output |

---

### Task 1: Extend `ensureRunning` to return the loaded `*config.Config`

**Files:**
- Modify: `cmd/claustro/up.go` (function `ensureRunning` and its caller `runUp`; also `runClaude` in `cmd/claustro/claude.go` which is the other current caller)

This is a pure refactor: `ensureRunning` already loads `config.Config` internally to drive the auto-up path; we move that load to the very top so it runs even when the container is already running, and surface it to the caller. No behavior change.

- [ ] **Step 1: Update `ensureRunning` signature and body**

In `cmd/claustro/up.go`, change the signature from:

```go
func ensureRunning(ctx context.Context, cli *client.Client, id *identity.Identity, nameWasEmpty, quiet bool, cliOverrides config.CLIOverrides) (_ *identity.Identity, alreadyRunning bool, _ error) {
```

to:

```go
func ensureRunning(ctx context.Context, cli *client.Client, id *identity.Identity, nameWasEmpty, quiet bool, cliOverrides config.CLIOverrides) (_ *identity.Identity, _ *config.Config, alreadyRunning bool, _ error) {
```

Move the `config.Load(id.HostPath)` call to the top of the function (before the existing `container.FindByIdentity`). Use the loaded `cfg` for the rest of the function. Return `(id, cfg, true/false, nil)` from each return point; return `(nil, nil, false, err)` on error.

The full body becomes:

```go
func ensureRunning(ctx context.Context, cli *client.Client, id *identity.Identity, nameWasEmpty, quiet bool, cliOverrides config.CLIOverrides) (_ *identity.Identity, _ *config.Config, alreadyRunning bool, _ error) {
	cfg, err := config.Load(id.HostPath)
	if err != nil {
		return nil, nil, false, fmt.Errorf("loading config: %w", err)
	}

	existing, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return nil, nil, false, fmt.Errorf("finding sandbox: %w", err)
	}
	if existing != nil && strings.Contains(existing.Status, "Up") {
		if !quiet {
			fmt.Printf("Sandbox %q is already running (%s)\n", id.ContainerName(), existing.Status)
		}
		return id, cfg, true, nil
	}

	// If the name was auto-generated and a container with that name already exists,
	// retry with a new random name. HostPath is CWD-derived and unchanged by rename,
	// so the cfg loaded above is still valid.
	if nameWasEmpty && existing != nil {
		id, err = generateUniqueName(ctx, cli)
		if err != nil {
			return nil, nil, false, err
		}
	}

	if quiet {
		fmt.Fprintf(os.Stderr, "Starting sandbox %s...\n", id.ContainerName())
	}

	dotenv, err := config.LoadDotenv(id.HostPath)
	if err != nil {
		return nil, nil, false, fmt.Errorf("loading .env: %w", err)
	}

	resolved, err := cfg.Resolve(id.HostPath, cliOverrides, dotenv)
	if err != nil {
		return nil, nil, false, fmt.Errorf("resolving config: %w", err)
	}
	slog.Debug("resolved sandbox config",
		"name", resolved.Name,
		"workdir", resolved.Workdir,
		"mounts", len(resolved.Mounts),
		"env_vars", len(resolved.Env),
		"image", resolved.ImageName,
	)

	opts, err := buildImageIfNeeded(ctx, cli, id, cfg)
	if err != nil {
		return nil, nil, false, err
	}
	opts.Firewall = resolved.Firewall
	opts.CPUs = resolved.CPUs
	opts.Memory = resolved.Memory

	mounts, err := setupVolumesAndMounts(ctx, cli, id, cfg, resolved)
	if err != nil {
		return nil, nil, false, err
	}

	slog.Info("creating sandbox", "container", id.ContainerName())
	containerID, err := container.Create(ctx, cli, id, mounts, opts)
	if err != nil {
		return nil, nil, false, fmt.Errorf("creating container: %w", err)
	}
	if err := container.Start(ctx, cli, containerID); err != nil {
		return nil, nil, false, fmt.Errorf("starting container: %w", err)
	}

	// Start MCP SSE sibling containers (non-fatal on failure).
	if len(cfg.MCP.SSE) > 0 {
		mcp.StartSSESiblings(ctx, cli, id, cfg.MCP.SSE)
	}

	// Write MCP config into the container.
	if err := writeMCPConfig(ctx, cli, containerID, cfg, resolved.IsolatedState); err != nil {
		slog.Warn("failed to write MCP config", "err", err)
	}

	if err := applyFirewall(ctx, cli, containerID, id, cfg, resolved.Firewall); err != nil {
		return nil, nil, false, err
	}

	return id, cfg, false, nil
}
```

Delete the now-duplicated `cfg, err := config.Load(...)` further down in the body.

- [ ] **Step 2: Update `runUp` (the other caller) for the new signature**

In `cmd/claustro/up.go`, change:

```go
	id, alreadyRunning, err := ensureRunning(ctx, cli, id, nameWasEmpty, false, cliOverrides)
```

to:

```go
	id, _, alreadyRunning, err := ensureRunning(ctx, cli, id, nameWasEmpty, false, cliOverrides)
```

- [ ] **Step 3: Update `runClaude` for the new signature (temporary; it will be deleted in Task 4)**

In `cmd/claustro/claude.go`, change:

```go
	id, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name})
```

to:

```go
	id, _, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name})
```

- [ ] **Step 4: Verify build and tests still pass**

Run: `go build ./... && go test ./cmd/claustro/...`
Expected: build succeeds, all existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/claustro/up.go cmd/claustro/claude.go
git commit -m "refactor: surface loaded config from ensureRunning"
```

---

### Task 2: Add `cmd/claustro/agent.go` with pure helpers and tests

**Files:**
- Create: `cmd/claustro/agent.go`
- Create: `cmd/claustro/agent_test.go`

This task adds the pure (Docker-free, Cobra-free) building blocks and their tests. `runAgent` and the registered command come in later tasks.

- [ ] **Step 1: Write the failing tests**

Create `cmd/claustro/agent_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func ptrBool(b bool) *bool { return &b }

func TestBuildAgentCmd_OrderingClaude(t *testing.T) {
	got := buildAgentCmd(claudeSpec, []string{"--model", "opus", "fix bug"})
	assert.Equal(t, []string{
		"claude",
		"--dangerously-skip-permissions",
		"--model", "opus", "fix bug",
	}, got)
}

func TestBuildAgentCmd_OrderingCodex(t *testing.T) {
	got := buildAgentCmd(codexSpec, []string{"--model", "o3"})
	assert.Equal(t, []string{
		"codex",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "o3",
	}, got)
}

func TestBuildAgentCmd_NoExtraArgs(t *testing.T) {
	got := buildAgentCmd(codexSpec, nil)
	assert.Equal(t, []string{
		"codex",
		"--dangerously-bypass-approvals-and-sandbox",
	}, got)
}

func TestAgentSpec_Claude(t *testing.T) {
	assert.Equal(t, "claude", claudeSpec.Name)
	assert.Equal(t, "", claudeSpec.ConfigKey, "claude has no config gate")
	assert.Contains(t, claudeSpec.DefaultArgs, "--dangerously-skip-permissions")
	assert.NotEmpty(t, claudeSpec.DisplayName)
}

func TestAgentSpec_Codex(t *testing.T) {
	assert.Equal(t, "codex", codexSpec.Name)
	assert.Equal(t, "codex", codexSpec.ConfigKey)
	assert.Contains(t, codexSpec.DefaultArgs, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotEmpty(t, codexSpec.DisplayName)
}

func TestCheckAgentEnabled_NilConfigKeySkipsCheck(t *testing.T) {
	cfg := &config.Config{}
	err := checkAgentEnabled(cfg, claudeSpec) // claudeSpec.ConfigKey == ""
	require.NoError(t, err)
}

func TestCheckAgentEnabled_EnabledByDefault(t *testing.T) {
	cfg := &config.Config{ImageBuild: config.DefaultImageBuildConfig()}
	err := checkAgentEnabled(cfg, codexSpec)
	require.NoError(t, err)
}

func TestCheckAgentEnabled_DisabledReturnsHelpfulError(t *testing.T) {
	cfg := &config.Config{
		ImageBuild: config.ImageBuildConfig{
			Agents: config.AgentsConfig{Codex: ptrBool(false)},
		},
	}
	err := checkAgentEnabled(cfg, codexSpec)
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "codex is disabled")
	assert.Contains(t, msg, "claustro.yaml")
	assert.Contains(t, msg, "image.agents.codex")
	assert.Contains(t, msg, "claustro rebuild")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/claustro/ -run 'TestBuildAgentCmd|TestAgentSpec|TestCheckAgentEnabled' -v`
Expected: compilation error — `buildAgentCmd`, `claudeSpec`, `codexSpec`, `checkAgentEnabled`, `AgentSpec` undefined.

- [ ] **Step 3: Create `cmd/claustro/agent.go` with the helpers and specs**

Create `cmd/claustro/agent.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"

	"github.com/uniforgeai/claustro/internal/config"
)

// AgentSpec describes how a coding agent (Claude, Codex, ...) is launched
// inside a sandbox container.
type AgentSpec struct {
	// Name is the binary name invoked inside the container, e.g. "claude" or "codex".
	Name string
	// ConfigKey is the key passed to ImageBuildConfig.IsAgentEnabled to decide
	// whether the agent is installed in the image. Empty string disables the check
	// (used for agents that are unconditionally part of the image, like Claude Code).
	ConfigKey string
	// DefaultArgs are prepended after Name and before any caller-supplied extra args.
	DefaultArgs []string
	// DisplayName is the human-readable name used in error/help messages.
	DisplayName string
}

// claudeSpec launches Claude Code inside the sandbox.
// Claude Code is unconditionally installed in the image and has no config gate.
var claudeSpec = AgentSpec{
	Name:        "claude",
	ConfigKey:   "",
	DefaultArgs: []string{"--dangerously-skip-permissions"},
	DisplayName: "Claude Code",
}

// codexSpec launches OpenAI's Codex CLI inside the sandbox.
// Codex is opt-out via image.agents.codex in claustro.yaml.
// The sandbox container is itself an externally sandboxed environment, so we use
// the bypass flag whose own help text reads "Intended solely for running in
// environments that are externally sandboxed."
var codexSpec = AgentSpec{
	Name:        "codex",
	ConfigKey:   "codex",
	DefaultArgs: []string{"--dangerously-bypass-approvals-and-sandbox"},
	DisplayName: "Codex CLI",
}

// buildAgentCmd composes the in-container command line: [Name, DefaultArgs..., extraArgs...].
func buildAgentCmd(spec AgentSpec, extraArgs []string) []string {
	cmd := make([]string, 0, 1+len(spec.DefaultArgs)+len(extraArgs))
	cmd = append(cmd, spec.Name)
	cmd = append(cmd, spec.DefaultArgs...)
	cmd = append(cmd, extraArgs...)
	return cmd
}

// checkAgentEnabled returns a helpful error when the spec has a ConfigKey and the
// agent is disabled in the project's claustro.yaml. Returns nil when ConfigKey is
// empty (agent has no config gate) or when the agent is enabled.
func checkAgentEnabled(cfg *config.Config, spec AgentSpec) error {
	if spec.ConfigKey == "" {
		return nil
	}
	if cfg.ImageBuild.IsAgentEnabled(spec.ConfigKey) {
		return nil
	}
	return fmt.Errorf(
		"%s is disabled in claustro.yaml (image.agents.%s: false). "+
			"Enable it and run 'claustro rebuild', or run 'claustro shell' to use other tools.",
		spec.Name, spec.ConfigKey,
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/claustro/ -run 'TestBuildAgentCmd|TestAgentSpec|TestCheckAgentEnabled' -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/claustro/agent.go cmd/claustro/agent_test.go
git commit -m "feat: add AgentSpec and pure helpers for agent commands"
```

---

### Task 3: Add `runAgent` to `agent.go`

**Files:**
- Modify: `cmd/claustro/agent.go`

`runAgent` is the consolidated body of the current `runClaude`, with the disabled-agent check inserted after `ensureRunning`. We add it without callers in this task and wire `claude.go` and `codex.go` to it in Tasks 4 and 5.

`runAgent` is hard to unit-test directly (touches Docker). Coverage for its building blocks (`buildAgentCmd`, `checkAgentEnabled`) was added in Task 2; the integration is covered by manual end-to-end testing pre-merge.

- [ ] **Step 1: Add `runAgent` and required imports**

Append to `cmd/claustro/agent.go`. Update the import block at the top of the file to include the new packages:

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)
```

Append the function:

```go
// runAgent is the shared entry point for `claustro claude` and `claustro codex`.
// It resolves the target sandbox, ensures it is running, optionally checks that
// the agent is enabled in the project config, and execs the agent inside the
// container with an interactive TTY and clipboard bridge.
func runAgent(ctx context.Context, nameFlag string, spec AgentSpec, extraArgs []string) error {
	nameWasEmpty := nameFlag == ""

	id, err := identity.FromCWD(nameFlag)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	// Auto-select sandbox by name when none was supplied.
	if nameWasEmpty {
		containers, err := container.ListByProject(ctx, cli, id.Project, false)
		if err != nil {
			return fmt.Errorf("listing sandboxes: %w", err)
		}
		switch len(containers) {
		case 0:
			// No sandbox running — fall through to auto-up.
		case 1:
			resolvedName := containers[0].Labels["claustro.name"]
			id, err = identity.FromCWD(resolvedName)
			if err != nil {
				return fmt.Errorf("resolving identity: %w", err)
			}
		default:
			names := make([]string, len(containers))
			for i, c := range containers {
				names[i] = "  " + c.Labels["claustro.name"]
			}
			return fmt.Errorf("multiple sandboxes running, specify --name:\n%s", strings.Join(names, "\n"))
		}
	}

	id, cfg, _, err := ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: nameFlag})
	if err != nil {
		return err
	}

	if err := checkAgentEnabled(cfg, spec); err != nil {
		return err
	}

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	execCmd := buildAgentCmd(spec, extraArgs)
	sockDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	return container.Exec(ctx, cli, c.ID, execCmd, container.ExecOptions{
		Interactive:      true,
		ClipboardSockDir: sockDir,
	})
}
```

- [ ] **Step 2: Verify build still passes**

Run: `go build ./...`
Expected: build succeeds (no callers yet, but the function and its imports compile).

- [ ] **Step 3: Run all tests to verify nothing regressed**

Run: `go test ./cmd/claustro/...`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/claustro/agent.go
git commit -m "feat: add runAgent shared helper for agent commands"
```

---

### Task 4: Refactor `claude.go` to delegate to `runAgent`

**Files:**
- Modify: `cmd/claustro/claude.go`

After this task, `claude.go` is a thin Cobra wrapper. The old `runClaude` is removed; behavior is preserved via `runAgent(claudeSpec, ...)`. Existing tests in `cmd/claustro/commands_test.go` (`TestClaudeCmd_DescribesAutoUp`, `TestClaudeCmd_Defaults`) still cover the command surface.

- [ ] **Step 1: Replace the contents of `cmd/claustro/claude.go`**

Replace the entire file with:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"github.com/spf13/cobra"
)

func newClaudeCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Launch Claude Code inside a sandbox",
		Long:  "Runs 'claude --dangerously-skip-permissions' inside the sandbox. Automatically starts a sandbox if none is running. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(cmd.Context(), name, claudeSpec, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}
```

- [ ] **Step 2: Verify build passes**

Run: `go build ./...`
Expected: build succeeds.

- [ ] **Step 3: Run tests**

Run: `go test ./cmd/claustro/...`
Expected: all PASS, including the existing `TestClaudeCmd_DescribesAutoUp` and `TestClaudeCmd_Defaults`.

- [ ] **Step 4: Commit**

```bash
git add cmd/claustro/claude.go
git commit -m "refactor: collapse claude command onto runAgent helper"
```

---

### Task 5: Add `cmd/claustro/codex.go` and register the command

**Files:**
- Create: `cmd/claustro/codex.go`
- Modify: `cmd/claustro/commands.go`
- Modify: `cmd/claustro/commands_test.go`

- [ ] **Step 1: Write the failing test**

In `cmd/claustro/commands_test.go`:

(a) Add `"codex"` to the `expected` slice in `TestSetupCommands_RegistersAllCommands`. Update the line:

```go
		expected := []string{"burn", "claude", "config", "doctor", "exec", "init", "logs", "ls", "nuke", "rebuild", "shell", "status", "up", "validate"}
```

to:

```go
		expected := []string{"burn", "claude", "codex", "config", "doctor", "exec", "init", "logs", "ls", "nuke", "rebuild", "shell", "status", "up", "validate"}
```

(b) Add a new test below `TestClaudeCmd_Defaults`:

```go
func TestCodexCmd_DescribesAutoUp(t *testing.T) {
	root := makeRoot()
	cmd := findSubcmd(root, "codex")
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Long, "Automatically starts a sandbox if none is running")
}

func TestCodexCmd_Defaults(t *testing.T) {
	cmd := newCodexCmd()
	f := cmd.Flags().Lookup("name")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/claustro/ -run 'TestCodexCmd|TestSetupCommands' -v`
Expected: compilation error (`newCodexCmd` undefined) and `TestSetupCommands_RegistersAllCommands/codex` FAIL.

- [ ] **Step 3: Create `cmd/claustro/codex.go`**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"github.com/spf13/cobra"
)

func newCodexCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Launch Codex CLI inside a sandbox",
		Long:  "Runs 'codex --dangerously-bypass-approvals-and-sandbox' inside the sandbox. Automatically starts a sandbox if none is running. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(cmd.Context(), name, codexSpec, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}
```

- [ ] **Step 4: Register the command in `cmd/claustro/commands.go`**

Add the `newCodexCmd()` line after `newClaudeCmd()`:

```go
func setupCommands(root *cobra.Command) {
	root.AddCommand(newInitCmd())
	root.AddCommand(newUpCmd())
	root.AddCommand(newBurnCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newClaudeCmd())
	root.AddCommand(newCodexCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newNukeCmd())
	root.AddCommand(newRebuildCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newLogsCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newUpdateCmd())
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/claustro/...`
Expected: all PASS, including the new codex tests.

- [ ] **Step 6: Commit**

```bash
git add cmd/claustro/codex.go cmd/claustro/commands.go cmd/claustro/commands_test.go
git commit -m "feat: add claustro codex command"
```

---

### Task 6: Add `claustro codex` hint to `up` success output

**Files:**
- Modify: `cmd/claustro/up.go`

The success message currently lists `claustro shell` and `claustro claude`. Add a third line for `claustro codex`.

- [ ] **Step 1: Update the two output branches in `runUp`**

In `cmd/claustro/up.go`, replace this block:

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
	return nil
```

with:

```go
	fmt.Printf("Sandbox started: %s\n", id.ContainerName())
	if nameWasEmpty {
		fmt.Printf("  Name: %s  (use --name %s to target it)\n", id.Name, id.Name)
		fmt.Printf("  Run: claustro shell  --name %s\n", id.Name)
		fmt.Printf("  Run: claustro claude --name %s\n", id.Name)
		fmt.Printf("  Run: claustro codex  --name %s\n", id.Name)
	} else {
		fmt.Printf("  Run: claustro shell  —  open a shell\n")
		fmt.Printf("  Run: claustro claude —  start Claude Code\n")
		fmt.Printf("  Run: claustro codex  —  start Codex CLI\n")
	}
	return nil
```

(The trivial whitespace adjustment in the `--name` lines keeps the column alignment identical for `shell`, `claude`, and `codex`.)

- [ ] **Step 2: Verify build and tests pass**

Run: `go build ./... && go test ./cmd/claustro/...`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/claustro/up.go
git commit -m "feat: surface claustro codex hint in up success output"
```

---

### Task 7: Final verification

**Files:** None (verification only).

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 2: Full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 3: Lint**

Run: `golangci-lint run`
Expected: no new warnings.

- [ ] **Step 4: Manual smoke (recommended, not codified)**

In two terminals from the same project root:

```
# terminal 1
claustro codex
# terminal 2
claustro claude
# terminal 3
claustro ls
# expect: exactly one container listed
```

- [ ] **Step 5: Lint-fix commit if needed**

If lint produced fixes:

```bash
git add -A
git commit -m "chore: lint fixes for codex command"
```

Otherwise no commit required.

---

## Out of Scope (per spec)

- Host-side bin shims (`codex`/`claude` on `$PATH`).
- Generic `claustro agent <name>` dispatcher.
- Typed enum for agent names.
- Image-level changes (already in #34).
- Probing the image for a missing codex binary when config says it should be present.
