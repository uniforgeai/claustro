## Context

The walking skeleton landed `up`, `burn`, `shell`, `claude`, and `ls`. The internal packages (`container`, `image`, `identity`, `mount`) already provide most of the primitives needed for the remaining M1 commands. This change is purely additive — new thin command files in `cmd/claustro/` and a few new functions in `internal/container` and `internal/image`.

## Goals / Non-Goals

**Goals:**
- Add `nuke`, `rebuild`, `exec`, `status`, `logs` commands following the same patterns as existing commands.
- Add a `Makefile` at the project root for `build`, `run`, `test`, `lint`, and `clean`.
- Keep command files thin — business logic lives in `internal/` packages.

**Non-Goals:**
- Config file support (`sandbox.yaml`) — that is M2.
- Firewall or MCP integration — that is M3.
- Changing any existing command behavior.

## Decisions

### `exec` — reuse `container.Exec(interactive=false)`
`container.Exec` already supports non-interactive mode. The `exec` command just resolves the sandbox identity, finds the container, and calls `container.Exec` with the user-provided args. No new internal code needed.

### `logs` — `cli.ContainerLogs` with SDK options
Docker SDK's `ContainerLogs` accepts `Follow` and `Tail` options natively. The command exposes `--follow`/`-f` and `--tail N` flags and streams output to stdout/stderr. No new internal function needed — the command file calls the SDK directly (it's a one-liner that doesn't warrant an abstraction).

### `status` — `cli.ContainerInspect` for rich details
`ContainerInspect` returns the full container JSON including state, mounts, network settings, and image. The `status` command formats the relevant fields as human-readable output using `text/tabwriter`. A new `container.Inspect` helper is added so the command stays thin.

### `nuke` — loop over `ListByProject` + Stop/Remove + NetworkRemove
`nuke` is `burn` applied to all sandboxes in a project (or all claustro sandboxes with `--all`). It reuses `ListByProject`, then for each container calls `Stop`+`Remove`, then removes the per-sandbox network via `cli.NetworkRemove`. A new `container.RemoveNetwork` helper is added.

### `rebuild` — force `image.EnsureBuilt` + optional restart
`image.EnsureBuilt` currently skips the build if the image exists. A new `image.Build` function (no skip check) forces a rebuild. `rebuild` calls `image.Build` unconditionally. An optional `--restart` flag (default off) stops all project sandboxes, rebuilds, and restarts them.

### Makefile — standard phony targets
A minimal `Makefile` at the project root with: `build`, `run` (calls `go run ./cmd/claustro`), `test`, `lint`, and `clean`. Cross-compilation targets are out of scope for M1 (those belong in M4/CI).

## Risks / Trade-offs

- **`nuke --all` is destructive** → Requires no additional confirmation prompt for now (M1 is a dev tool); a `--force` flag can be added in M2 if needed.
- **`rebuild --restart` restart ordering** → Restart is best-effort (stop all, rebuild, start all). Partial failures leave some containers stopped. Acceptable for a dev tool; no rollback.
- **`logs` multiplexed stream** → Docker multiplexes stdout/stderr in a custom binary framing when TTY is not allocated. `stdcopy.StdCopy` from the Docker SDK must be used to demultiplex correctly, otherwise output is garbled.
