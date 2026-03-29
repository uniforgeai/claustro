# M4: Polish & Distribution — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the M4 milestone — configurable image system, init wizard, config command, config validation, release pipeline (GoReleaser + GitHub Actions + Homebrew), and Hugo documentation site.

**Architecture:** Three independent workstreams: (A) Image Config + CLI commands — templated Dockerfile, `claustro init` wizard, `claustro config` command with interactive subcommands + get/set; (B) Release Pipeline — GoReleaser cross-compilation, GitHub Actions CI/release workflows, Homebrew tap; (C) Validation + Docs — config validation at load time, `claustro validate` command, doctor integration, Hugo docs site. Workstreams A, B, C can be parallelized.

**Tech Stack:** Go 1.26, Cobra, Docker SDK, `charmbracelet/huh` (TUI forms), GoReleaser, Hugo (hugo-book theme), GitHub Actions

---

## File Map

### New Files

| File | Responsibility |
|------|----------------|
| `internal/config/validate.go` | Config validation logic — `Validate()` returning `[]ValidationResult` |
| `internal/config/validate_test.go` | Tests for config validation |
| `internal/config/image_config.go` | `ImageBuildConfig` struct (languages/tools/mcp toggles) + defaults |
| `internal/config/image_config_test.go` | Tests for image config parsing and defaults |
| `internal/image/template.go` | Dockerfile Go template + `RenderDockerfile(ImageBuildConfig)` |
| `internal/image/template_test.go` | Tests for template rendering |
| `internal/wizard/wizard.go` | Shared `huh` form builders for init + config interactive prompts |
| `internal/wizard/wizard_test.go` | Tests for wizard form output (config generation) |
| `cmd/claustro/init_cmd.go` | `claustro init` command |
| `cmd/claustro/config_cmd.go` | `claustro config` command + subcommands + get/set |
| `cmd/claustro/validate.go` | `claustro validate` command |
| `.goreleaser.yaml` | GoReleaser configuration |
| `.github/workflows/ci.yml` | CI workflow (build, test, lint) |
| `.github/workflows/release.yml` | Release workflow (GoReleaser on tag push) |
| `.github/workflows/docs.yml` | Hugo docs site deployment to GitHub Pages |
| `docs/site/hugo.toml` | Hugo site configuration |
| `docs/site/content/_index.md` | Landing page |
| `docs/site/content/getting-started/*.md` | Installation, quickstart, configuration docs |
| `docs/site/content/commands/*.md` | Per-command reference docs |
| `docs/site/content/guides/*.md` | Monorepo, firewall, MCP, custom image guides |
| `docs/site/content/reference/*.md` | claustro.yaml reference, env vars |

### Modified Files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `ImageBuildConfig` field to `Config`, update `postProcess()` |
| `internal/config/config_test.go` | Add tests for `ImageBuildConfig` parsing |
| `internal/image/image.go` | Replace static `dockerfile` embed with template rendering, update `buildContext()` |
| `internal/image/image_test.go` | Update tests for templated build context |
| `internal/doctor/doctor.go` | Extend `CheckConfigFile()` to run full validation |
| `internal/doctor/doctor_test.go` | Add tests for validation-aware doctor check |
| `cmd/claustro/commands.go` | Register `init`, `config`, `validate` commands |
| `cmd/claustro/commands_test.go` | Update `TestSetupCommands_RegistersAllCommands` |
| `go.mod` | Add `charmbracelet/huh` dependency |

---

## Task 1: Config Validation

**Files:**
- Create: `internal/config/validate.go`
- Create: `internal/config/validate_test.go`
- Modify: `internal/config/config.go` (call Validate from Load)

### Step 1.1: Write validation types and test scaffolding

- [ ] **Write the failing tests for Validate()**

Create `internal/config/validate_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := Config{}
	results := cfg.Validate()
	assert.Empty(t, results, "empty config should be valid")
}

func TestValidate_InvalidCPUs(t *testing.T) {
	cfg := Config{
		Defaults: DefaultsConfig{
			Resources: ResourcesConfig{CPUs: "abc"},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Equal(t, SeverityError, results[0].Severity)
	assert.Contains(t, results[0].Field, "defaults.resources.cpus")
}

func TestValidate_ZeroCPUs(t *testing.T) {
	cfg := Config{
		Defaults: DefaultsConfig{
			Resources: ResourcesConfig{CPUs: "0"},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Equal(t, SeverityWarning, results[0].Severity)
}

func TestValidate_InvalidMemory(t *testing.T) {
	cfg := Config{
		Defaults: DefaultsConfig{
			Resources: ResourcesConfig{Memory: "lots"},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Equal(t, SeverityError, results[0].Severity)
	assert.Contains(t, results[0].Field, "defaults.resources.memory")
}

func TestValidate_ValidMemory(t *testing.T) {
	tests := []string{"8G", "512M", "1024K", "16g", "256m"}
	for _, mem := range tests {
		t.Run(mem, func(t *testing.T) {
			cfg := Config{
				Defaults: DefaultsConfig{
					Resources: ResourcesConfig{Memory: mem},
				},
			}
			results := cfg.Validate()
			assert.Empty(t, results)
		})
	}
}

func TestValidate_InvalidMountFormat(t *testing.T) {
	cfg := Config{
		Sandboxes: map[string]SandboxDef{
			"api": {Mounts: []string{"libs"}},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Equal(t, SeverityError, results[0].Severity)
	assert.Contains(t, results[0].Field, "sandboxes.api.mounts[0]")
}

func TestValidate_InvalidMountMode(t *testing.T) {
	cfg := Config{
		Sandboxes: map[string]SandboxDef{
			"api": {Mounts: []string{"./src:/workspace/src:rx"}},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Contains(t, results[0].Message, "mode")
}

func TestValidate_ValidMounts(t *testing.T) {
	cfg := Config{
		Sandboxes: map[string]SandboxDef{
			"api": {Mounts: []string{
				"./src:/workspace/src",
				"./libs:/workspace/libs:ro",
				"./data:/workspace/data:rw",
			}},
		},
	}
	results := cfg.Validate()
	assert.Empty(t, results)
}

func TestValidate_InvalidSandboxName(t *testing.T) {
	cfg := Config{
		Sandboxes: map[string]SandboxDef{
			"my sandbox!": {},
		},
	}
	results := cfg.Validate()
	require.NotEmpty(t, results)
	assert.Equal(t, SeverityError, results[0].Severity)
	assert.Contains(t, results[0].Field, "sandboxes.my sandbox!")
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := Config{
		Defaults: DefaultsConfig{
			Resources: ResourcesConfig{CPUs: "abc", Memory: "lots"},
		},
		Sandboxes: map[string]SandboxDef{
			"bad name!": {Mounts: []string{"invalid"}},
		},
	}
	results := cfg.Validate()
	assert.GreaterOrEqual(t, len(results), 3, "should have at least 3 validation errors")
}

func TestValidate_ErrorsOnly(t *testing.T) {
	cfg := Config{
		Defaults: DefaultsConfig{
			Resources: ResourcesConfig{CPUs: "abc"},
		},
	}
	results := cfg.Validate()
	errors := cfg.Errors()
	warnings := cfg.Warnings()
	assert.Equal(t, len(results), len(errors)+len(warnings))
	assert.NotEmpty(t, errors)
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/config/ -run TestValidate -v`
Expected: compilation error — `Validate`, `Errors`, `Warnings`, `SeverityError`, `SeverityWarning` not defined.

### Step 1.2: Implement validation

- [ ] **Create `internal/config/validate.go`**

```go
package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Severity represents how critical a validation issue is.
type Severity int

const (
	SeverityError   Severity = iota // Blocks execution
	SeverityWarning                 // Logged but does not block
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warn"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// ValidationResult describes a single validation issue.
type ValidationResult struct {
	Field    string
	Message  string
	Severity Severity
}

var (
	memoryPattern    = regexp.MustCompile(`(?i)^\d+[GMK]$`)
	sandboxNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
)

// Validate checks the config for errors and warnings.
// Returns an empty slice if the config is valid.
func (c *Config) Validate() []ValidationResult {
	var results []ValidationResult

	results = append(results, c.validateResources()...)
	results = append(results, c.validateSandboxes()...)
	results = append(results, c.validateImageBuild()...)

	return results
}

// Errors returns only error-severity results.
func (c *Config) Errors() []ValidationResult {
	return filterBySeverity(c.Validate(), SeverityError)
}

// Warnings returns only warning-severity results.
func (c *Config) Warnings() []ValidationResult {
	return filterBySeverity(c.Validate(), SeverityWarning)
}

func filterBySeverity(results []ValidationResult, sev Severity) []ValidationResult {
	var out []ValidationResult
	for _, r := range results {
		if r.Severity == sev {
			out = append(out, r)
		}
	}
	return out
}

func (c *Config) validateResources() []ValidationResult {
	var results []ValidationResult
	res := c.Defaults.Resources

	if res.CPUs != "" {
		v, err := strconv.ParseFloat(res.CPUs, 64)
		if err != nil {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.cpus",
				Message:  fmt.Sprintf("invalid CPU value %q: must be a number", res.CPUs),
				Severity: SeverityError,
			})
		} else if v == 0 {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.cpus",
				Message:  fmt.Sprintf("%q is unusually low", res.CPUs),
				Severity: SeverityWarning,
			})
		}
	}

	if res.Memory != "" {
		if !memoryPattern.MatchString(res.Memory) {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.memory",
				Message:  fmt.Sprintf("invalid memory value %q: expected pattern like 8G, 512M, 1024K", res.Memory),
				Severity: SeverityError,
			})
		}
	}

	return results
}

func (c *Config) validateSandboxes() []ValidationResult {
	var results []ValidationResult

	for name, sb := range c.Sandboxes {
		if !sandboxNameRegex.MatchString(name) {
			results = append(results, ValidationResult{
				Field:    fmt.Sprintf("sandboxes.%s", name),
				Message:  fmt.Sprintf("sandbox name %q is not a valid identifier (use alphanumeric, hyphens, underscores)", name),
				Severity: SeverityError,
			})
		}

		for i, m := range sb.Mounts {
			if err := validateMountFormat(m); err != nil {
				results = append(results, ValidationResult{
					Field:    fmt.Sprintf("sandboxes.%s.mounts[%d]", name, i),
					Message:  err.Error(),
					Severity: SeverityError,
				})
			}
		}
	}

	return results
}

func validateMountFormat(mount string) error {
	parts := strings.Split(mount, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("invalid mount format %q (expected host:container[:mode])", mount)
	}
	if len(parts) == 3 {
		mode := parts[2]
		if mode != "ro" && mode != "rw" {
			return fmt.Errorf("invalid mount mode %q in %q (expected ro or rw)", mode, mount)
		}
	}
	return nil
}

func (c *Config) validateImageBuild() []ValidationResult {
	var results []ValidationResult

	if c.ImageBuild.Languages.Node != nil && !*c.ImageBuild.Languages.Node {
		results = append(results, ValidationResult{
			Field:    "image.languages.node",
			Message:  "Node.js cannot be disabled (required for Claude Code)",
			Severity: SeverityError,
		})
	}

	return results
}
```

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/config/ -run TestValidate -v`
Expected: all tests PASS.

- [ ] **Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go
git commit -m "feat: add config validation with errors and warnings"
```

### Step 1.3: Integrate validation into config.Load()

- [ ] **Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestLoad_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
defaults:
  resources:
    cpus: "not-a-number"
`), 0o644)
	require.NoError(t, err)

	_, err = Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cpus")
}

func TestLoad_ValidationWarnings_DoNotBlock(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
defaults:
  resources:
    cpus: "0"
`), 0o644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err, "warnings should not block Load")
	assert.Equal(t, "0", cfg.Defaults.Resources.CPUs)
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/config/ -run TestLoad_Validation -v`
Expected: `TestLoad_ValidationErrors` FAILS (Load does not run validation yet).

- [ ] **Modify `config.Load()` to run validation**

In `internal/config/config.go`, after the `postProcess()` call in `Load()`, add:

```go
	if err := cfg.postProcess(); err != nil {
		return Config{}, fmt.Errorf("processing config: %w", err)
	}

	// Validate config and reject if any errors found.
	results := cfg.Validate()
	for _, r := range results {
		if r.Severity == SeverityError {
			var msgs []string
			for _, r := range results {
				if r.Severity == SeverityError {
					msgs = append(msgs, fmt.Sprintf("%s: %s", r.Field, r.Message))
				}
			}
			return Config{}, fmt.Errorf("invalid config:\n  %s", strings.Join(msgs, "\n  "))
		}
	}

	return cfg, nil
```

Add `"strings"` to the import block if not already present.

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/config/ -v`
Expected: all tests PASS.

- [ ] **Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: integrate validation into config.Load()"
```

---

## Task 2: `claustro validate` Command + Doctor Integration

**Files:**
- Create: `cmd/claustro/validate.go`
- Modify: `internal/doctor/doctor.go` (extend `CheckConfigFile`)
- Modify: `internal/doctor/doctor_test.go`
- Modify: `cmd/claustro/commands.go`
- Modify: `cmd/claustro/commands_test.go`

### Step 2.1: Write and implement `claustro validate` command

- [ ] **Create `cmd/claustro/validate.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate claustro.yaml configuration",
		Long:  "Check the claustro.yaml file in the current directory for errors and warnings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate()
		},
	}
}

func runValidate() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	raw, err := config.LoadRaw(dir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if raw == nil {
		fmt.Println("no claustro.yaml found")
		return nil
	}

	results := raw.Validate()
	if len(results) == 0 {
		fmt.Println("claustro.yaml: valid")
		return nil
	}

	var hasError bool
	var errorCount, warnCount int
	for _, r := range results {
		if r.Severity == config.SeverityError {
			hasError = true
			errorCount++
		} else {
			warnCount++
		}
	}

	summary := "claustro.yaml:"
	if errorCount > 0 {
		summary += fmt.Sprintf(" %d error(s)", errorCount)
	}
	if warnCount > 0 {
		if errorCount > 0 {
			summary += ","
		}
		summary += fmt.Sprintf(" %d warning(s)", warnCount)
	}
	fmt.Println(summary)

	for _, r := range results {
		fmt.Printf("  %s: %s: %s\n", r.Severity, r.Field, r.Message)
	}

	if hasError {
		return fmt.Errorf("validation failed")
	}
	return nil
}
```

- [ ] **Add `LoadRaw` to `internal/config/config.go`**

`LoadRaw` is like `Load` but returns the config without running validation (so the caller can inspect all results). Add this function below the existing `Load()`:

```go
// LoadRaw loads and parses the config without running validation.
// Returns nil if no claustro.yaml exists.
func LoadRaw(projectPath string) (*Config, error) {
	path := filepath.Join(projectPath, "claustro.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.postProcess(); err != nil {
		return nil, fmt.Errorf("processing config: %w", err)
	}

	return &cfg, nil
}
```

- [ ] **Register the validate command in `cmd/claustro/commands.go`**

Add `root.AddCommand(newValidateCmd())` inside `setupCommands()`.

- [ ] **Update `cmd/claustro/commands_test.go`**

Update the `expected` slice in `TestSetupCommands_RegistersAllCommands`:

```go
expected := []string{"burn", "claude", "config", "doctor", "exec", "init", "logs", "ls", "nuke", "rebuild", "shell", "status", "up", "validate"}
```

(Note: this list also adds "config" and "init" — those commands will be created in later tasks. For now, skip updating this test until all commands are registered. Instead, add a standalone test:)

```go
func TestValidateCmd_Exists(t *testing.T) {
	cmd := newValidateCmd()
	assert.Equal(t, "validate", cmd.Name())
}
```

- [ ] **Run tests**

Run: `cd /workspace && go test ./cmd/claustro/ -run TestValidateCmd -v`
Expected: PASS.

- [ ] **Commit**

```bash
git add cmd/claustro/validate.go internal/config/config.go cmd/claustro/commands.go cmd/claustro/commands_test.go
git commit -m "feat: add claustro validate command"
```

### Step 2.2: Extend doctor config check with full validation

- [ ] **Write the failing test**

Add to `internal/doctor/doctor_test.go`:

```go
func TestCheckConfigFile_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
defaults:
  resources:
    cpus: "not-a-number"
    memory: "invalid"
`), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, Fail, result.Status)
	assert.Contains(t, result.Detail, "cpus")
}

func TestCheckConfigFile_WarningsOnly(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
defaults:
  resources:
    cpus: "0"
`), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, Warn, result.Status)
}

func TestCheckConfigFile_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
project: myapp
defaults:
  resources:
    cpus: "4"
    memory: "8G"
`), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, Pass, result.Status)
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/doctor/ -run TestCheckConfigFile_Invalid -v`
Expected: FAIL (current implementation only checks file existence, not validation).

- [ ] **Update `CheckConfigFile` in `internal/doctor/doctor.go`**

Replace the existing `CheckConfigFile` function (currently at lines 332-351) with:

```go
// CheckConfigFile validates claustro.yaml if present.
func CheckConfigFile(dir string) CheckResult {
	name := "Config File"

	path := filepath.Join(dir, "claustro.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "no claustro.yaml found (optional)",
			FixHint: "run: claustro init",
		}
	}

	cfg, err := config.LoadRaw(dir)
	if err != nil {
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  fmt.Sprintf("failed to parse: %s", err),
			FixHint: "fix the YAML syntax in claustro.yaml",
		}
	}

	if cfg == nil {
		return CheckResult{
			Name:   name,
			Status: Pass,
			Detail: "claustro.yaml is valid",
		}
	}

	results := cfg.Validate()
	var errors, warnings int
	var details []string
	for _, r := range results {
		if r.Severity == config.SeverityError {
			errors++
			details = append(details, fmt.Sprintf("%s: %s: %s", r.Severity, r.Field, r.Message))
		} else {
			warnings++
			details = append(details, fmt.Sprintf("%s: %s: %s", r.Severity, r.Field, r.Message))
		}
	}

	if errors > 0 {
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  strings.Join(details, "; "),
			FixHint: "fix the errors in claustro.yaml",
		}
	}

	if warnings > 0 {
		return CheckResult{
			Name:   name,
			Status: Warn,
			Detail: strings.Join(details, "; "),
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: "claustro.yaml is valid",
	}
}
```

Add these imports to `internal/doctor/doctor.go`:

```go
"strings"
"github.com/uniforgeai/claustro/internal/config"
```

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/doctor/ -v`
Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/doctor/doctor.go internal/doctor/doctor_test.go
git commit -m "feat: extend doctor config check with full validation"
```

---

## Task 3: ImageBuildConfig Data Model

**Files:**
- Create: `internal/config/image_config.go`
- Create: `internal/config/image_config_test.go`
- Modify: `internal/config/config.go` (add `ImageBuild` field, update `postProcess`)

### Step 3.1: Write tests for ImageBuildConfig

- [ ] **Create `internal/config/image_config_test.go`**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultImageBuildConfig(t *testing.T) {
	cfg := DefaultImageBuildConfig()

	assert.True(t, cfg.IsLanguageEnabled("node"))
	assert.True(t, cfg.IsLanguageEnabled("go"))
	assert.True(t, cfg.IsLanguageEnabled("rust"))
	assert.True(t, cfg.IsLanguageEnabled("python"))
	assert.True(t, cfg.IsToolGroupEnabled("dev"))
	assert.True(t, cfg.IsToolGroupEnabled("build"))
	assert.True(t, cfg.IsMCPServerEnabled("filesystem"))
	assert.True(t, cfg.IsMCPServerEnabled("memory"))
	assert.True(t, cfg.IsMCPServerEnabled("fetch"))
}

func TestImageBuildConfig_DisableLanguage(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	f := false
	cfg.Languages.Go = &f

	assert.True(t, cfg.IsLanguageEnabled("node"), "node is always enabled")
	assert.False(t, cfg.IsLanguageEnabled("go"))
	assert.True(t, cfg.IsLanguageEnabled("rust"))
}

func TestImageBuildConfig_DisableTool(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	f := false
	cfg.Tools.Dev = &f

	assert.False(t, cfg.IsToolGroupEnabled("dev"))
	assert.True(t, cfg.IsToolGroupEnabled("build"))
}

func TestImageBuildConfig_DisableMCP(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	f := false
	cfg.MCPServers.Filesystem = &f

	assert.False(t, cfg.IsMCPServerEnabled("filesystem"))
	assert.True(t, cfg.IsMCPServerEnabled("memory"))
}

func TestImageBuildConfig_UnknownKeyReturnsFalse(t *testing.T) {
	cfg := DefaultImageBuildConfig()
	assert.False(t, cfg.IsLanguageEnabled("java"))
	assert.False(t, cfg.IsToolGroupEnabled("gaming"))
	assert.False(t, cfg.IsMCPServerEnabled("unknown"))
}

func TestLoad_ImageBuildConfig(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
image:
  languages:
    go: true
    rust: false
    python: true
  tools:
    dev: true
    build: false
  mcp_servers:
    filesystem: true
    memory: false
    fetch: true
`), 0o644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.False(t, cfg.ImageBuild.IsToolGroupEnabled("build"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("memory"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("fetch"))
}

func TestLoad_NoImageBlock_DefaultsAllEnabled(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(`
project: myapp
`), 0o644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("build"))
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/config/ -run TestDefaultImageBuild -v`
Expected: compilation error — `ImageBuildConfig`, `DefaultImageBuildConfig`, etc. not defined.

### Step 3.2: Implement ImageBuildConfig

- [ ] **Create `internal/config/image_config.go`**

```go
package config

// LanguagesConfig controls which language runtimes are installed in the image.
type LanguagesConfig struct {
	Node   *bool `yaml:"node"`
	Go     *bool `yaml:"go"`
	Rust   *bool `yaml:"rust"`
	Python *bool `yaml:"python"`
}

// ToolsConfig controls which tool groups are installed.
type ToolsConfig struct {
	Dev   *bool `yaml:"dev"`   // ripgrep, fd, fzf, jq, tree, htop, tmux
	Build *bool `yaml:"build"` // gcc, pkg-config, libssl-dev, make
}

// MCPServersConfig controls which MCP servers are pre-installed.
type MCPServersConfig struct {
	Filesystem *bool `yaml:"filesystem"`
	Memory     *bool `yaml:"memory"`
	Fetch      *bool `yaml:"fetch"`
}

// ImageBuildConfig controls what goes into the Docker image.
type ImageBuildConfig struct {
	Languages  LanguagesConfig  `yaml:"languages"`
	Tools      ToolsConfig      `yaml:"tools"`
	MCPServers MCPServersConfig `yaml:"mcp_servers"`
}

// DefaultImageBuildConfig returns a config with everything enabled.
func DefaultImageBuildConfig() ImageBuildConfig {
	return ImageBuildConfig{}
}

// IsLanguageEnabled returns whether a language runtime should be installed.
// Node is always true. Unknown languages return false.
// A nil pointer means true (default enabled).
func (c *ImageBuildConfig) IsLanguageEnabled(lang string) bool {
	switch lang {
	case "node":
		return true // always on
	case "go":
		return c.Languages.Go == nil || *c.Languages.Go
	case "rust":
		return c.Languages.Rust == nil || *c.Languages.Rust
	case "python":
		return c.Languages.Python == nil || *c.Languages.Python
	default:
		return false
	}
}

// IsToolGroupEnabled returns whether a tool group should be installed.
// A nil pointer means true (default enabled).
func (c *ImageBuildConfig) IsToolGroupEnabled(group string) bool {
	switch group {
	case "dev":
		return c.Tools.Dev == nil || *c.Tools.Dev
	case "build":
		return c.Tools.Build == nil || *c.Tools.Build
	default:
		return false
	}
}

// IsMCPServerEnabled returns whether an MCP server should be pre-installed.
// A nil pointer means true (default enabled).
func (c *ImageBuildConfig) IsMCPServerEnabled(server string) bool {
	switch server {
	case "filesystem":
		return c.MCPServers.Filesystem == nil || *c.MCPServers.Filesystem
	case "memory":
		return c.MCPServers.Memory == nil || *c.MCPServers.Memory
	case "fetch":
		return c.MCPServers.Fetch == nil || *c.MCPServers.Fetch
	default:
		return false
	}
}
```

- [ ] **Add `ImageBuild` field to `Config` struct in `internal/config/config.go`**

Add this field to the `Config` struct:

```go
type Config struct {
	Project   string                 `yaml:"project"`
	RawImage  yaml.Node              `yaml:"image"`
	Defaults  DefaultsConfig         `yaml:"defaults"`
	Sandboxes map[string]SandboxDef  `yaml:"sandboxes"`
	Firewall  FirewallConfig         `yaml:"firewall"`
	MCP       MCPConfig              `yaml:"mcp"`
	Git       GitConfig              `yaml:"git"`
	ImageBuild ImageBuildConfig      `yaml:"-"` // populated by postProcess

	// Parsed image fields (populated by postProcess)
	ImageName   string
	ImageConfig ImageConfig
}
```

- [ ] **Update `postProcess()` in `internal/config/config.go`**

The current `postProcess()` handles scalar vs mapping for the `image:` field. Update it to also handle the new `ImageBuildConfig` format. The `RawImage` yaml.Node now has three possible forms:

1. Empty (no `image:` key) → default ImageBuildConfig (all enabled)
2. Scalar string (e.g., `image: custom:latest`) → set ImageName
3. Mapping with `extra:` key → existing ImageConfig behavior
4. Mapping with `languages:`/`tools:`/`mcp_servers:` keys → new ImageBuildConfig

Replace the `postProcess()` method:

```go
func (c *Config) postProcess() error {
	if c.RawImage.Kind == 0 {
		// No image key present — defaults apply.
		return nil
	}

	switch c.RawImage.Kind {
	case yaml.ScalarNode:
		c.ImageName = c.RawImage.Value
	case yaml.MappingNode:
		// Try ImageBuildConfig first (has languages/tools/mcp_servers).
		// Then fall back to ImageConfig (has extra).
		var build ImageBuildConfig
		if err := c.RawImage.Decode(&build); err == nil {
			c.ImageBuild = build
		}
		var ic ImageConfig
		if err := c.RawImage.Decode(&ic); err != nil {
			return fmt.Errorf("parsing image config: %w", err)
		}
		c.ImageConfig = ic
	default:
		return fmt.Errorf("image: must be a string or a mapping, got %v", c.RawImage.Kind)
	}

	return nil
}
```

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/config/ -v`
Expected: all tests PASS.

- [ ] **Commit**

```bash
git add internal/config/image_config.go internal/config/image_config_test.go internal/config/config.go
git commit -m "feat: add ImageBuildConfig for configurable Docker image composition"
```

---

## Task 4: Templated Dockerfile

**Files:**
- Create: `internal/image/template.go`
- Create: `internal/image/template_test.go`
- Modify: `internal/image/image.go` (update `buildContext` to use template)
- Modify: `internal/image/image_test.go` (update build context tests)

### Step 4.1: Write tests for Dockerfile template rendering

- [ ] **Create `internal/image/template_test.go`**

```go
package image

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func TestRenderDockerfile_AllEnabled(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	result, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	// Always present
	assert.Contains(t, result, "FROM ubuntu:24.04")
	assert.Contains(t, result, "nodesource")
	assert.Contains(t, result, "claude-code")
	assert.Contains(t, result, "claustro-init")

	// Languages
	assert.Contains(t, result, "go.dev/dl")
	assert.Contains(t, result, "rustup")
	assert.Contains(t, result, "python3")

	// Dev tools
	assert.Contains(t, result, "ripgrep")
	assert.Contains(t, result, "fzf")

	// Build tools
	assert.Contains(t, result, "build-essential")
	assert.Contains(t, result, "pkg-config")

	// MCP servers
	assert.Contains(t, result, "server-filesystem")
	assert.Contains(t, result, "server-memory")
	assert.Contains(t, result, "mcp-server-fetch")
}

func TestRenderDockerfile_MinimalConfig(t *testing.T) {
	f := false
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     &f,
			Rust:   &f,
			Python: &f,
		},
		Tools: config.ToolsConfig{
			Dev:   &f,
			Build: &f,
		},
		MCPServers: config.MCPServersConfig{
			Filesystem: &f,
			Memory:     &f,
			Fetch:      &f,
		},
	}
	result, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	// Always present
	assert.Contains(t, result, "FROM ubuntu:24.04")
	assert.Contains(t, result, "nodesource")
	assert.Contains(t, result, "claude-code")

	// Languages excluded
	assert.NotContains(t, result, "go.dev/dl")
	assert.NotContains(t, result, "rustup")
	// Note: python3 base package may still be present for system use,
	// but the Python dev section should be absent
	assert.NotContains(t, result, "python3-venv")

	// Dev tools excluded
	assert.NotContains(t, result, "ripgrep")
	assert.NotContains(t, result, "fzf")

	// Build tools excluded
	assert.NotContains(t, result, "build-essential")

	// MCP servers excluded
	assert.NotContains(t, result, "server-filesystem")
	assert.NotContains(t, result, "server-memory")
	assert.NotContains(t, result, "mcp-server-fetch")
}

func TestRenderDockerfile_SelectiveLanguages(t *testing.T) {
	f := false
	tr := true
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     &tr,
			Rust:   &f,
			Python: &tr,
		},
	}
	result, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	assert.Contains(t, result, "go.dev/dl")
	assert.NotContains(t, result, "rustup")
	assert.Contains(t, result, "python3")
}

func TestRenderDockerfile_NodeAlwaysPresent(t *testing.T) {
	f := false
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Node: &f, // Even if explicitly false, node should still be present
		},
	}
	result, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	// Node and Claude Code are always present regardless of config
	assert.Contains(t, result, "nodesource")
	assert.Contains(t, result, "claude-code")
}

func TestRenderDockerfile_IsValidDockerfile(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	result, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	lines := strings.Split(result, "\n")
	assert.True(t, strings.HasPrefix(lines[0], "FROM "), "Dockerfile must start with FROM")
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/image/ -run TestRenderDockerfile -v`
Expected: compilation error — `RenderDockerfile` not defined.

### Step 4.2: Implement Dockerfile template

- [ ] **Create `internal/image/template.go`**

```go
package image

import (
	"bytes"
	"text/template"

	"github.com/uniforgeai/claustro/internal/config"
)

const dockerfileTmpl = `FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Base system packages (always installed)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    gnupg \
    iptables \
    zsh \
    && rm -rf /var/lib/apt/lists/*
{{if .DevTools}}
# Dev tools: ripgrep, fd, fzf, jq, tree, htop, tmux
RUN apt-get update && apt-get install -y --no-install-recommends \
    fd-find \
    fzf \
    htop \
    jq \
    ripgrep \
    tmux \
    tree \
    && rm -rf /var/lib/apt/lists/*
{{end}}
{{if .BuildTools}}
# Build essentials: gcc, make, pkg-config, libssl-dev
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    make \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*
{{end}}
# Node.js LTS (always installed — required for Claude Code)
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*
{{if .Go}}
# Go (latest stable)
RUN curl -fsSL https://go.dev/dl/go1.24.2.linux-$(dpkg --print-architecture).tar.gz \
    | tar -C /usr/local -xz
{{end}}
{{if .Rust}}
# Rust (via rustup — installed for root, configured for sandbox user later)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
{{end}}
{{if .Python}}
# Python 3 + pip + venv
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*
{{end}}
# GitHub CLI (always installed)
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | gpg --dearmor -o /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Claude Code (always installed)
# Install globally via npm. Do NOT use npx — it creates a local cache
# that conflicts with the npm-global one and breaks auto-update detection.
RUN npm install -g @anthropic-ai/claude-code

# Install MCP servers (optional)
{{if or .MCPFilesystem .MCPMemory}}
RUN npm install -g{{if .MCPFilesystem}} \
    @modelcontextprotocol/server-filesystem{{end}}{{if .MCPMemory}} \
    @modelcontextprotocol/server-memory{{end}}
{{end}}
{{if .MCPFetch}}
# MCP fetch server (Python-based)
RUN pip3 install --break-system-packages mcp-server-fetch
{{end}}
# Install ccstatusline (optional — native build may fail on some architectures)
RUN npm install -g ccstatusline || true
{{if .Go}}
# gopls for Go LSP support
RUN /usr/local/go/bin/go install golang.org/x/tools/gopls@latest \
    && mv /root/go/bin/gopls /usr/local/bin/
{{end}}
# Clipboard shims (always installed)
COPY xclip-shim /usr/local/bin/xclip
COPY wl-paste-shim /usr/local/bin/wl-paste

# Sandbox user (always — uid 1000)
RUN userdel -r ubuntu 2>/dev/null || true \
    && useradd -m -s /bin/zsh -u 1000 sandbox

COPY claustro-init /usr/local/bin/claustro-init
RUN chmod 755 /usr/local/bin/claustro-init
{{if .Rust}}
# Set up Rust for sandbox user
USER sandbox
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
USER root
{{end}}
# Writable cache directories for sandbox user
RUN mkdir -p /home/sandbox/.npm /home/sandbox/.cache/pip \
    && chown -R sandbox:sandbox /home/sandbox/.npm /home/sandbox/.cache/pip

ENV HOME=/home/sandbox
ENV PATH="/usr/local/go/bin:/home/sandbox/.cargo/bin:/home/sandbox/.local/bin:${PATH}"

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/claustro-init"]
CMD ["sleep", "infinity"]
`

// templateData holds the flags for Dockerfile template rendering.
type templateData struct {
	Go            bool
	Rust          bool
	Python        bool
	DevTools      bool
	BuildTools    bool
	MCPFilesystem bool
	MCPMemory     bool
	MCPFetch      bool
}

// RenderDockerfile renders the Dockerfile template using the given config.
func RenderDockerfile(cfg *config.ImageBuildConfig) (string, error) {
	data := templateData{
		Go:            cfg.IsLanguageEnabled("go"),
		Rust:          cfg.IsLanguageEnabled("rust"),
		Python:        cfg.IsLanguageEnabled("python"),
		DevTools:      cfg.IsToolGroupEnabled("dev"),
		BuildTools:    cfg.IsToolGroupEnabled("build"),
		MCPFilesystem: cfg.IsMCPServerEnabled("filesystem"),
		MCPMemory:     cfg.IsMCPServerEnabled("memory"),
		MCPFetch:      cfg.IsMCPServerEnabled("fetch"),
	}

	tmpl, err := template.New("Dockerfile").Parse(dockerfileTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
```

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/image/ -run TestRenderDockerfile -v`
Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/image/template.go internal/image/template_test.go
git commit -m "feat: add templated Dockerfile rendering for configurable images"
```

### Step 4.3: Wire template into build pipeline

- [ ] **Update `internal/image/image.go`**

Replace the static `//go:embed Dockerfile` with template rendering. The `buildContext()` function currently reads the embedded `dockerfile` bytes. Update it to accept an `ImageBuildConfig` and render via the template.

Change the function signature of `buildContext` and update callers:

1. Remove the `//go:embed Dockerfile` and `var dockerfile []byte` lines.

2. Update `buildContext()` to:

```go
func buildContext(imgCfg *config.ImageBuildConfig) ([]byte, error) {
	rendered, err := RenderDockerfile(imgCfg)
	if err != nil {
		return nil, fmt.Errorf("rendering Dockerfile: %w", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	files := []struct {
		name    string
		content []byte
		mode    int64
	}{
		{"Dockerfile", []byte(rendered), 0644},
		{"claustro-init", initScript, 0755},
		{"xclip-shim", xclipShim, 0755},
		{"wl-paste-shim", wlPasteShim, 0755},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.name,
			Mode: f.mode,
			Size: int64(len(f.content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
```

3. Add `"github.com/uniforgeai/claustro/internal/config"` to imports.

4. Update `buildImage()` to accept and pass `imgCfg`:

```go
func buildImage(ctx context.Context, cli *client.Client, imgCfg *config.ImageBuildConfig, noCache bool, w io.Writer) error {
	buildCtx, err := buildContext(imgCfg)
	// ... rest unchanged
}
```

5. Update `EnsureBuilt()` and `Build()` signatures to accept `imgCfg *config.ImageBuildConfig`:

```go
func EnsureBuilt(ctx context.Context, cli *client.Client, imgCfg *config.ImageBuildConfig, w io.Writer) error {
	// ...
	return buildImage(ctx, cli, imgCfg, false, w)
}

func Build(ctx context.Context, cli *client.Client, imgCfg *config.ImageBuildConfig, w io.Writer) error {
	// ...
	return buildImage(ctx, cli, imgCfg, true, w)
}
```

- [ ] **Update callers in `cmd/claustro/up.go`**

In `ensureRunning()`, the call to `image.EnsureBuilt()` needs to pass the image config from the loaded config. Update the call:

```go
// Before:
if err := image.EnsureBuilt(ctx, cli, os.Stdout); err != nil {
// After:
if err := image.EnsureBuilt(ctx, cli, &cfg.ImageBuild, os.Stdout); err != nil {
```

Similarly update `cmd/claustro/rebuild.go` to pass a default config (since rebuild doesn't load project config by default — use `config.DefaultImageBuildConfig()`):

```go
imgCfg := config.DefaultImageBuildConfig()
if err := image.Build(ctx, cli, &imgCfg, os.Stdout); err != nil {
```

- [ ] **Update `internal/image/image_test.go`**

Update `TestBuildContext_ContainsRequiredFiles` and `TestBuildContext_InitScriptIsExecutable` to pass a config:

```go
func TestBuildContext_ContainsRequiredFiles(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	ctx, err := buildContext(&cfg)
	require.NoError(t, err)
	// ... rest unchanged
}

func TestBuildContext_InitScriptIsExecutable(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	ctx, err := buildContext(&cfg)
	require.NoError(t, err)
	// ... rest unchanged
}
```

Add import: `"github.com/uniforgeai/claustro/internal/config"`

- [ ] **Run all tests**

Run: `cd /workspace && go test ./...`
Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/image/image.go internal/image/image_test.go internal/image/template.go cmd/claustro/up.go cmd/claustro/rebuild.go
git commit -m "feat: wire templated Dockerfile into image build pipeline"
```

---

## Task 5: Add `charmbracelet/huh` Dependency

**Files:**
- Modify: `go.mod`

### Step 5.1: Add the dependency

- [ ] **Run go get**

```bash
cd /workspace && go get github.com/charmbracelet/huh@latest
```

- [ ] **Tidy modules**

```bash
cd /workspace && go mod tidy
```

- [ ] **Verify build**

Run: `cd /workspace && go build ./...`
Expected: no errors.

- [ ] **Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add charmbracelet/huh dependency for TUI forms"
```

---

## Task 6: `claustro init` Wizard

**Files:**
- Create: `internal/wizard/wizard.go`
- Create: `internal/wizard/wizard_test.go`
- Create: `cmd/claustro/init_cmd.go`

### Step 6.1: Write tests for wizard config generation

- [ ] **Create `internal/wizard/wizard_test.go`**

```go
package wizard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConfig_Defaults(t *testing.T) {
	opts := Options{
		Project:    "myapp",
		Languages:  []string{"go", "rust", "python"},
		Tools:      []string{"dev", "build"},
		MCPServers: []string{"filesystem", "memory", "fetch"},
		CPUs:       "4",
		Memory:     "8G",
		Firewall:   false,
		ReadOnly:   false,
		ForwardAgent: true,
		MountGitconfig: true,
		MountGhConfig: true,
	}

	cfg := BuildConfig(opts)

	assert.Equal(t, "myapp", cfg.Project)
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("build"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.Equal(t, "4", cfg.Defaults.Resources.CPUs)
	assert.Equal(t, "8G", cfg.Defaults.Resources.Memory)
}

func TestBuildConfig_SelectiveLanguages(t *testing.T) {
	opts := Options{
		Project:    "myapp",
		Languages:  []string{"go", "python"},
		Tools:      []string{"dev"},
		MCPServers: []string{"filesystem"},
		CPUs:       "2",
		Memory:     "4G",
	}

	cfg := BuildConfig(opts)

	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"), "node always enabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.False(t, cfg.ImageBuild.IsToolGroupEnabled("build"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("memory"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("fetch"))
}

func TestBuildConfig_FirewallEnabled(t *testing.T) {
	opts := Options{
		Project:  "myapp",
		Firewall: true,
	}

	cfg := BuildConfig(opts)

	require.NotNil(t, cfg.Firewall.Enabled)
	assert.True(t, *cfg.Firewall.Enabled)
}

func TestMarshalConfig(t *testing.T) {
	opts := Options{
		Project:   "myapp",
		Languages: []string{"go", "python"},
		Tools:     []string{"dev"},
		CPUs:      "4",
		Memory:    "8G",
	}

	cfg := BuildConfig(opts)
	data, err := MarshalConfig(cfg)
	require.NoError(t, err)

	assert.Contains(t, string(data), "project: myapp")
	assert.Contains(t, string(data), "go: true")
	assert.Contains(t, string(data), "rust: false")
	assert.Contains(t, string(data), "python: true")
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions("my-project")
	assert.Equal(t, "my-project", opts.Project)
	assert.Contains(t, opts.Languages, "go")
	assert.Contains(t, opts.Languages, "rust")
	assert.Contains(t, opts.Languages, "python")
	assert.Contains(t, opts.Tools, "dev")
	assert.Contains(t, opts.Tools, "build")
	assert.Contains(t, opts.MCPServers, "filesystem")
	assert.Contains(t, opts.MCPServers, "memory")
	assert.Contains(t, opts.MCPServers, "fetch")
	assert.Equal(t, "4", opts.CPUs)
	assert.Equal(t, "8G", opts.Memory)
}
```

- [ ] **Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/wizard/ -v`
Expected: compilation error — package does not exist.

### Step 6.2: Implement wizard config builder

- [ ] **Create `internal/wizard/wizard.go`**

```go
package wizard

import (
	"github.com/uniforgeai/claustro/internal/config"
	"gopkg.in/yaml.v3"
)

// Options holds all user choices from the init wizard or CLI flags.
type Options struct {
	Project        string
	Languages      []string // subset of: go, rust, python
	Tools          []string // subset of: dev, build
	MCPServers     []string // subset of: filesystem, memory, fetch
	CPUs           string
	Memory         string
	Firewall       bool
	ReadOnly       bool
	ForwardAgent   bool
	MountGitconfig bool
	MountGhConfig  bool
}

// DefaultOptions returns sensible defaults for the wizard.
func DefaultOptions(project string) Options {
	return Options{
		Project:        project,
		Languages:      []string{"go", "rust", "python"},
		Tools:          []string{"dev", "build"},
		MCPServers:     []string{"filesystem", "memory", "fetch"},
		CPUs:           "4",
		Memory:         "8G",
		Firewall:       false,
		ReadOnly:       false,
		ForwardAgent:   true,
		MountGitconfig: true,
		MountGhConfig:  true,
	}
}

// BuildConfig converts wizard options into a config.Config.
func BuildConfig(opts Options) config.Config {
	cfg := config.Config{
		Project: opts.Project,
		Defaults: config.DefaultsConfig{
			Resources: config.ResourcesConfig{
				CPUs:   opts.CPUs,
				Memory: opts.Memory,
			},
		},
	}

	if opts.Firewall {
		cfg.Firewall.Enabled = boolPtr(true)
	}

	if opts.ReadOnly {
		cfg.Defaults.ReadOnly = boolPtr(true)
	}

	cfg.Git = config.GitConfig{
		ForwardAgent:   boolPtr(opts.ForwardAgent),
		MountGitconfig: boolPtr(opts.MountGitconfig),
		MountGhConfig:  boolPtr(opts.MountGhConfig),
	}

	cfg.ImageBuild = buildImageConfig(opts)

	return cfg
}

func buildImageConfig(opts Options) config.ImageBuildConfig {
	langSet := toSet(opts.Languages)
	toolSet := toSet(opts.Tools)
	mcpSet := toSet(opts.MCPServers)

	return config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     boolPtr(langSet["go"]),
			Rust:   boolPtr(langSet["rust"]),
			Python: boolPtr(langSet["python"]),
		},
		Tools: config.ToolsConfig{
			Dev:   boolPtr(toolSet["dev"]),
			Build: boolPtr(toolSet["build"]),
		},
		MCPServers: config.MCPServersConfig{
			Filesystem: boolPtr(mcpSet["filesystem"]),
			Memory:     boolPtr(mcpSet["memory"]),
			Fetch:      boolPtr(mcpSet["fetch"]),
		},
	}
}

// MarshalConfig serializes a Config to YAML bytes suitable for claustro.yaml.
func MarshalConfig(cfg config.Config) ([]byte, error) {
	// Build a clean output structure to avoid yaml.Node serialization issues.
	out := marshalableConfig{
		Project: cfg.Project,
		Image:   marshalableImage{Languages: marshalableLangs{}, Tools: marshalableTools{}, MCPServers: marshalableMCP{}},
		Defaults: marshalableDefaults{
			Resources: marshalableResources{
				CPUs:   cfg.Defaults.Resources.CPUs,
				Memory: cfg.Defaults.Resources.Memory,
			},
		},
		Git: cfg.Git,
	}

	if cfg.Firewall.Enabled != nil {
		out.Firewall.Enabled = cfg.Firewall.Enabled
	}
	if cfg.Defaults.ReadOnly != nil {
		out.Defaults.ReadOnly = cfg.Defaults.ReadOnly
	}

	out.Image.Languages.Node = true
	out.Image.Languages.Go = cfg.ImageBuild.IsLanguageEnabled("go")
	out.Image.Languages.Rust = cfg.ImageBuild.IsLanguageEnabled("rust")
	out.Image.Languages.Python = cfg.ImageBuild.IsLanguageEnabled("python")
	out.Image.Tools.Dev = cfg.ImageBuild.IsToolGroupEnabled("dev")
	out.Image.Tools.Build = cfg.ImageBuild.IsToolGroupEnabled("build")
	out.Image.MCPServers.Filesystem = cfg.ImageBuild.IsMCPServerEnabled("filesystem")
	out.Image.MCPServers.Memory = cfg.ImageBuild.IsMCPServerEnabled("memory")
	out.Image.MCPServers.Fetch = cfg.ImageBuild.IsMCPServerEnabled("fetch")

	return yaml.Marshal(out)
}

type marshalableConfig struct {
	Project  string              `yaml:"project"`
	Image    marshalableImage    `yaml:"image"`
	Defaults marshalableDefaults `yaml:"defaults"`
	Firewall marshalableFW       `yaml:"firewall,omitempty"`
	Git      config.GitConfig    `yaml:"git"`
}

type marshalableImage struct {
	Languages  marshalableLangs `yaml:"languages"`
	Tools      marshalableTools `yaml:"tools"`
	MCPServers marshalableMCP   `yaml:"mcp_servers"`
}

type marshalableLangs struct {
	Node   bool `yaml:"node"`
	Go     bool `yaml:"go"`
	Rust   bool `yaml:"rust"`
	Python bool `yaml:"python"`
}

type marshalableTools struct {
	Dev   bool `yaml:"dev"`
	Build bool `yaml:"build"`
}

type marshalableMCP struct {
	Filesystem bool `yaml:"filesystem"`
	Memory     bool `yaml:"memory"`
	Fetch      bool `yaml:"fetch"`
}

type marshalableDefaults struct {
	ReadOnly  *bool               `yaml:"readonly,omitempty"`
	Resources marshalableResources `yaml:"resources"`
}

type marshalableResources struct {
	CPUs   string `yaml:"cpus"`
	Memory string `yaml:"memory"`
}

type marshalableFW struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

func boolPtr(b bool) *bool {
	return &b
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool)
	for _, item := range items {
		m[item] = true
	}
	return m
}
```

- [ ] **Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/wizard/ -v`
Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/wizard/wizard.go internal/wizard/wizard_test.go
git commit -m "feat: add wizard config builder for init command"
```

### Step 6.3: Create `claustro init` command

- [ ] **Create `cmd/claustro/init_cmd.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/wizard"
)

func newInitCmd() *cobra.Command {
	var (
		project    string
		languages  string
		tools      string
		mcpServers string
		cpus       string
		memory     string
		firewall   bool
		readonly   bool
		yes        bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new claustro project",
		Long: `Interactive wizard that generates a claustro.yaml configuration file.
Walks through project settings, image composition (languages, tools, MCP servers),
resource defaults, and security options.

Use --yes to accept all defaults without prompts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, project, languages, tools, mcpServers, cpus, memory, firewall, readonly, yes)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name (default: directory basename)")
	cmd.Flags().StringVar(&languages, "languages", "", "Comma-separated languages: go,rust,python")
	cmd.Flags().StringVar(&tools, "tools", "", "Comma-separated tool groups: dev,build")
	cmd.Flags().StringVar(&mcpServers, "mcp", "", "Comma-separated MCP servers: filesystem,memory,fetch")
	cmd.Flags().StringVar(&cpus, "cpus", "", "CPU limit (default: 4)")
	cmd.Flags().StringVar(&memory, "memory", "", "Memory limit (default: 8G)")
	cmd.Flags().BoolVar(&firewall, "firewall", false, "Enable egress firewall")
	cmd.Flags().BoolVar(&readonly, "readonly", false, "Mount source as read-only")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Accept all defaults, no prompts")

	return cmd
}

func runInit(cmd *cobra.Command, project, languages, tools, mcpServers, cpus, memory string, firewall, readonly, yes bool) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Check if claustro.yaml already exists.
	configPath := filepath.Join(dir, "claustro.yaml")
	if _, err := os.Stat(configPath); err == nil && !yes {
		var overwrite bool
		err := huh.NewConfirm().
			Title("claustro.yaml already exists. Overwrite?").
			Value(&overwrite).
			Run()
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Build default options.
	defaultProject := filepath.Base(dir)
	opts := wizard.DefaultOptions(defaultProject)

	// Apply flag overrides.
	if project != "" {
		opts.Project = project
	}
	if cmd.Flags().Changed("languages") {
		opts.Languages = splitCSV(languages)
	}
	if cmd.Flags().Changed("tools") {
		opts.Tools = splitCSV(tools)
	}
	if cmd.Flags().Changed("mcp") {
		opts.MCPServers = splitCSV(mcpServers)
	}
	if cpus != "" {
		opts.CPUs = cpus
	}
	if memory != "" {
		opts.Memory = memory
	}
	if cmd.Flags().Changed("firewall") {
		opts.Firewall = firewall
	}
	if cmd.Flags().Changed("readonly") {
		opts.ReadOnly = readonly
	}

	// Interactive wizard (skipped with --yes).
	if !yes {
		opts, err = runWizard(opts)
		if err != nil {
			return err
		}
	}

	// Generate and write config.
	cfg := wizard.BuildConfig(opts)
	data, err := wizard.MarshalConfig(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("writing claustro.yaml: %w", err)
	}

	fmt.Printf("Created claustro.yaml for project %q\n", opts.Project)
	return nil
}

func runWizard(opts wizard.Options) (wizard.Options, error) {
	// Step 1: Project name
	err := huh.NewInput().
		Title("Project name").
		Value(&opts.Project).
		Run()
	if err != nil {
		return opts, err
	}

	// Step 2: Languages
	var selectedLangs []string
	err = huh.NewMultiSelect[string]().
		Title("Languages (Node.js always included)").
		Options(
			huh.NewOption("Go", "go").Selected(contains(opts.Languages, "go")),
			huh.NewOption("Rust", "rust").Selected(contains(opts.Languages, "rust")),
			huh.NewOption("Python", "python").Selected(contains(opts.Languages, "python")),
		).
		Value(&selectedLangs).
		Run()
	if err != nil {
		return opts, err
	}
	opts.Languages = selectedLangs

	// Step 3: Tool groups
	var selectedTools []string
	err = huh.NewMultiSelect[string]().
		Title("Tool groups").
		Options(
			huh.NewOption("Dev tools (ripgrep, fd, fzf, jq, tree, htop, tmux)", "dev").Selected(contains(opts.Tools, "dev")),
			huh.NewOption("Build essentials (gcc, make, pkg-config, libssl-dev)", "build").Selected(contains(opts.Tools, "build")),
		).
		Value(&selectedTools).
		Run()
	if err != nil {
		return opts, err
	}
	opts.Tools = selectedTools

	// Step 4: MCP servers
	var selectedMCP []string
	err = huh.NewMultiSelect[string]().
		Title("MCP servers").
		Options(
			huh.NewOption("Filesystem", "filesystem").Selected(contains(opts.MCPServers, "filesystem")),
			huh.NewOption("Memory", "memory").Selected(contains(opts.MCPServers, "memory")),
			huh.NewOption("Fetch", "fetch").Selected(contains(opts.MCPServers, "fetch")),
		).
		Value(&selectedMCP).
		Run()
	if err != nil {
		return opts, err
	}
	opts.MCPServers = selectedMCP

	// Step 5: Resources
	err = huh.NewInput().
		Title("CPU limit").
		Value(&opts.CPUs).
		Run()
	if err != nil {
		return opts, err
	}

	err = huh.NewInput().
		Title("Memory limit (e.g., 8G, 512M)").
		Value(&opts.Memory).
		Run()
	if err != nil {
		return opts, err
	}

	// Step 6: Firewall
	err = huh.NewConfirm().
		Title("Enable egress firewall?").
		Value(&opts.Firewall).
		Run()
	if err != nil {
		return opts, err
	}

	// Step 7: Read-only
	err = huh.NewConfirm().
		Title("Mount source as read-only?").
		Value(&opts.ReadOnly).
		Run()
	if err != nil {
		return opts, err
	}

	// Step 8: Git config
	err = huh.NewConfirm().
		Title("Forward SSH agent?").
		Value(&opts.ForwardAgent).
		Run()
	if err != nil {
		return opts, err
	}

	err = huh.NewConfirm().
		Title("Mount ~/.gitconfig?").
		Value(&opts.MountGitconfig).
		Run()
	if err != nil {
		return opts, err
	}

	err = huh.NewConfirm().
		Title("Mount ~/.config/gh/ (GitHub CLI)?").
		Value(&opts.MountGhConfig).
		Run()
	if err != nil {
		return opts, err
	}

	return opts, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

- [ ] **Register in `cmd/claustro/commands.go`**

Add `root.AddCommand(newInitCmd())` inside `setupCommands()`.

- [ ] **Run build and tests**

Run: `cd /workspace && go build ./... && go test ./cmd/claustro/ -run TestSetup -v`
Expected: build succeeds. (Interactive tests can't run in CI — the command test just checks registration.)

- [ ] **Commit**

```bash
git add cmd/claustro/init_cmd.go cmd/claustro/commands.go
git commit -m "feat: add claustro init wizard command"
```

---

## Task 7: `claustro config` Command

**Files:**
- Create: `cmd/claustro/config_cmd.go`

### Step 7.1: Implement config command with subcommands and get/set

- [ ] **Create `cmd/claustro/config_cmd.go`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/wizard"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify claustro configuration",
		Long:  "Interactive subcommands to modify claustro.yaml sections, or get/set individual values.",
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigLanguagesCmd())
	cmd.AddCommand(newConfigToolsCmd())
	cmd.AddCommand(newConfigMCPCmd())
	cmd.AddCommand(newConfigDefaultsCmd())
	cmd.AddCommand(newConfigFirewallCmd())
	cmd.AddCommand(newConfigGitCmd())

	return cmd
}

// --- get/set subcommands ---

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path>",
		Short: "Get a configuration value",
		Long:  "Get a value from claustro.yaml using dot-notation (e.g., image.languages.go).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigGet(args[0])
		},
	}
}

func runConfigGet(path string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "claustro.yaml"))
	if err != nil {
		return fmt.Errorf("reading claustro.yaml: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing claustro.yaml: %w", err)
	}

	val, err := getNestedValue(raw, path)
	if err != nil {
		return err
	}

	fmt.Println(val)
	return nil
}

func getNestedValue(m map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var current any = m

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found in path %q", part, path)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot traverse into %T at key %q", current, part)
		}
	}

	return current, nil
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> <value>",
		Short: "Set a configuration value",
		Long:  "Set a value in claustro.yaml using dot-notation (e.g., image.languages.rust true).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(args[0], args[1])
		},
	}
}

func runConfigSet(path, value string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	configPath := filepath.Join(dir, "claustro.yaml")

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		data = []byte{}
	} else if err != nil {
		return fmt.Errorf("reading claustro.yaml: %w", err)
	}

	var raw map[string]any
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing claustro.yaml: %w", err)
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}

	setNestedValue(raw, path, parseValue(value))

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("writing claustro.yaml: %w", err)
	}

	fmt.Printf("Set %s = %s\n", path, value)
	return nil
}

func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part]
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		if nextMap, ok := next.(map[string]any); ok {
			current = nextMap
		} else {
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
}

func parseValue(s string) any {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	return s
}

// --- interactive subcommands ---

func newConfigLanguagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "languages",
		Short: "Configure image languages interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("languages", func(cfg *config.Config) error {
				var selected []string
				err := huh.NewMultiSelect[string]().
					Title("Languages (Node.js always included)").
					Options(
						huh.NewOption("Go", "go").Selected(cfg.ImageBuild.IsLanguageEnabled("go")),
						huh.NewOption("Rust", "rust").Selected(cfg.ImageBuild.IsLanguageEnabled("rust")),
						huh.NewOption("Python", "python").Selected(cfg.ImageBuild.IsLanguageEnabled("python")),
					).
					Value(&selected).
					Run()
				if err != nil {
					return err
				}
				set := toSet(selected)
				cfg.ImageBuild.Languages.Go = boolPtr(set["go"])
				cfg.ImageBuild.Languages.Rust = boolPtr(set["rust"])
				cfg.ImageBuild.Languages.Python = boolPtr(set["python"])
				return nil
			})
		},
	}
}

func newConfigToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "Configure image tool groups interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("tools", func(cfg *config.Config) error {
				var selected []string
				err := huh.NewMultiSelect[string]().
					Title("Tool groups").
					Options(
						huh.NewOption("Dev tools (ripgrep, fd, fzf, jq, tree, htop, tmux)", "dev").Selected(cfg.ImageBuild.IsToolGroupEnabled("dev")),
						huh.NewOption("Build essentials (gcc, make, pkg-config, libssl-dev)", "build").Selected(cfg.ImageBuild.IsToolGroupEnabled("build")),
					).
					Value(&selected).
					Run()
				if err != nil {
					return err
				}
				set := toSet(selected)
				cfg.ImageBuild.Tools.Dev = boolPtr(set["dev"])
				cfg.ImageBuild.Tools.Build = boolPtr(set["build"])
				return nil
			})
		},
	}
}

func newConfigMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Configure MCP servers interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("mcp-servers", func(cfg *config.Config) error {
				var selected []string
				err := huh.NewMultiSelect[string]().
					Title("MCP servers").
					Options(
						huh.NewOption("Filesystem", "filesystem").Selected(cfg.ImageBuild.IsMCPServerEnabled("filesystem")),
						huh.NewOption("Memory", "memory").Selected(cfg.ImageBuild.IsMCPServerEnabled("memory")),
						huh.NewOption("Fetch", "fetch").Selected(cfg.ImageBuild.IsMCPServerEnabled("fetch")),
					).
					Value(&selected).
					Run()
				if err != nil {
					return err
				}
				set := toSet(selected)
				cfg.ImageBuild.MCPServers.Filesystem = boolPtr(set["filesystem"])
				cfg.ImageBuild.MCPServers.Memory = boolPtr(set["memory"])
				cfg.ImageBuild.MCPServers.Fetch = boolPtr(set["fetch"])
				return nil
			})
		},
	}
}

func newConfigDefaultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "defaults",
		Short: "Configure resource defaults interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("defaults", func(cfg *config.Config) error {
				cpus := cfg.Defaults.Resources.CPUs
				if cpus == "" {
					cpus = "4"
				}
				mem := cfg.Defaults.Resources.Memory
				if mem == "" {
					mem = "8G"
				}

				err := huh.NewInput().
					Title("CPU limit").
					Value(&cpus).
					Run()
				if err != nil {
					return err
				}

				err = huh.NewInput().
					Title("Memory limit (e.g., 8G, 512M)").
					Value(&mem).
					Run()
				if err != nil {
					return err
				}

				var ro bool
				if cfg.Defaults.ReadOnly != nil {
					ro = *cfg.Defaults.ReadOnly
				}
				err = huh.NewConfirm().
					Title("Mount source as read-only by default?").
					Value(&ro).
					Run()
				if err != nil {
					return err
				}

				cfg.Defaults.Resources.CPUs = cpus
				cfg.Defaults.Resources.Memory = mem
				cfg.Defaults.ReadOnly = boolPtr(ro)
				return nil
			})
		},
	}
}

func newConfigFirewallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "firewall",
		Short: "Configure firewall settings interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("firewall", func(cfg *config.Config) error {
				enabled := false
				if cfg.Firewall.Enabled != nil {
					enabled = *cfg.Firewall.Enabled
				}

				err := huh.NewConfirm().
					Title("Enable egress firewall?").
					Value(&enabled).
					Run()
				if err != nil {
					return err
				}

				cfg.Firewall.Enabled = boolPtr(enabled)
				return nil
			})
		},
	}
}

func newConfigGitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "git",
		Short: "Configure git integration interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("git", func(cfg *config.Config) error {
				fwd := cfg.Git.IsForwardAgent()
				gitcfg := cfg.Git.IsMountGitconfig()
				ghcfg := cfg.Git.IsMountGhConfig()

				if err := huh.NewConfirm().Title("Forward SSH agent?").Value(&fwd).Run(); err != nil {
					return err
				}
				if err := huh.NewConfirm().Title("Mount ~/.gitconfig?").Value(&gitcfg).Run(); err != nil {
					return err
				}
				if err := huh.NewConfirm().Title("Mount ~/.config/gh/?").Value(&ghcfg).Run(); err != nil {
					return err
				}

				cfg.Git.ForwardAgent = boolPtr(fwd)
				cfg.Git.MountGitconfig = boolPtr(gitcfg)
				cfg.Git.MountGhConfig = boolPtr(ghcfg)
				return nil
			})
		},
	}
}

// runConfigSection loads the config, runs the editor function, and saves.
func runConfigSection(section string, editor func(*config.Config) error) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, err := config.LoadRaw(dir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		empty := config.Config{}
		cfg = &empty
	}

	if err := editor(cfg); err != nil {
		return err
	}

	data, err := wizard.MarshalConfig(*cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	configPath := filepath.Join(dir, "claustro.yaml")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("writing claustro.yaml: %w", err)
	}

	fmt.Printf("Updated %s configuration\n", section)
	return nil
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool)
	for _, item := range items {
		m[item] = true
	}
	return m
}

func boolPtr(b bool) *bool {
	return &b
}
```

- [ ] **Register in `cmd/claustro/commands.go`**

Add `root.AddCommand(newConfigCmd())` inside `setupCommands()`.

- [ ] **Run build**

Run: `cd /workspace && go build ./...`
Expected: no errors.

- [ ] **Commit**

```bash
git add cmd/claustro/config_cmd.go cmd/claustro/commands.go
git commit -m "feat: add claustro config command with subcommands and get/set"
```

---

## Task 8: Update Command Registration Tests

**Files:**
- Modify: `cmd/claustro/commands_test.go`

### Step 8.1: Update the all-commands test

- [ ] **Update `TestSetupCommands_RegistersAllCommands`**

Replace the `expected` slice:

```go
expected := []string{"burn", "claude", "config", "doctor", "exec", "init", "logs", "ls", "nuke", "rebuild", "shell", "status", "up", "validate"}
```

- [ ] **Add tests for new commands**

Add to `cmd/claustro/commands_test.go`:

```go
func TestInitCmd_Defaults(t *testing.T) {
	cmd := newInitCmd()
	assert.Equal(t, "init", cmd.Name())

	flags := []string{"project", "languages", "tools", "mcp", "cpus", "memory", "firewall", "readonly", "yes"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			f := cmd.Flags().Lookup(name)
			assert.NotNil(t, f, "flag %q should exist", name)
		})
	}
}

func TestConfigCmd_HasSubcommands(t *testing.T) {
	cmd := newConfigCmd()
	assert.Equal(t, "config", cmd.Name())

	subs := []string{"get", "set", "languages", "tools", "mcp", "defaults", "firewall", "git"}
	for _, name := range subs {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, sub := range cmd.Commands() {
				if sub.Name() == name {
					found = true
					break
				}
			}
			assert.True(t, found, "subcommand %q should exist", name)
		})
	}
}

func TestValidateCmd_Defaults(t *testing.T) {
	cmd := newValidateCmd()
	assert.Equal(t, "validate", cmd.Name())
}
```

- [ ] **Run tests**

Run: `cd /workspace && go test ./cmd/claustro/ -v`
Expected: all PASS.

- [ ] **Commit**

```bash
git add cmd/claustro/commands_test.go
git commit -m "test: update command registration tests for init, config, validate"
```

---

## Task 9: GoReleaser Configuration

**Files:**
- Create: `.goreleaser.yaml`

### Step 9.1: Create GoReleaser config

- [ ] **Create `.goreleaser.yaml`**

```yaml
version: 2

project_name: claustro

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - id: claustro
    main: ./cmd/claustro
    binary: claustro
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w

archives:
  - id: claustro
    builds:
      - claustro
    format_overrides:
      - goos: darwin
        format: zip
    files:
      - README.md
      - LICENSE*

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
  groups:
    - title: Features
      regexp: '^.*?feat(\(.+\))??!?:.+$'
    - title: Bug Fixes
      regexp: '^.*?fix(\(.+\))??!?:.+$'
    - title: Refactoring
      regexp: '^.*?refactor(\(.+\))??!?:.+$'

brews:
  - name: claustro
    repository:
      owner: uniforgeai
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://github.com/uniforgeai/claustro"
    description: "Disposable Docker sandboxes for Claude Code"
    license: "MIT"
    install: |
      bin.install "claustro"
    test: |
      system "#{bin}/claustro", "--help"

release:
  github:
    owner: uniforgeai
    name: claustro
  draft: false
  prerelease: auto
```

- [ ] **Commit**

```bash
git add .goreleaser.yaml
git commit -m "chore: add GoReleaser configuration for cross-platform releases"
```

---

## Task 10: GitHub Actions Workflows

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `.github/workflows/docs.yml`

### Step 10.1: Create CI workflow

- [ ] **Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build
        run: go build ./...

      - name: Test
        run: go test ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

- [ ] **Commit**

```bash
mkdir -p .github/workflows
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions CI workflow for build, test, lint"
```

### Step 10.2: Create release workflow

- [ ] **Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

- [ ] **Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow with GoReleaser"
```

### Step 10.3: Create docs deployment workflow

- [ ] **Create `.github/workflows/docs.yml`**

```yaml
name: Deploy Docs

on:
  push:
    branches: [main]
    paths:
      - "docs/site/**"

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: true

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v3
        with:
          hugo-version: latest

      - name: Build site
        run: hugo -s docs/site --minify

      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: docs/site/public

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

- [ ] **Commit**

```bash
git add .github/workflows/docs.yml
git commit -m "ci: add GitHub Actions docs deployment workflow for Hugo site"
```

---

## Task 11: Hugo Documentation Site

**Files:**
- Create: `docs/site/hugo.toml`
- Create: `docs/site/content/_index.md`
- Create: `docs/site/content/getting-started/_index.md`
- Create: `docs/site/content/getting-started/installation.md`
- Create: `docs/site/content/getting-started/quickstart.md`
- Create: `docs/site/content/getting-started/configuration.md`
- Create: `docs/site/content/commands/_index.md`
- Create: `docs/site/content/commands/*.md` (one per command)
- Create: `docs/site/content/guides/_index.md`
- Create: `docs/site/content/guides/*.md`
- Create: `docs/site/content/reference/_index.md`
- Create: `docs/site/content/reference/claustro-yaml.md`
- Create: `docs/site/content/reference/environment-variables.md`

### Step 11.1: Initialize Hugo site structure

- [ ] **Create `docs/site/hugo.toml`**

```toml
baseURL = "https://uniforgeai.github.io/claustro/"
title = "claustro"
theme = "hugo-book"

[params]
  BookToC = true
  BookSection = "/"
  BookMenuBundle = "/menu"
  BookRepo = "https://github.com/uniforgeai/claustro"

[markup.goldmark.renderer]
  unsafe = true
```

- [ ] **Create `docs/site/content/_index.md`**

```markdown
---
title: claustro
type: docs
---

# claustro

Disposable Docker sandboxes for Claude Code.

claustro manages Docker containers for running Claude Code safely against local source code. Source stays on the host (bind-mounted), containers are cheap to burn and respawn.

## Key Features

- **Configurable images** — pick your languages, tools, and MCP servers
- **Multiple sandboxes** — run parallel Claude Code sessions per project
- **Egress firewall** — optional outbound traffic restriction
- **MCP server support** — stdio and SSE-based MCP servers
- **Git integration** — SSH agent, gitconfig, and GitHub CLI forwarding
- **Zero conflict** — never touches your project's docker-compose setup

## Quick Start

```bash
brew tap uniforgeai/tap
brew install claustro

cd ~/projects/my-app
claustro init
claustro up
claustro claude
```

See [Getting Started](getting-started/) for the full guide.
```

- [ ] **Commit**

```bash
mkdir -p docs/site/content/getting-started docs/site/content/commands docs/site/content/guides docs/site/content/reference
git add docs/site/hugo.toml docs/site/content/_index.md
git commit -m "docs: initialize Hugo documentation site with landing page"
```

### Step 11.2: Create getting-started pages

- [ ] **Create `docs/site/content/getting-started/_index.md`**

```markdown
---
title: Getting Started
weight: 1
bookCollapseSection: false
---

# Getting Started

Get up and running with claustro in minutes.
```

- [ ] **Create `docs/site/content/getting-started/installation.md`**

```markdown
---
title: Installation
weight: 1
---

# Installation

## Homebrew (macOS / Linux)

```bash
brew tap uniforgeai/tap
brew install claustro
```

## Binary Download

Download the latest release from [GitHub Releases](https://github.com/uniforgeai/claustro/releases).

Available platforms:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64` (Intel Mac)
- `darwin/arm64` (Apple Silicon)

## From Source

```bash
go install github.com/uniforgeai/claustro/cmd/claustro@latest
```

## Prerequisites

- **Docker Engine** or **Docker Desktop** must be installed and running
- Run `claustro doctor` to verify your environment
```

- [ ] **Create `docs/site/content/getting-started/quickstart.md`**

```markdown
---
title: Quick Start
weight: 2
---

# Quick Start

## 1. Initialize your project

```bash
cd ~/projects/my-app
claustro init
```

This walks you through a setup wizard and creates `claustro.yaml`. Use `claustro init -y` to accept all defaults.

## 2. Start a sandbox

```bash
claustro up
```

This builds the Docker image (first run only) and starts a sandbox container with your source code mounted at `/workspace`.

## 3. Launch Claude Code

```bash
claustro claude
```

Claude Code starts inside the sandbox with `--dangerously-skip-permissions` — safe because it's in a disposable container.

## 4. Clean up

```bash
claustro burn      # stop and remove container (keeps image)
claustro nuke      # also remove cache volumes
```

## Multiple sandboxes

```bash
claustro up --name api
claustro up --name web
claustro claude api
claustro claude web
claustro ls            # list running sandboxes
```
```

- [ ] **Create `docs/site/content/getting-started/configuration.md`**

```markdown
---
title: Configuration
weight: 3
---

# Configuration

claustro uses a `claustro.yaml` file in your project root. Generate one with `claustro init` or create it manually.

## Minimal example

```yaml
project: my-app
```

## Full example

```yaml
project: my-app

image:
  languages:
    node: true      # always on
    go: true
    rust: false
    python: true
  tools:
    dev: true       # ripgrep, fd, fzf, jq, tree, htop, tmux
    build: true     # gcc, make, pkg-config, libssl-dev
  mcp_servers:
    filesystem: true
    memory: true
    fetch: true

defaults:
  readonly: false
  resources:
    cpus: "4"
    memory: 8G

sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
    env:
      DATABASE_URL: postgresql://localhost:5432/dev

firewall:
  enabled: false
  allow:
    - custom-registry.company.com

git:
  forward_agent: true
  mount_gitconfig: true
  mount_gh_config: true
  mount_ssh_dir: false
```

See [claustro.yaml Reference](../reference/claustro-yaml/) for the full schema.
```

- [ ] **Commit**

```bash
git add docs/site/content/getting-started/
git commit -m "docs: add getting-started pages (installation, quickstart, configuration)"
```

### Step 11.3: Create command reference pages

- [ ] **Create `docs/site/content/commands/_index.md`**

```markdown
---
title: Commands
weight: 2
bookCollapseSection: false
---

# Commands

Complete reference for all claustro CLI commands.
```

- [ ] **Create command pages**

Create one file per command in `docs/site/content/commands/`. Each follows this template pattern. Create the following files:

**`docs/site/content/commands/init.md`**:
```markdown
---
title: claustro init
weight: 1
---

# claustro init

Initialize a new claustro project with an interactive setup wizard.

## Usage

```bash
claustro init [flags]
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--project` | Project name | directory basename |
| `--languages` | Comma-separated: go,rust,python | all enabled |
| `--tools` | Comma-separated: dev,build | all enabled |
| `--mcp` | Comma-separated: filesystem,memory,fetch | all enabled |
| `--cpus` | CPU limit | 4 |
| `--memory` | Memory limit | 8G |
| `--firewall` | Enable egress firewall | false |
| `--readonly` | Mount source read-only | false |
| `-y, --yes` | Accept all defaults | false |

## Examples

```bash
# Interactive wizard
claustro init

# All defaults, no prompts
claustro init -y

# Custom languages and resources
claustro init --languages go,python --cpus 8 --memory 16G
```
```

**`docs/site/content/commands/up.md`**:
```markdown
---
title: claustro up
weight: 2
---

# claustro up

Create and start a sandbox container.

## Usage

```bash
claustro up [flags]
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Sandbox name | auto-generated |
| `--workdir` | Working directory | project root |
| `--mount` | Additional bind mount (repeatable) | none |
| `--env` | Environment variable KEY=VALUE (repeatable) | none |
| `--readonly` | Mount source read-only | false |
| `--firewall` | Enable egress firewall | false |
| `--isolated-state` | Use Docker volume for ~/.claude | false |

## Examples

```bash
claustro up
claustro up --name api --workdir ./services/api
claustro up --mount ./libs:/workspace/libs:ro --firewall
```
```

**`docs/site/content/commands/burn.md`**:
```markdown
---
title: claustro burn
weight: 3
---

# claustro burn

Stop and remove a sandbox container. Preserves the image and cache volumes.

## Usage

```bash
claustro burn [name] [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--name` | Sandbox name |
| `--all` | Burn all sandboxes for the current project |
```

**`docs/site/content/commands/shell.md`**:
```markdown
---
title: claustro shell
weight: 4
---

# claustro shell

Open an interactive shell (zsh) inside a running sandbox.

## Usage

```bash
claustro shell [name] [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--name` | Sandbox name |
```

**`docs/site/content/commands/claude.md`**:
```markdown
---
title: claustro claude
weight: 5
---

# claustro claude

Launch Claude Code inside a sandbox. Automatically starts a sandbox if none is running.

## Usage

```bash
claustro claude [name] [flags] [-- claude-args...]
```

## Flags

| Flag | Description |
|------|-------------|
| `--name` | Sandbox name |

## Examples

```bash
claustro claude
claustro claude api
claustro claude -- --model opus
```
```

**`docs/site/content/commands/config.md`**:
```markdown
---
title: claustro config
weight: 6
---

# claustro config

View and modify claustro configuration.

## Interactive Subcommands

```bash
claustro config languages    # Configure image languages
claustro config tools        # Configure tool groups
claustro config mcp          # Configure MCP servers
claustro config defaults     # Configure resource defaults
claustro config firewall     # Configure firewall settings
claustro config git          # Configure git integration
```

## Get/Set

```bash
claustro config get <path>          # Get a value
claustro config set <path> <value>  # Set a value
```

## Examples

```bash
claustro config get image.languages.go
claustro config set defaults.resources.cpus 8
claustro config set image.languages.rust true
```
```

**`docs/site/content/commands/validate.md`**:
```markdown
---
title: claustro validate
weight: 7
---

# claustro validate

Validate the claustro.yaml configuration file.

## Usage

```bash
claustro validate
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Valid (warnings OK) |
| 1 | Errors found |

## Examples

```bash
$ claustro validate
claustro.yaml: valid

$ claustro validate
claustro.yaml: 1 error(s)
  error: defaults.resources.cpus: invalid CPU value "abc": must be a number
```
```

**`docs/site/content/commands/doctor.md`**:
```markdown
---
title: claustro doctor
weight: 8
---

# claustro doctor

Check host environment health. Validates Docker, git, SSH, clipboard, and configuration.

## Usage

```bash
claustro doctor
```
```

Create stub pages for the remaining commands (`exec`, `ls`, `nuke`, `rebuild`, `status`, `logs`) following the same pattern — each with title, short description, usage, and flags table.

- [ ] **Commit**

```bash
git add docs/site/content/commands/
git commit -m "docs: add command reference pages"
```

### Step 11.4: Create guides and reference pages

- [ ] **Create `docs/site/content/guides/_index.md`**

```markdown
---
title: Guides
weight: 3
bookCollapseSection: false
---

# Guides

In-depth guides for common workflows and advanced configuration.
```

- [ ] **Create `docs/site/content/guides/monorepo.md`**

```markdown
---
title: Monorepo Setup
weight: 1
---

# Monorepo Setup

Run multiple sandboxes targeting different parts of a monorepo.

## Named sandboxes

```yaml
# claustro.yaml
sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
      - ./proto:/workspace/proto:ro
    env:
      DATABASE_URL: postgresql://localhost:5432/dev

  web:
    workdir: ./packages/frontend
    env:
      API_URL: http://localhost:3000
```

## Running

```bash
claustro up --name api
claustro up --name web
claustro claude api
claustro claude web
claustro ls
```
```

- [ ] **Create `docs/site/content/guides/firewall.md`**

```markdown
---
title: Egress Firewall
weight: 2
---

# Egress Firewall

Restrict outbound network access from sandboxes.

## Enable

```bash
claustro up --firewall
```

Or in `claustro.yaml`:

```yaml
firewall:
  enabled: true
  allow:
    - custom-registry.company.com
    - api.openai.com
```

## Default whitelist

When enabled, the following domains are always allowed:
- `api.anthropic.com` — Anthropic API
- `registry.npmjs.org` — npm
- `pypi.org` — Python packages
- `github.com` — Git operations
- `archive.ubuntu.com`, `security.ubuntu.com` — System updates

Docker internal networks (172.16.0.0/12, 192.168.0.0/16, 10.0.0.0/8) are always allowed for compose-sibling services.
```

- [ ] **Create `docs/site/content/guides/mcp-servers.md`**

```markdown
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
```

- [ ] **Create `docs/site/content/guides/custom-image.md`**

```markdown
---
title: Custom Image
weight: 4
---

# Custom Image Configuration

## Language selection

Choose which language runtimes to include. Node.js is always installed (required for Claude Code).

```yaml
image:
  languages:
    node: true      # always on
    go: true
    rust: false
    python: true
```

## Tool groups

```yaml
image:
  tools:
    dev: true       # ripgrep, fd, fzf, jq, tree, htop, tmux
    build: true     # gcc, make, pkg-config, libssl-dev
```

## Extra Dockerfile steps

For project-specific customization:

```yaml
image:
  extra:
    - run: apt-get update && apt-get install -y ffmpeg
    - run: pip install black ruff
```

## Using a completely custom image

```yaml
image: my-registry/my-image:latest
```

The custom image must have Claude Code pre-installed.
```

- [ ] **Create `docs/site/content/reference/_index.md`**

```markdown
---
title: Reference
weight: 4
bookCollapseSection: false
---

# Reference

Detailed reference documentation.
```

- [ ] **Create `docs/site/content/reference/claustro-yaml.md`**

```markdown
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

When set to a string, uses that image directly. When set to a mapping, configures the built image.

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

## mcp

### mcp.stdio

Named stdio MCP servers:

| Field | Type | Description |
|-------|------|-------------|
| `command` | string | Command to run |
| `args` | list | Command arguments |

### mcp.sse

Named SSE MCP servers (run as sibling containers):

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
```

- [ ] **Create `docs/site/content/reference/environment-variables.md`**

```markdown
---
title: Environment Variables
weight: 2
---

# Environment Variables

## Host environment

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Forwarded to sandbox for Claude Code auth |
| `DOCKER_HOST` | Docker daemon connection (if non-default) |

## Sandbox environment

| Variable | Description |
|----------|-------------|
| `HOME` | `/home/sandbox` |
| `PATH` | Includes Go, Rust, npm, pip paths |

## Custom variables

Pass via CLI:

```bash
claustro up --env DATABASE_URL=postgresql://localhost:5432/dev
```

Or in `claustro.yaml`:

```yaml
sandboxes:
  api:
    env:
      DATABASE_URL: postgresql://localhost:5432/dev
```

Or via `.env` file in the project root.
```

- [ ] **Commit**

```bash
git add docs/site/content/guides/ docs/site/content/reference/
git commit -m "docs: add guides and reference pages for Hugo site"
```

---

## Task 12: Hugo Theme Setup

**Files:**
- Modify: `docs/site/hugo.toml` (if theme submodule needed)

### Step 12.1: Add hugo-book theme

- [ ] **Add theme as git submodule**

```bash
cd /workspace
git submodule add https://github.com/alex-shpak/hugo-book docs/site/themes/hugo-book
```

- [ ] **Verify Hugo builds** (requires Hugo installed — skip in CI if not available)

```bash
cd /workspace && hugo version 2>/dev/null && hugo -s docs/site || echo "Hugo not installed — skip local build verification"
```

- [ ] **Commit**

```bash
git add .gitmodules docs/site/themes/hugo-book
git commit -m "chore: add hugo-book theme as git submodule"
```

---

## Task 13: Final Integration Test

### Step 13.1: Run full test suite

- [ ] **Run all tests**

```bash
cd /workspace && go build ./... && go test ./...
```

Expected: all tests PASS, build succeeds.

- [ ] **Verify linting** (if golangci-lint available)

```bash
cd /workspace && golangci-lint run 2>/dev/null || echo "golangci-lint not installed — skip"
```

- [ ] **Commit any remaining fixes**

If any tests or lint issues were found, fix and commit.

---

## Dependency Graph

```
Task 1 (Validation)
  └── Task 2 (Validate cmd + Doctor) — depends on Validate()

Task 3 (ImageBuildConfig)
  └── Task 4 (Templated Dockerfile) — depends on ImageBuildConfig

Task 5 (huh dependency)
  ├── Task 6 (Init wizard) — depends on huh + Task 3
  └── Task 7 (Config command) — depends on huh + Task 3

Task 8 (Command tests) — depends on Tasks 2, 6, 7

Task 9 (GoReleaser) — independent
Task 10 (GitHub Actions) — independent
Task 11 (Hugo docs content) — independent
Task 12 (Hugo theme) — depends on Task 11

Task 13 (Final integration) — depends on all above
```

**Parallelizable groups:**
- Group A: Tasks 1→2 (validation)
- Group B: Tasks 3→4→5→6→7→8 (image config + CLI commands)
- Group C: Tasks 9, 10 (release pipeline)
- Group D: Tasks 11→12 (docs site)
- Final: Task 13
