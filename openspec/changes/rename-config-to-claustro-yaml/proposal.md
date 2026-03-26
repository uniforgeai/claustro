## Why

The project config file is currently named `sandbox.yaml`, carried over from the early design phase before the tool was named "claustro". Using `sandbox.yaml` is ambiguous — other tools in the ecosystem (Pulumi, various CI systems) use files with similar names. `claustro.yaml` is unambiguous, consistent with the binary name, and makes the file instantly recognizable as belonging to claustro.

## What Changes

A pure filename rename: `sandbox.yaml` → `claustro.yaml`.

The `Config` struct, all loading logic, all behavioral semantics, and the sandbox.yaml schema remain identical. Only the filename string changes.

## Capabilities

### Modified Capabilities

- `project-config`: Config file is now discovered as `claustro.yaml` instead of `sandbox.yaml`.

## Milestone

M2 prerequisite — rename before M2 expands the config schema so users learn the correct filename from the start.

## Impact

- `internal/config/config.go`: Change `"sandbox.yaml"` → `"claustro.yaml"` in `Load()`.
- `internal/config/config_test.go`: Update all `os.WriteFile(..., "sandbox.yaml", ...)` calls to `"claustro.yaml"`.
- `openspec/specs/spec.md`: Update all references to `sandbox.yaml` → `claustro.yaml`.
- Any comments or documentation referencing `sandbox.yaml`.
- No behavioral change. No new dependencies.
