# Codex CLI Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Install OpenAI Codex CLI in the sandbox image (opt-out via config) and forward host `~/.codex/` + `OPENAI_API_KEY` into containers.

**Architecture:** Follow the existing opt-out pattern used by `LanguagesConfig`, `ToolsConfig`, and `MCPServersConfig`. Add an `AgentsConfig` struct with a `Codex *bool` field (nil = enabled). Wire it through the Dockerfile template, mount assembly, and container env vars.

**Tech Stack:** Go 1.23+, Docker SDK for Go, Testify, text/template

**Spec:** `docs/specs/codex-integration.md`

---

### Task 1: Add `AgentsConfig` struct and `IsAgentEnabled()` method

**Files:**
- Modify: `internal/config/image_config.go`
- Modify: `internal/config/image_config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/image_config_test.go`:

```go
func TestDefaultImageBuildConfig_AgentsEnabled(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	assert.True(t, cfg.IsAgentEnabled("codex"), "codex should be enabled by default")
}

func TestImageBuildConfig_DisableAgents(t *testing.T) {
	f := false
	cfg := ImageBuildConfig{
		Agents: AgentsConfig{
			Codex: &f,
		},
	}
	assert.False(t, cfg.IsAgentEnabled("codex"))
}

func TestImageBuildConfig_UnknownAgent(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	assert.False(t, cfg.IsAgentEnabled("unknown"), "unknown agent should return false")
	assert.False(t, cfg.IsAgentEnabled(""), "empty agent should return false")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestDefaultImageBuildConfig_Agents -v`
Expected: compilation error — `AgentsConfig` and `IsAgentEnabled` undefined.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/config/image_config.go`, after the `MCPServersConfig` struct:

```go
// AgentsConfig controls which additional coding agents are installed in the sandbox image.
// A nil pointer means the agent is enabled (opt-out model).
type AgentsConfig struct {
	Codex *bool `yaml:"codex"`
}
```

Add the `Agents` field to `ImageBuildConfig`:

```go
type ImageBuildConfig struct {
	Languages  LanguagesConfig  `yaml:"languages"`
	Tools      ToolsConfig      `yaml:"tools"`
	MCPServers MCPServersConfig `yaml:"mcp_servers"`
	Agents     AgentsConfig     `yaml:"agents"`
}
```

Add the method after `IsMCPServerEnabled`:

```go
// IsAgentEnabled reports whether the given coding agent should be installed.
// nil means true (enabled), false means disabled. Unknown agents return false.
func (c *ImageBuildConfig) IsAgentEnabled(agent string) bool {
	switch agent {
	case "codex":
		return c.Agents.Codex == nil || *c.Agents.Codex
	default:
		return false
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: all tests PASS, including the three new ones.

- [ ] **Step 5: Commit**

```bash
git add internal/config/image_config.go internal/config/image_config_test.go
git commit -m "feat: add AgentsConfig and IsAgentEnabled to image build config"
```

---

### Task 2: Add YAML parsing test for agents config

**Files:**
- Modify: `internal/config/image_config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/image_config_test.go`:

```go
func TestLoad_AgentsConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  agents:
    codex: false
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.False(t, cfg.ImageBuild.IsAgentEnabled("codex"))
}

func TestLoad_MissingAgentsBlock_AllEnabled(t *testing.T) {
	dir := t.TempDir()
	content := `project: my-project`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.True(t, cfg.ImageBuild.IsAgentEnabled("codex"), "codex should be enabled when agents block is missing")
}
```

- [ ] **Step 2: Run tests to verify they pass**

These should already pass because Task 1 wired `AgentsConfig` into `ImageBuildConfig` and `postProcess()` already decodes `ImageBuild` from the `RawImage` mapping node. Run:

Run: `go test ./internal/config/ -run TestLoad_Agents -v`
Expected: PASS. If they fail, debug and fix the YAML decoding path.

- [ ] **Step 3: Commit**

```bash
git add internal/config/image_config_test.go
git commit -m "test: add YAML parsing tests for agents config"
```

---

### Task 3: Add Codex install block to Dockerfile template

**Files:**
- Modify: `internal/image/template.go`
- Modify: `internal/image/Dockerfile.tmpl`
- Modify: `internal/image/template_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/image/template_test.go`:

```go
func TestRenderDockerfile_CodexEnabled(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.Contains(t, out, "@openai/codex", "codex should be installed by default")
}

func TestRenderDockerfile_CodexDisabled(t *testing.T) {
	f := false
	cfg := config.DefaultImageBuildConfig()
	cfg.Agents.Codex = &f

	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.NotContains(t, out, "@openai/codex", "codex should not be installed when disabled")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/image/ -run TestRenderDockerfile_Codex -v`
Expected: FAIL — `@openai/codex` not found in output.

- [ ] **Step 3: Add `Codex` field to `templateData` and populate it**

In `internal/image/template.go`, add the `Codex` field to `templateData`:

```go
type templateData struct {
	Go, Rust, Python                    bool
	DevTools, BuildTools                bool
	MCPFilesystem, MCPMemory, MCPFetch  bool
	VoiceMode                           bool
	Codex                               bool
}
```

In `RenderDockerfile`, add the field after `VoiceMode`:

```go
	data := templateData{
		// ... existing fields ...
		VoiceMode: cfg.IsToolGroupEnabled("voice"),
		Codex:     cfg.IsAgentEnabled("codex"),
	}
```

Add `"codex", data.Codex` to the `slog.Info` call.

- [ ] **Step 4: Add Codex install block to Dockerfile.tmpl**

Insert after the `ccstatusline` install block (line 71) and before the `gopls` block:

```dockerfile
{{if .Codex}}
# Install Codex CLI
RUN npm install -g @openai/codex
{{end}}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/image/ -v`
Expected: all tests PASS.

- [ ] **Step 6: Update the `TestRenderDockerfile_AllEnabled` test**

Add to the existing assertions in `TestRenderDockerfile_AllEnabled`:

```go
	// Codex
	assert.Contains(t, out, "@openai/codex")
```

Also add to the `TestRenderDockerfile_MinimalConfig` test's "conditional items absent" section:

```go
	assert.NotContains(t, out, "@openai/codex")
```

Update the minimal config to also disable agents:

```go
	cfg := config.ImageBuildConfig{
		// ... existing fields ...
		Agents: config.AgentsConfig{
			Codex: &f,
		},
	}
```

- [ ] **Step 7: Run all tests again**

Run: `go test ./internal/image/ -v`
Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/image/template.go internal/image/Dockerfile.tmpl internal/image/template_test.go
git commit -m "feat: add Codex CLI install block to Dockerfile template"
```

---

### Task 4: Add `addCodexMounts()` to mount assembly

**Files:**
- Modify: `internal/mount/mount.go`
- Modify: `internal/mount/mount_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/mount/mount_test.go`:

```go
func TestAssemble_codexMountedWhenPresent(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	codexDir := filepath.Join(home, ".codex")
	if !fileExists(codexDir) {
		t.Skip("~/.codex does not exist on this machine")
	}

	mounts, err := Assemble("/some/project", nil, "", false, false)
	require.NoError(t, err)

	found := false
	for _, m := range mounts {
		if m.Target == "/home/sandbox/.codex" {
			found = true
			assert.Equal(t, codexDir, m.Source)
			assert.False(t, m.ReadOnly, "codex dir should be read-write")
		}
	}
	assert.True(t, found, "~/.codex mount should be present when directory exists")
}

func TestAssemble_codexSkippedWhenIsolated(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "", false, true)
	require.NoError(t, err)

	for _, m := range mounts {
		assert.NotEqual(t, "/home/sandbox/.codex", m.Target,
			"~/.codex mount should be skipped when isolatedState=true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mount/ -run TestAssemble_codex -v`
Expected: FAIL — no mount with target `/home/sandbox/.codex`.

- [ ] **Step 3: Write minimal implementation**

Add the constant in `internal/mount/mount.go` alongside the existing container path constants:

```go
const (
	// ... existing constants ...
	containerCodexDir = containerHome + "/.codex"
)
```

Add the function after `addClaudeMounts`:

```go
// addCodexMounts appends the ~/.codex directory mount for Codex CLI state.
// Skipped when isolatedState is true or when ~/.codex does not exist on the host.
func addCodexMounts(mounts *[]mount.Mount, home string, isolatedState bool) {
	if isolatedState {
		return
	}

	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); err != nil {
		return
	}

	*mounts = append(*mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: codexDir,
		Target: containerCodexDir,
	})
}
```

Call it from `Assemble`, right after `addClaudeMounts`:

```go
	addClaudeMounts(&mounts, home, isolatedState)
	addCodexMounts(&mounts, home, isolatedState)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mount/ -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mount/mount.go internal/mount/mount_test.go
git commit -m "feat: add Codex config mount to sandbox container"
```

---

### Task 5: Add `OPENAI_API_KEY` env var passthrough

**Files:**
- Modify: `internal/container/container.go`
- Modify: `internal/container/container_test.go` (or create if absent)

- [ ] **Step 1: Check if container_test.go exists**

Run: `ls internal/container/container_test.go`

If it does not exist, create it with the copyright header.

- [ ] **Step 2: Write the failing test**

The `Create` function calls the Docker SDK and is hard to unit-test without mocking. Instead, extract the env assembly into a testable helper. Add a new function and test:

Add to `internal/container/container.go` (extract from `Create`):

```go
// sandboxEnv assembles environment variables for a new sandbox container.
func sandboxEnv(hostPath string) []string {
	env := []string{
		"CLAUSTRO_HOST_PATH=" + hostPath,
		"HOME=" + containerHome,
	}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		env = append(env, "SSH_AUTH_SOCK="+claustromount.SSHAgentContainerSock(sock))
	}
	// Forward API keys from host environment.
	for _, key := range []string{"OPENAI_API_KEY"} {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}
	return env
}
```

Create `internal/container/env_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxEnv_AlwaysIncludesBase(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	env := sandboxEnv("/some/project")
	assert.Contains(t, env, "CLAUSTRO_HOST_PATH=/some/project")
	assert.Contains(t, env, "HOME=/home/sandbox")
}

func TestSandboxEnv_OpenAIKeyPassedThrough(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")

	env := sandboxEnv("/some/project")
	assert.Contains(t, env, "OPENAI_API_KEY=sk-test-123")
}

func TestSandboxEnv_OpenAIKeyOmittedWhenEmpty(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	env := sandboxEnv("/some/project")
	for _, e := range env {
		assert.NotContains(t, e, "OPENAI_API_KEY",
			"OPENAI_API_KEY should not appear when not set")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/container/ -run TestSandboxEnv -v`
Expected: compilation error — `sandboxEnv` undefined.

- [ ] **Step 4: Write the implementation**

In `internal/container/container.go`, add the `sandboxEnv` function (shown in Step 2) and replace the inline env assembly in `Create`:

Replace lines 80-86 in `Create`:

```go
	env := sandboxEnv(id.HostPath)
```

Remove the old inline `env` construction (the `env := []string{...}` block and the SSH_AUTH_SOCK append).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/container/ -v`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/container/container.go internal/container/env_test.go
git commit -m "feat: forward OPENAI_API_KEY into sandbox containers"
```

---

### Task 6: Verify full build and test suite

**Files:** None (verification only)

- [ ] **Step 1: Run full build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 2: Run full test suite**

Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 3: Run linter**

Run: `golangci-lint run`
Expected: no new warnings.

- [ ] **Step 4: Final commit (if any lint fixes needed)**

```bash
git add -A
git commit -m "chore: lint fixes for codex integration"
```
