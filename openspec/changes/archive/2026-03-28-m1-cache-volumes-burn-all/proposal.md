## Why

Two gaps remain before M1 is fully closed:

1. **Cache volumes are missing**: The spec states that `burn` preserves "cache volumes (npm, pip)" and `nuke` removes them. Currently no cache volumes are created or mounted — every sandbox starts with a cold npm/pip cache, making rebuilds and tool installs inside the container slow.

2. **`burn --all` is missing**: The spec requires `claustro burn --all` to stop and remove all sandboxes for the current project. Today `burn` only targets a single named sandbox. Users running multiple sandboxes must burn them one by one.

## What Changes

### Cache volumes

Two named Docker volumes are created per sandbox during `up`:
- `claustro-{project}-{name}-npm` — mounted at `/home/sandbox/.npm`
- `claustro-{project}-{name}-pip` — mounted at `/home/sandbox/.cache/pip`

Volume names follow the spec pattern: `claude-sandbox-{project}_{name}_{purpose}` (adapted for claustro prefix). These volumes persist across `burn` operations and are only removed by `nuke`.

### `burn --all`

`claustro burn --all` stops and removes every sandbox container for the current project. It does NOT remove cache volumes (that is `nuke` behavior). The `--name` and `--all` flags are mutually exclusive.

## Capabilities

### Modified Capabilities

- `sandbox-lifecycle`: `up` now creates two cache volumes and mounts them. `burn` adds `--all` to target all project sandboxes.
- `sandbox-identity`: `Identity` gains a `VolumeName(purpose string) string` helper returning `claustro-{project}-{name}-{purpose}`.

## Milestone

M1 — closes the final gaps.

## Impact

- `internal/identity/identity.go`: Add `VolumeName(purpose string) string`.
- `internal/identity/identity_test.go`: Add tests for `VolumeName`.
- `internal/container/container.go`: Add `EnsureVolume(ctx, cli, name string, labels map[string]string) error` and `RemoveVolume(ctx, cli, name string) error`.
- `cmd/claustro/up.go`: Call `EnsureVolume` for npm and pip volumes; include them in `container.Create` mounts.
- `cmd/claustro/burn.go`: Add `--all` flag; when set, call `ListByProject` and loop Stop+Remove per container.
- `cmd/claustro/nuke.go`: Call `RemoveVolume` for npm and pip volumes after removing the container.
- No new external Go dependencies.
