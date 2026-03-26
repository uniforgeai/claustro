## Why

The walking skeleton delivered the core sandbox lifecycle commands (`up`, `burn`, `shell`, `claude`, `ls`), but M1 is incomplete: `nuke`, `rebuild`, `exec`, `status`, and `logs` are missing, and there is no Makefile for fast local development iteration.

## What Changes

- **`nuke` command**: Stop and remove all sandboxes for a project (or all claustro sandboxes with `--all`), including their networks.
- **`rebuild` command**: Remove and rebuild the `claustro:latest` Docker image, optionally restarting affected sandboxes.
- **`exec` command**: Run a one-off command inside a running sandbox and return its output (non-interactive, unlike `shell`).
- **`status` command**: Show detailed runtime status for a named sandbox (state, mounts, network, image, uptime).
- **`logs` command**: Stream or tail container logs from a running or stopped sandbox, with `--follow` and `--tail` flags.
- **`Makefile`**: Developer convenience targets — `build`, `run`, `test`, `lint`, `clean`, and a `dev` target for quick local iteration.

## Capabilities

### New Capabilities

- `dev-tooling`: Makefile with build, run, test, lint, and clean targets for local development.

### Modified Capabilities

- `sandbox-lifecycle`: Adding the five missing M1 commands (`nuke`, `rebuild`, `exec`, `status`, `logs`) to the existing lifecycle spec.

## Impact

- `cmd/claustro/`: Five new command files (`nuke.go`, `rebuild.go`, `exec.go`, `status.go`, `logs.go`).
- `internal/container/`: Extended with `Nuke`, `Logs`, `Status` operations.
- `internal/image/`: Extended with `Rebuild` operation.
- `Makefile` added at project root.
- No new external dependencies required.
