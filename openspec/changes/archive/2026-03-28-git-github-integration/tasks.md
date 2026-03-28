## 1. `GitConfig` struct in `internal/config`

- [x] 1.1 Add `GitConfig` struct to `internal/config/config.go` with fields: `ForwardAgent *bool`, `MountGitconfig *bool`, `MountGhConfig *bool`, `MountSSHDir *bool`
- [x] 1.2 Add `Config.Git GitConfig` field with yaml tag `git`
- [x] 1.3 Add helper methods with default-on semantics: `IsForwardAgent() bool`, `IsMountGitconfig() bool`, `IsMountGhConfig() bool`, `IsMountSSHDir() bool` — return `true` when field is nil (not set), except `IsMountSSHDir` which returns `false` when nil
- [x] 1.4 Add unit tests for the helper methods (nil=default, explicit true, explicit false)

## 2. Git mounts in `internal/mount`

- [x] 2.1 Update `Assemble(hostProjectPath string)` signature to `Assemble(hostProjectPath string, git *config.GitConfig) ([]mount.Mount, error)`
- [x] 2.2 Conditionally mount `~/.gitconfig` → `/home/sandbox/.gitconfig` (read-only, if file exists and `git.IsMountGitconfig()`)
- [x] 2.3 Conditionally mount `~/.config/gh/` → `/home/sandbox/.config/gh/` (read-write, if dir exists and `git.IsMountGhConfig()`)
- [x] 2.4 Conditionally mount SSH agent socket: if `git.IsForwardAgent()` and `$SSH_AUTH_SOCK` is set, bind-mount the socket path
- [x] 2.5 Conditionally mount `~/.ssh/` → `/home/sandbox/.ssh/` (read-only, if dir exists and `git.IsMountSSHDir()`)
- [x] 2.6 Update `internal/mount/mount_test.go` with tests for each conditional mount
- [x] 2.7 Update `cmd/claustro/up.go` to pass `cfg.Git` to `mount.Assemble`

## 3. SSH_AUTH_SOCK forwarding in `internal/container`

- [x] 3.1 In `container.Create`, when `SSH_AUTH_SOCK` is set in the host environment, append `SSH_AUTH_SOCK=<value>` to the container's `Env` slice

## 4. `gh` CLI in Dockerfile

- [x] 4.1 Add `gh` CLI installation to `internal/image/Dockerfile` using the official GitHub CLI apt repository

## 5. Verify

- [x] 5.1 `go build ./...` passes with no errors
- [x] 5.2 `go test ./...` passes
- [x] 5.3 `golangci-lint run` passes with no new warnings
