# Design: Auto-Up on Claude Command

## Approach

Extract the sandbox creation logic from `runUp()` into a reusable function that both `up` and `claude` can call. The `claude` command checks if a sandbox is running; if not, it calls the shared up logic with a quiet flag to suppress verbose output.

## Key Changes

### 1. Extract `ensureRunning()` helper

Create a function in `cmd/claustro/` (e.g., in `up.go` or a new `sandbox.go`) that:
- Takes project identity, Docker client, and a `quiet bool` parameter
- Handles image build, mount assembly, volume creation, container create+start
- When `quiet=true`, prints only a one-line "Starting sandbox..." message
- Returns the container ID and the resolved identity

### 2. Modify `claude.go`

In `runClaude()`, when `container.FindByIdentity` returns nil (no running sandbox):
- Instead of returning `errNotRunning(id)`, call `ensureRunning()` with `quiet=true`
- Then proceed to exec Claude Code in the newly started container

### 3. Modify `up.go`

Refactor `runUp()` to use `ensureRunning()` internally, keeping its current verbose output behavior (`quiet=false`).

## Edge Cases

- **Image not yet built**: Auto-up must build it. Image build output should still stream to stderr so the user knows what's happening on first run.
- **Name collision on auto-generated name**: Same retry logic as current `up` command.
- **Container exists but stopped**: Same as current `up` — treat as needing creation.
