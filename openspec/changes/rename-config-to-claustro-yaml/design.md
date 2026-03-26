## Context

`internal/config/config.go` contains a single hardcoded filename string: `"claustro.yaml"` (line 30 in `Load()`). Error messages also mention the filename. Tests in `config_test.go` create temporary files named `claustro.yaml`. The spec document references `claustro.yaml` throughout.

This change touches no logic — only string literals and documentation.

## Goals / Non-Goals

**Goals:**
- Rename the config filename from `claustro.yaml` to `claustro.yaml` everywhere.
- No behavioral change whatsoever.

**Non-Goals:**
- Backwards compatibility shim (no fallback to `claustro.yaml` if `claustro.yaml` not found — clean break).
- Schema changes to the config format.

## Decisions

### No fallback / migration

Since this is early-stage software with no public users yet, there is no need to support both filenames. `Load()` looks for `claustro.yaml` only. Any existing `claustro.yaml` file in a project is simply ignored until the user renames it.

### All references updated atomically

Spec doc, source code, and tests are updated in the same commit so there is no intermediate state where docs and code disagree.

## Files to Update

| File | Change |
|---|---|
| `internal/config/config.go` | `"claustro.yaml"` → `"claustro.yaml"` in `Load()` and error messages |
| `internal/config/config_test.go` | All `"claustro.yaml"` filename strings → `"claustro.yaml"` |
| `openspec/specs/spec.md` | All `claustro.yaml` references → `claustro.yaml` |
| `openspec/changes/image-customization/proposal.md` | Reference to `claustro.yaml` → `claustro.yaml` |
| `openspec/changes/image-customization/tasks.md` | Reference to `claustro.yaml` → `claustro.yaml` |
| `openspec/changes/git-github-integration/proposal.md` | Reference to `claustro.yaml` → `claustro.yaml` |
