## Context

`identity.FromCWD(name string)` is the single entry point for sandbox identity resolution. The `fromPath` helper sets `name = "default"` when `name == ""`. All downstream resource names (`ContainerName()`, `NetworkName()`, labels) derive from `Identity.Name` — no changes needed there.

For commands that target an existing sandbox, the current pattern is: resolve identity from `--name` flag, then call `container.FindByIdentity`. With random names, users won't always know the name — so targeting commands need an auto-select path.

## Goals / Non-Goals

**Goals:**
- Random `adjective_noun` names when `--name` is omitted on `up`.
- Auto-select the single running sandbox for targeting commands.
- Helpful error listing available names when multiple sandboxes exist.
- No external dependencies.

**Non-Goals:**
- Uniqueness guarantees (collision probability with 2500 combinations is acceptable for a dev tool).
- Configurable word lists or custom formats.
- Persisting the name anywhere beyond Docker labels (always recoverable via `claustro ls`).

## Decisions

### Generator location: `internal/identity/names.go`

All naming logic lives in `internal/identity`. A separate package would be premature — only `identity.FromCWD` calls the generator.

### Word lists

Hardcoded `var` slices — no `go:embed`, no files. ~50 adjectives + ~50 nouns.

**Adjectives:** amber, bold, bright, calm, cold, crisp, dark, deep, dense, dry, dusk, epic, faint, firm, fleet, free, fresh, frosted, gilded, glad, grand, grey, grim, hazy, icy, jade, jade, keen, lush, misty, noble, pale, prime, proud, quick, rare, rich, royal, sharp, silent, slim, slow, smoky, soft, stark, still, stoic, sunny, swift, teal, warm, wild, wise

**Nouns:** bear, brook, canyon, cedar, cliff, cloud, coast, coral, crane, creek, dune, eagle, elm, falcon, fern, fjord, fox, gale, glacier, hawk, heron, iris, isle, kelp, lake, lark, lynx, maple, marsh, mesa, mist, moon, moss, otter, owl, panda, peak, pine, raven, reed, ridge, river, robin, sage, seal, shore, sparrow, storm, swan, tide, vale, wave, wren

### Auto-select helper: `cmd/claustro/resolve.go`

A shared `resolveName(ctx, cli, projectSlug, name string) (string, error)` function:
1. If `name != ""` → return `name` (no change).
2. Call `container.ListByProject(ctx, cli, projectSlug, false)`.
3. 0 results → `"no sandboxes running for this project — run: claustro up"`.
4. 1 result → return `c.Labels["claustro.name"]`.
5. 2+ results → error listing names: `"multiple sandboxes running — specify --name:\n  ..."`.

This helper is used by shell, claude, exec, burn (single-target path), status, logs.

### Collision handling on `up`

After generating a name, `up` checks `container.FindByIdentity`. If a container with that name already exists for the project, generate a new name (up to 5 retries, then error). In practice, with 2500 combinations, collision requires intentional effort.

### `up` output change

Before: `Sandbox started: claustro-myapp-default`
After:
```
Sandbox started: claustro-myapp-happy-panda
  Name: happy_panda  (use --name happy_panda to target it)
  Run: claustro shell --name happy_panda
  Run: claustro claude --name happy_panda
```

When auto-select is active (single sandbox), the targeting commands omit `--name` from their hint since auto-select will handle it.
