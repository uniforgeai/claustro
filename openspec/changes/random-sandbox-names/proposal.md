## Why

Every sandbox currently gets the name `default` when `--name` is not provided. This creates three problems:

1. **No distinctiveness**: All default sandboxes look identical in `claustro ls` output. When a user has burned and recreated sandboxes multiple times, there is no way to tell them apart at a glance.
2. **Multi-sandbox friction**: The `default` name implies there is only one sandbox per project. Users are subtly discouraged from running multiple named sandboxes because the default always clobbers itself.
3. **Unmemorable container names**: `claustro-myproject-default` carries no personality. Docker solved this years ago with generated names like `festive_darwin` — users remember them.

## What Changes

### Random name generation

When `claustro up` is run without `--name`, a random `adjective_noun` name is generated (e.g., `happy_panda`, `swift_falcon`, `calm_river`). The name is printed prominently so the user can use it in subsequent commands.

The `--name` flag on all commands still overrides.

### Auto-select for targeting commands

For commands that target an existing sandbox (`shell`, `claude`, `exec`, `burn`, `status`, `logs`): if `--name` is not provided and exactly one sandbox is running for the project, that sandbox is auto-selected. If multiple exist, the command requires `--name` and lists available names.

### Implementation

- Name generator lives in `internal/identity` — pure function, hardcoded word lists, no external deps.
- ~50 adjectives (colors, emotions, natural phenomena) × ~50 nouns (animals, nature elements) = 2500 combinations.
- `identity.FromCWD("")` now calls `RandomName()` instead of defaulting to `"default"`.
- A shared `resolveName` helper in `cmd/claustro/` handles auto-select logic for targeting commands.

## Capabilities

### Modified Capabilities

- `sandbox-identity`: Empty name now produces a random `adjective_noun` string instead of `"default"`.
- `sandbox-lifecycle`: `up` prints the generated name; targeting commands auto-select when unambiguous.

## Milestone

M1/M2 boundary — logically M1 behavior but depends on no prior work; implement after M1 gaps are closed.

## Impact

- `internal/identity/names.go`: New file with `RandomName()` and word lists.
- `internal/identity/identity.go`: Replace `"default"` fallback with `RandomName()`.
- `internal/identity/identity_test.go`: Update tests for new name format.
- `cmd/claustro/up.go`: Print generated name prominently.
- `cmd/claustro/resolve.go`: New shared helper `resolveName(ctx, cli, name string) (string, error)`.
- `cmd/claustro/shell.go`, `claude.go`, `exec.go`, `burn.go`, `status.go`, `logs.go`: Use `resolveName`.
- No new external dependencies.
