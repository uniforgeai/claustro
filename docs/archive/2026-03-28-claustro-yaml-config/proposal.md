## Why

M2 features (multi-sandbox, `--workdir`, `--mount`, `.env`) all need a richer config model. The current `Config` struct only carries `image.extra` and `git.*` fields. Before we can build named sandboxes or mount overrides, the config package must parse the full `claustro.yaml` schema defined in the spec and expose it to the rest of the codebase.

This change ships the config infrastructure first so every subsequent M2 change can consume it without touching the config layer.

## What Changes

- Expand `internal/config.Config` to match the full spec schema: `project`, `image` (top-level string + `extra`), `defaults` (firewall, readonly, resources), `sandboxes` (per-name workdir, mounts, env), `firewall`, `mcp`.
- Add `Resolve(name string)` method that merges a named sandbox config over defaults + CLI overrides, returning a flat `SandboxConfig` ready for container creation.
- Add `.env` file loading: read `<projectPath>/.env` alongside `claustro.yaml`, merge into env map (per-sandbox env wins over `.env` values).
- Wire `--name`, `--workdir`, `--mount`, `--env` flags on `up` command (flag parsing only; container creation changes are a separate change).
- Preserve backward compatibility: projects without `claustro.yaml` still get sensible zero-value defaults.

## Capabilities

### New Capabilities

- `config-full-schema`: Parse and validate the complete `claustro.yaml` structure.
- `config-resolve`: Merge defaults + named sandbox + CLI overrides into a single resolved config.
- `dotenv-loading`: Load `.env` file from project root into sandbox environment.
- `cli-sandbox-flags`: `--name`, `--workdir`, `--mount`, `--env` flags on `claustro up`.

### Modified Capabilities

- `project-config`: `Config` struct expanded; existing `Image.Extra` and `Git.*` fields unchanged.

## Milestone

M2 — foundation change.

## Impact

- `internal/config/config.go`: Expand struct, add `Resolve()`, add `.env` loader.
- `internal/config/config_test.go`: Tests for full schema parsing, resolution, `.env` loading.
- `cmd/claustro/up.go`: Add `--name`, `--workdir`, `--mount`, `--env` flags; pass resolved config downstream.
- No changes to `internal/container`, `internal/image`, or `internal/network` (those consume the resolved config in follow-up changes).
