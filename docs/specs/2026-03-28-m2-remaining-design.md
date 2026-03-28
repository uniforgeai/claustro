# M2 Remaining Features — Design Spec

> **Status:** Approved
> **Date:** 2026-03-28

## Scope

Two remaining M2 features from the project spec:

1. `--readonly` flag for source mounts
2. `--isolated-state` flag for volume-backed `~/.claude`

Note: `gh` CLI was already added to the Dockerfile — no work needed there.

---

## Feature 1: `--readonly` flag

Mount the project source at `/workspace` as read-only when requested.

### Behavior

- `claustro up --readonly` mounts source at `/workspace` with `:ro`
- Additional `--mount` flags retain their own `:ro`/`:rw` specifiers independently
- `defaults.readonly: true` in `claustro.yaml` sets the default; `--readonly` CLI flag overrides
- When not specified, source remains read-write (current behavior)

### Changes

**`cmd/claustro/up.go`:**
- Add `--readonly` bool flag to `newUpCmd()`
- Pass value into `CLIOverrides`

**`internal/config/resolve.go`:**
- Add `ReadOnly *bool` field to `CLIOverrides`
- In `Resolve()`, when `cli.ReadOnly != nil`, set `sc.ReadOnly = *cli.ReadOnly`

**`internal/mount/mount.go`:**
- Add `readOnly bool` parameter to `Assemble()` signature
- When true, set `ReadOnly: true` on the `/workspace` bind mount

**`cmd/claustro/up.go` (caller):**
- Pass `resolved.ReadOnly` to `mount.Assemble()`

---

## Feature 2: `--isolated-state` flag

Use a named Docker volume for Claude state instead of bind-mounting the host `~/.claude`.

### Behavior

- `claustro up --isolated-state` creates a project-level Docker volume for Claude state
- Volume name: `claustro-{project}-claude-state` (shared across all sandboxes in the project)
- Volume is mounted at `/home/sandbox/.claude` inside the container
- Host `~/.claude` is not bind-mounted and not affected
- Host `~/.claude.json` is also not bind-mounted (host config doesn't apply)
- Plugin host-path remount is skipped (no host `.claude` directory in play)
- Claude auth must be set up fresh inside the container (since host auth isn't forwarded)

### Changes

**`cmd/claustro/up.go`:**
- Add `--isolated-state` bool flag to `newUpCmd()`
- Pass value into `CLIOverrides`
- When true: create volume via `container.EnsureVolume()`, append volume mount, pass flag to `Assemble()`

**`internal/config/resolve.go`:**
- Add `IsolatedState bool` field to `CLIOverrides` and `SandboxConfig`

**`internal/mount/mount.go`:**
- Add `isolatedState bool` parameter to `Assemble()` signature
- When true, skip: `~/.claude` bind mount, `~/.claude.json` bind mount, plugin dir remount

**`internal/identity/identity.go`:**
- Add `ProjectVolumeName(project, purpose) string` that returns `claustro-{project}_{purpose}` (no sandbox name component, so the volume is shared across sandboxes)

---

## Files touched

| File | Changes |
|------|---------|
| `cmd/claustro/up.go` | Two new flags, plumbing to Assemble and volume creation |
| `internal/config/resolve.go` | `CLIOverrides` and `SandboxConfig` fields, override logic |
| `internal/mount/mount.go` | Two new params to `Assemble()`, conditional mount logic |
| `internal/identity/identity.go` | `ProjectVolumeName()` helper |
| Tests for all of the above | |

## Out of scope

- `claustro.yaml` support for `isolated-state` (CLI-only for now)
- Making `--readonly` affect additional mounts (they have their own specifiers)
- Firewall, MCP, distribution (M3/M4)
