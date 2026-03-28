# Design: Doctor Command

## Architecture

### `internal/doctor` package

```go
// CheckResult represents the outcome of a single health check.
type CheckResult struct {
    Name    string       // e.g. "Docker Engine"
    Status  CheckStatus  // Pass, Fail, Warn, Skip
    Detail  string       // e.g. "Docker 27.5.1, API 1.47"
    FixHint string       // e.g. "Install Docker: https://docs.docker.com/get-docker/"
}

type CheckStatus int
const (
    Pass CheckStatus = iota
    Warn
    Fail
    Skip
)
```

### Check functions

Each check is a standalone function with signature:

```go
func CheckDocker(ctx context.Context) CheckResult
func CheckDockerSocket() CheckResult
func CheckBaseImage(ctx context.Context, cli client.APIClient) CheckResult
func CheckGitConfig() CheckResult
func CheckSSHAgent() CheckResult
func CheckGitHubCLI() CheckResult
func CheckClipboard() CheckResult
func CheckConfigFile(dir string) CheckResult
```

The GitHub CLI check is the one exception where we shell out — `gh` is an external tool, not something we control via SDK. This is acceptable because we're inspecting the user's host environment, not managing containers.

### `cmd/claustro/doctor.go`

Thin command that:
1. Creates Docker client (if possible)
2. Runs each check in order
3. Formats and prints results with ✓/✗/⚠ indicators
4. Returns exit code 1 if any check has `Fail` status

### Check ordering

Checks run sequentially because some depend on earlier results:
1. Docker Engine (if fail, skip Docker Socket and Base Image)
2. Docker Socket
3. Base Image
4. Git Config
5. SSH Agent
6. GitHub CLI
7. Clipboard
8. Config File (info-only, never fails)

### Output formatting

- Green ✓ for Pass
- Yellow ⚠ for Warn
- Red ✗ for Fail
- Gray - for Skip
- Summary line at the end: "N/M checks passed. X issues found."
- Color output respects `NO_COLOR` env var and terminal detection

### Platform differences

- **Clipboard check**: On macOS, check for `pbpaste`. On Linux, check for `xclip` or `wl-paste` and `DISPLAY`/`WAYLAND_DISPLAY`.
- **SSH agent**: Same on all platforms — check `SSH_AUTH_SOCK` and `ssh-add -l`.
- **Docker socket**: `/var/run/docker.sock` on Linux, may differ on macOS with OrbStack/Docker Desktop.
