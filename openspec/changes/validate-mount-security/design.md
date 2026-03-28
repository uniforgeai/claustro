# Design: Validate Mount Security

## Architecture

### `internal/validate` package

```go
// Severity levels for validation findings.
type Severity int

const (
    SeverityOK   Severity = iota
    SeverityInfo
    SeverityWarn
    SeverityError
    SeverityBlock // hard constraint from spec — cannot proceed
)

// Finding represents a single validation result.
type Finding struct {
    Category string   // "mount" or "container"
    Name     string   // e.g. "Docker socket", "Home directory"
    Severity Severity
    Message  string   // human-readable explanation
    FixHint  string   // suggested fix
}

// Report holds all findings from a validation run.
type Report struct {
    Findings []Finding
}

func (r *Report) HasErrors() bool   // any Error or Block findings
func (r *Report) HasWarnings() bool
func (r *Report) Summary() string   // "5/7 checks passed. 1 blocked, 1 warning."
```

### Rule engine

Rules are plain functions, not a plugin system. Each rule takes the resolved mount list and returns findings:

```go
// MountRule checks a single mount entry.
type MountRule func(m mount.Mount) *Finding

// ContainerRule checks container-level security settings.
type ContainerRule func(cfg ContainerSecurityConfig) *Finding

// ContainerSecurityConfig captures the security-relevant container settings.
type ContainerSecurityConfig struct {
    SecurityOpts []string
    Privileged   bool
    CapAdd       []string
    User         string
    CPULimit     int64
    MemoryLimit  int64
}
```

### Built-in mount rules

```go
// Blocked (spec-mandated)
func RuleNoDockerSocket(m mount.Mount) *Finding
func RuleNoEtc(m mount.Mount) *Finding
func RuleNoProcSys(m mount.Mount) *Finding
func RuleNoRootFS(m mount.Mount) *Finding

// Errors (strongly discouraged)
func RuleNoHomeDir(m mount.Mount) *Finding
func RuleNoVar(m mount.Mount) *Finding
func RuleNoCredentialPaths(m mount.Mount) *Finding
func RuleNoTmp(m mount.Mount) *Finding

// Warnings
func RuleSSHReadOnly(m mount.Mount) *Finding
func RuleBroadMount(m mount.Mount) *Finding
func RulePreferReadOnly(m mount.Mount) *Finding
```

### Built-in container rules

```go
func RuleNoNewPrivileges(cfg ContainerSecurityConfig) *Finding
func RuleNotPrivileged(cfg ContainerSecurityConfig) *Finding
func RuleNoCapAdd(cfg ContainerSecurityConfig) *Finding
func RuleResourceLimits(cfg ContainerSecurityConfig) *Finding
func RuleNonRootUser(cfg ContainerSecurityConfig) *Finding
```

### Validation orchestrator

```go
// ValidateMounts runs all mount rules against the provided mount list.
func ValidateMounts(mounts []mount.Mount, homeDir string) *Report

// ValidateContainer runs all container security rules.
func ValidateContainer(cfg ContainerSecurityConfig) *Report

// ValidateAll runs both mount and container validation.
func ValidateAll(mounts []mount.Mount, homeDir string, cfg ContainerSecurityConfig) *Report
```

### Pre-flight vs live inspection

**Pre-flight** (default): Uses `internal/config` to resolve the full configuration and `internal/mount` to assemble the mount list — same logic that `up` uses. No running container required.

**Live inspection** (`--live`): Uses the Docker SDK to inspect a running container (`client.ContainerInspect`), extracts the actual mounts and security settings from the container JSON, and validates those. This catches any drift between config and reality.

```go
// FromContainerInspect extracts validation inputs from a running container.
func FromContainerInspect(info types.ContainerJSON) ([]mount.Mount, ContainerSecurityConfig)
```

### Credential path list

Known credential paths checked by `RuleNoCredentialPaths`:

```go
var credentialPaths = []string{
    ".aws",
    ".kube",
    ".config/gcloud",
    ".azure",
    ".vault-token",
    ".docker/config.json",
    ".npmrc",          // can contain auth tokens
    ".pypirc",         // can contain auth tokens
    ".env",
    ".netrc",
}
```

These are matched against the source path of each mount. If the source path ends with or contains any of these, it's flagged.

### `cmd/claustro/validate.go`

Thin command that:
1. Loads configuration (config file + flags)
2. Resolves the mount list using existing `internal/mount.Assemble` logic
3. If `--live`: inspects the running container via Docker SDK
4. Calls `validate.ValidateAll`
5. Prints the report with colored indicators (respects `NO_COLOR`)
6. Exits with code 1 if any `Block` or `Error` findings, 0 otherwise

### Output formatting

Same style as `doctor` command for consistency:
- `✓` green for OK
- `⚠` yellow for Warn
- `✗` red for Error and Block
- Summary line at end
- `NO_COLOR` and terminal detection respected

### Integration with `up` command (future)

Not in this change, but the validate package is designed so that `up` can call `ValidateMounts` as a pre-flight check and refuse to start if any `Block`-level findings exist. This would be a one-line integration in `up.go`.
