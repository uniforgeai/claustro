## Context

The spec defines a rich `claustro.yaml` schema (spec.md, lines 431-473) covering project slug, image, defaults, named sandboxes, firewall, and MCP. The current `Config` struct only knows about `image.extra` and `git.*`. M2 features like multi-sandbox and mount overrides need the full schema parsed and a resolution layer that merges defaults, named sandbox config, and CLI flags into a single flat config.

## Goals

- Parse the complete `claustro.yaml` schema into typed Go structs.
- Provide a `Resolve(name, cliOverrides)` method that produces a flat `SandboxConfig`.
- Load `.env` files and merge them into environment variables.
- Add `--name`, `--workdir`, `--mount`, `--env` CLI flags to `claustro up`.
- Keep the config package self-contained: no Docker SDK dependency, no side effects.

## Non-Goals

- Actually using the resolved config in container creation (follow-up change).
- Firewall or MCP config consumption (M3).
- Config file validation beyond YAML parsing (e.g., path existence checks).
- Config file generation or `init` command.

## Decisions

### 1. Flat SandboxConfig as the resolution output

The `Resolve` method returns a `SandboxConfig` struct with all values filled in — no pointers, no "check default then check sandbox" at call sites. This keeps consumer code simple: read a field, use it.

Resolution order (last wins): spec defaults -> `defaults:` section -> named sandbox section -> CLI flags.

### 2. Keep `git.*` and `image.extra` where they are

The existing `GitConfig` and `ImageConfig` types stay in place. `image` gains a top-level `Name` field (`image: claude-sandbox:latest` shorthand). The `extra` key nests under it as before. This avoids breaking existing code in `up.go` and `rebuild.go`.

### 3. .env loading in config package

`.env` parsing lives in `internal/config` rather than a separate package. It's a simple key=value parser (no shell expansion, no multiline). We use `os.ReadFile` + a small parser — no new dependency. The godotenv library is unnecessary for our subset.

### 4. Mount string format

Mounts in config and CLI use Docker-style `host:container[:mode]` strings. The config package parses these into a `Mount` struct (`HostPath`, `ContainerPath`, `ReadOnly`). Relative host paths are resolved against the project root at resolution time.

### 5. CLI flags on `up` only

`--name` is already used by identity resolution. `--workdir`, `--mount`, `--env` are added to `up` only — other commands that target a running sandbox don't need them (the sandbox already has its config baked in).

## Risks / Trade-offs

- **Schema drift**: The Go structs could diverge from the spec. Mitigation: tests that parse the example config from the spec verbatim.
- **No schema validation**: We parse but don't validate (e.g., `resources.cpus` could be garbage). Acceptable for M2; validation can layer on later.
- **`.env` parser subset**: We don't support shell expansion, multiline values, or `export` prefixes. This covers 95% of real `.env` files. Document the limitations.
