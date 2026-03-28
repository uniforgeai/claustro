# M2 Remaining Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--readonly` and `--isolated-state` flags to `claustro up`, completing all M2 features.

**Architecture:** Both flags flow through CLIOverrides -> SandboxConfig -> mount.Assemble(). The `--readonly` flag makes the `/workspace` source mount read-only. The `--isolated-state` flag replaces the `~/.claude` bind mount with a project-level Docker volume. A new `ProjectVolumeName()` helper in the identity package produces volume names without a sandbox-name component so the volume is shared across all sandboxes in a project.

**Tech Stack:** Go, Cobra, Docker SDK for Go, Testify

---

### Task 1: Add `ProjectVolumeName` to identity package

**Files:**
- Modify: `internal/identity/identity.go:76-80`
- Modify: `internal/identity/identity_test.go:118-137`

- [ ] **Step 1: Write the failing test**

Add to `internal/identity/identity_test.go` after the existing `TestIdentity_VolumeName` function:

```go
func TestProjectVolumeName(t *testing.T) {
	tests := []struct {
		project string
		purpose string
		want    string
	}{
		{"myapp", "claude-state", "claustro-myapp-claude-state"},
		{"my-saas", "claude-state", "claustro-my-saas-claude-state"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, ProjectVolumeName(tt.project, tt.purpose))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/ -run TestProjectVolumeName -v`
Expected: FAIL — `ProjectVolumeName` undefined

- [ ] **Step 3: Write minimal implementation**

Add to `internal/identity/identity.go` after the `VolumeName` method:

```go
// ProjectVolumeName returns a Docker volume name scoped to the project (not a specific sandbox).
// Format: claustro-{project}-{purpose}
func ProjectVolumeName(project, purpose string) string {
	return fmt.Sprintf("claustro-%s-%s", project, purpose)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/identity/ -run TestProjectVolumeName -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/identity/identity.go internal/identity/identity_test.go
git commit -m "feat: add ProjectVolumeName for project-scoped volumes"
```

---

### Task 2: Add `ReadOnly` and `IsolatedState` to CLIOverrides and SandboxConfig

**Files:**
- Modify: `internal/config/resolve.go:20-25`
- Modify: `internal/config/resolve_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/resolve_test.go`:

```go
func TestResolve_ReadOnlyCLIOverride(t *testing.T) {
	readOnly := true
	cfg := &Config{
		Defaults: DefaultsConfig{ReadOnly: boolPtr(false)},
	}
	cli := CLIOverrides{
		Name:     "test",
		ReadOnly: &readOnly,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.ReadOnly, "CLI --readonly should override config default")
}

func TestResolve_ReadOnlyCLINil_UsesDefault(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{ReadOnly: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.ReadOnly, "config default readonly should apply when CLI flag is nil")
}

func TestResolve_IsolatedStateCLIOverride(t *testing.T) {
	cfg := &Config{}
	cli := CLIOverrides{
		Name:          "test",
		IsolatedState: true,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.IsolatedState, "CLI --isolated-state should be reflected in SandboxConfig")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestResolve_ReadOnly|TestResolve_IsolatedState" -v`
Expected: FAIL — fields do not exist

- [ ] **Step 3: Write minimal implementation**

In `internal/config/resolve.go`, update `CLIOverrides`:

```go
// CLIOverrides holds values provided via CLI flags that override config file settings.
type CLIOverrides struct {
	Name          string
	Workdir       string
	Mounts        []string
	Env           map[string]string
	ReadOnly      *bool
	IsolatedState bool
}
```

Update `SandboxConfig`:

```go
// SandboxConfig is the fully resolved, flat configuration for a single sandbox.
// All merging (defaults, named sandbox, dotenv, CLI overrides) is already applied.
type SandboxConfig struct {
	Name          string
	Workdir       string
	Mounts        []Mount
	Env           map[string]string
	Firewall      bool
	ReadOnly      bool
	IsolatedState bool
	CPUs          string
	Memory        string
	ImageName     string
}
```

In the `Resolve` method, add after the existing `cli.Env` loop (before the `return`):

```go
	// CLI readonly override.
	if cli.ReadOnly != nil {
		sc.ReadOnly = *cli.ReadOnly
	}
	sc.IsolatedState = cli.IsolatedState
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/resolve.go internal/config/resolve_test.go
git commit -m "feat: add ReadOnly and IsolatedState to CLIOverrides and SandboxConfig"
```

---

### Task 3: Add `readOnly` and `isolatedState` params to `mount.Assemble`

**Files:**
- Modify: `internal/mount/mount.go:51`
- Modify: `internal/mount/mount_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/mount/mount_test.go`:

```go
func TestAssemble_readOnlySource(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "", true, false)
	require.NoError(t, err)

	for _, m := range mounts {
		if m.Target == "/workspace" {
			assert.True(t, m.ReadOnly, "/workspace should be read-only when readOnly=true")
			return
		}
	}
	t.Fatal("no /workspace mount found")
}

func TestAssemble_readOnlyFalse_sourceIsRW(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "", false, false)
	require.NoError(t, err)

	for _, m := range mounts {
		if m.Target == "/workspace" {
			assert.False(t, m.ReadOnly, "/workspace should be read-write when readOnly=false")
			return
		}
	}
	t.Fatal("no /workspace mount found")
}

func TestAssemble_isolatedState_skipsClaudeMounts(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "", false, true)
	require.NoError(t, err)

	for _, m := range mounts {
		switch m.Target {
		case "/home/sandbox/.claude":
			t.Error("~/.claude bind mount should be skipped when isolatedState=true")
		case "/home/sandbox/.claude.json":
			t.Error("~/.claude.json bind mount should be skipped when isolatedState=true")
		}
	}
}

func TestAssemble_isolatedState_skipsPluginRemount(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	pluginDir := filepath.Join(home, ".claude", "plugins")
	if !fileExists(pluginDir) {
		t.Skip("~/.claude/plugins does not exist")
	}
	if home == "/home/sandbox" {
		t.Skip("home is /home/sandbox, plugin remount never applies")
	}

	mounts, err := Assemble("/some/project", nil, "", false, true)
	require.NoError(t, err)

	for _, m := range mounts {
		if m.Target == pluginDir && m.Source == pluginDir {
			t.Error("plugin dir remount should be skipped when isolatedState=true")
		}
	}
}

func TestAssemble_notIsolated_includesClaudeMount(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	mounts, err := Assemble("/some/project", nil, "", false, false)
	require.NoError(t, err)

	assertMount(t, mounts, filepath.Join(home, ".claude"), "/home/sandbox/.claude", dockermount.TypeBind)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mount/ -run "TestAssemble_readOnly|TestAssemble_isolatedState|TestAssemble_notIsolated" -v`
Expected: FAIL — too many arguments in call to Assemble

- [ ] **Step 3: Update Assemble signature and implementation**

In `internal/mount/mount.go`, change the `Assemble` function signature and body:

```go
// Assemble returns the bind mounts needed for a sandbox.
//
// When readOnly is true, the source mount at /workspace is marked read-only.
// When isolatedState is true, the ~/.claude bind mount, ~/.claude.json bind mount,
// and plugin directory remount are all skipped (caller provides a volume mount instead).
func Assemble(hostProjectPath string, git *config.GitConfig, clipboardSockDir string, readOnly, isolatedState bool) ([]mount.Mount, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   hostProjectPath,
			Target:   "/workspace",
			ReadOnly: readOnly,
		},
	}

	if !isolatedState {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: filepath.Join(home, ".claude"),
			Target: "/home/sandbox/.claude",
		})

		claudeJSON := filepath.Join(home, ".claude.json")
		if _, err := os.Stat(claudeJSON); err == nil {
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: claudeJSON,
				Target: "/home/sandbox/.claude.json",
			})
		}
	}
```

The rest of the function body stays the same EXCEPT the plugin dir remount block (near the end) also needs the `isolatedState` guard. Change:

```go
	// Plugin path remapping...
	pluginDir := filepath.Join(home, ".claude", "plugins")
	containerPluginDir := "/home/sandbox/.claude/plugins"
	if pluginDir != containerPluginDir {
```

to:

```go
	// Plugin path remapping (skip when isolated state — no host .claude in play)
	if !isolatedState {
		pluginDir := filepath.Join(home, ".claude", "plugins")
		containerPluginDir := "/home/sandbox/.claude/plugins"
		if pluginDir != containerPluginDir {
			if _, err := os.Stat(pluginDir); err == nil {
				mounts = append(mounts, mount.Mount{
					Type:     mount.TypeBind,
					Source:   pluginDir,
					Target:   pluginDir,
					ReadOnly: true,
				})
			}
		}
	}
```

- [ ] **Step 4: Fix existing tests to use new signature**

Update ALL existing `Assemble` calls in `internal/mount/mount_test.go` to pass the two new `false, false` arguments. Every existing call like:

```go
Assemble("/some/project", nil, "")
```

becomes:

```go
Assemble("/some/project", nil, "", false, false)
```

And calls like:

```go
Assemble("/some/project", git, "")
```

become:

```go
Assemble("/some/project", git, "", false, false)
```

And the clipboard test:

```go
Assemble("/some/project", nil, sockDir)
```

becomes:

```go
Assemble("/some/project", nil, sockDir, false, false)
```

- [ ] **Step 5: Run all mount tests**

Run: `go test ./internal/mount/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/mount/mount.go internal/mount/mount_test.go
git commit -m "feat: add readOnly and isolatedState params to mount.Assemble"
```

---

### Task 4: Wire flags into `cmd/claustro/up.go`

**Files:**
- Modify: `cmd/claustro/up.go:21-47` (newUpCmd)
- Modify: `cmd/claustro/up.go:62-95` (runUp)
- Modify: `cmd/claustro/up.go:102-215` (ensureRunning)

- [ ] **Step 1: Add CLI flags to `newUpCmd`**

In `cmd/claustro/up.go`, add two new flag variables and register them:

```go
func newUpCmd() *cobra.Command {
	var (
		name          string
		workdir       string
		mounts        []string
		envs          []string
		readOnly      bool
		isolatedState bool
	)
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start a sandbox",
		Long:  "Build the claustro image if needed, then create and start a sandbox container.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliEnv := parseEnvFlags(envs)
			var readOnlyPtr *bool
			if cmd.Flags().Changed("readonly") {
				readOnlyPtr = &readOnly
			}
			return runUp(cmd.Context(), name, config.CLIOverrides{
				Name:          name,
				Workdir:       workdir,
				Mounts:        mounts,
				Env:           cliEnv,
				ReadOnly:      readOnlyPtr,
				IsolatedState: isolatedState,
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-generated)`)
	cmd.Flags().StringVar(&workdir, "workdir", "", `Working directory inside the container`)
	cmd.Flags().StringSliceVar(&mounts, "mount", nil, `Additional bind mount (host:container[:ro|rw])`)
	cmd.Flags().StringSliceVar(&envs, "env", nil, `Environment variable (KEY=VALUE)`)
	cmd.Flags().BoolVar(&readOnly, "readonly", false, `Mount source directory as read-only`)
	cmd.Flags().BoolVar(&isolatedState, "isolated-state", false, `Use a Docker volume for Claude state instead of bind-mounting ~/.claude`)
	return cmd
}
```

- [ ] **Step 2: Update the `Assemble` call in `ensureRunning`**

In `ensureRunning`, change the `Assemble` call from:

```go
	mounts, err := internalMount.Assemble(id.HostPath, &cfg.Git, socketDir)
```

to:

```go
	mounts, err := internalMount.Assemble(id.HostPath, &cfg.Git, socketDir, resolved.ReadOnly, resolved.IsolatedState)
```

- [ ] **Step 3: Add isolated-state volume creation**

In `ensureRunning`, after the `Assemble` call and before the npm/pip cache volume loop, add:

```go
	// When isolated state is requested, create a project-scoped volume for Claude state.
	if resolved.IsolatedState {
		volName := identity.ProjectVolumeName(id.Project, "claude-state")
		if err := container.EnsureVolume(ctx, cli, volName, id.Labels()); err != nil {
			return nil, false, fmt.Errorf("ensuring claude state volume %q: %w", volName, err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volName,
			Target: "/home/sandbox/.claude",
		})
	}
```

Add the `identity` import to the `mount.Mount` usage (it's already imported via `"github.com/docker/docker/api/types/mount"`).

- [ ] **Step 4: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/claustro/up.go
git commit -m "feat: wire --readonly and --isolated-state flags into claustro up"
```

---

### Task 5: Fix stale integration test `Assemble` calls

**Files:**
- Modify: `internal/container/container_integration_test.go:37`
- Modify: `internal/container/nuke_integration_test.go:27`

These integration tests call `mount.Assemble(cwd)` with only 1 argument — a stale signature from before git/clipboard params were added. Update them to the current 5-arg signature.

- [ ] **Step 1: Fix container_integration_test.go**

Change line 37 from:

```go
	mounts, err := mount.Assemble(cwd)
```

to:

```go
	mounts, err := mount.Assemble(cwd, nil, "", false, false)
```

- [ ] **Step 2: Fix nuke_integration_test.go**

Change line 27 from:

```go
	mounts, err := mount.Assemble(cwd)
```

to:

```go
	mounts, err := mount.Assemble(cwd, nil, "", false, false)
```

- [ ] **Step 3: Build to verify**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/container/container_integration_test.go internal/container/nuke_integration_test.go
git commit -m "fix: update stale Assemble calls in integration tests"
```

---

### Task 6: Verify and lint

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 2: Run linter**

Run: `golangci-lint run`
Expected: No new warnings

- [ ] **Step 3: Manual smoke check of flag help**

Run: `go run ./cmd/claustro up --help`
Expected: Output includes `--readonly` and `--isolated-state` flags with descriptions

- [ ] **Step 4: Final commit if any lint fixes needed**

Only if linter found issues. Otherwise this step is a no-op.
