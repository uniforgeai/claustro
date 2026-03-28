## Why

claustro has a validated POC confirming the Docker + Claude Code approach works. The next step is the minimal Go CLI that completes the core loop — spinning up a sandbox, opening a shell, running Claude Code, and tearing it down — so the tool is actually usable for real work.

## What Changes

- Introduce the Go module and project structure (`go.mod`, `cmd/claustro/`, `internal/`)
- Implement `claustro up` — build image (embedded Dockerfile) if absent, create and start a container with correct bind mounts and the host-path symlink workaround
- Implement `claustro shell` — open an interactive zsh session inside a running sandbox
- Implement `claustro claude` — launch `claude --dangerously-skip-permissions` inside a running sandbox
- Implement `claustro burn` — stop and remove a container, preserve image and volumes
- Implement `claustro ls` — list running sandboxes for the current project
- Implement sandbox identity: derive `{project}_{name}` from CWD directory basename + `--name` flag (default: `default`)
- Apply all POC findings: uid 9999, `~/.claude.json` mount, host-path symlink at startup

## Capabilities

### New Capabilities

- `sandbox-identity`: Derives a unique sandbox key (`{project}_{name}`) and all Docker resource names (container, network, volumes) from the host project path and optional name flag
- `sandbox-lifecycle`: Core container lifecycle — up, shell, claude, burn, ls — via Docker SDK

### Modified Capabilities

- none

## Impact

- New files: `go.mod`, `cmd/claustro/main.go`, `cmd/claustro/up.go`, `cmd/claustro/shell.go`, `cmd/claustro/claude.go`, `cmd/claustro/burn.go`, `cmd/claustro/ls.go`
- New packages: `internal/identity`, `internal/image`, `internal/mount`, `internal/container`
- New asset: embedded `Dockerfile` (based on POC, extended with full polyglot toolchain)
- Dependencies: `github.com/docker/docker`, `github.com/spf13/cobra`, `github.com/spf13/viper`
- No breaking changes (greenfield)
