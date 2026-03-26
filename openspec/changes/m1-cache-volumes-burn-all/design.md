## Context

`container.Create` currently builds a `HostConfig` with bind mounts only. Docker named volumes are a separate resource type — they must be created before the container, and they persist independently of the container lifecycle. The spec explicitly distinguishes `burn` (remove container, keep volumes) from `nuke` (remove container and volumes).

The `nuke` command already exists (`cmd/claustro/nuke.go`) and removes the container and network. It needs to be extended to also remove the two cache volumes.

## Goals / Non-Goals

**Goals:**
- Create npm and pip cache volumes on `up`; mount them into the container.
- Preserve volumes on `burn`.
- Destroy volumes on `nuke`.
- `burn --all` stops/removes all containers for the current project without touching volumes.
- `VolumeName` helper on `Identity` for consistent naming.

**Non-Goals:**
- Configurable volume mount paths (hardcode `/home/sandbox/.npm` and `/home/sandbox/.cache/pip` for now; M2 sandbox config can extend this).
- Volume inspection or listing commands (out of scope for M1).
- Cache volumes for other package managers (cargo, go module cache) — npm and pip are the M1 target per spec.

## Decisions

### Volume operations: in `internal/container`

Volume creation and removal belong alongside the other Docker SDK operations in `internal/container`. A dedicated `internal/volume` package would be premature given we have exactly two operations (`EnsureVolume`, `RemoveVolume`).

### `EnsureVolume` semantics

Check if the volume exists by name before creating it. If it exists, do nothing (idempotent). Use the sandbox labels so volumes are discoverable by label filter in the future.

### `burn --all` scope

`--all` is scoped to the **current project** (same as `claustro ls` without `--all`). It uses `container.ListByProject(project, allProjects=false)`. Cross-project burn-all is not in scope.

`--name` and `--all` are mutually exclusive — return an error if both are provided.

### Mount paths

| Volume purpose | Mount path inside container |
|---|---|
| npm | `/home/sandbox/.npm` |
| pip | `/home/sandbox/.cache/pip` |

These are the default cache directories for npm and pip respectively when running as user `sandbox`.

### Volume naming

`VolumeName(purpose string) string` → `claustro-{project}-{name}-{purpose}`

Examples:
- `claustro-myapp-default-npm`
- `claustro-myapp-backend-pip`

This follows the same prefix convention as container (`claustro-{project}-{name}`) and network (`claustro-{project}-{name}-net`).

## Volume lifecycle

```
up    → EnsureVolume(npm) + EnsureVolume(pip) → mount both → ContainerCreate → ContainerStart
burn  → ContainerStop + ContainerRemove  [volumes untouched]
nuke  → ContainerStop + ContainerRemove + RemoveVolume(npm) + RemoveVolume(pip) + RemoveNetwork
```
