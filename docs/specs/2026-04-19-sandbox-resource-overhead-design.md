# Sandbox Resource Overhead Reduction — Design

> **Status:** Draft
> **Date:** 2026-04-19
> **Author:** Peter (Uniforge GmbH)

## Overview

Reduce the perceptible CPU and memory overhead of running claustro sandboxes on macOS via OrbStack, particularly with 2–3 sandboxes running in parallel. Two complementary mechanisms ship together: **host-aware resource defaults** (lower per-sandbox cap when no explicit cap is set) and **idle auto-pause via a background daemon** (`claustrod`) that pauses sandboxes after 5 minutes with no active exec session and resumes them transparently on next attach.

A third item — swapping the base image from `ubuntu:24.04` to `debian:12-slim` — is a **standalone research task**, not part of this implementation spec. Results are documented in `docs/research/2026-04-19-debian-slim-vs-ubuntu.md`. If the swap shows >30% size reduction with no functional regression, it gets its own spec/PR later.

## Motivation

Current defaults are 4 CPU and 8 GiB per sandbox. With 2–3 sandboxes running, that reserves 8–12 CPU and 16–24 GiB — too much on a typical 8–10 core / 16–32 GiB Mac. Active iteration in one sandbox saturates the CPU cap, fans spin, host responsiveness drops. Idle sandboxes keep their share of host CPU/RAM available to the OrbStack VM even when the user is not working in them.

The goal is to reduce both the **per-sandbox ceiling** (so one active sandbox does not dominate the host) and the **multiplier from parallel sandboxes** (so the inactive ones cost effectively nothing).

## Non-Goals

- Image slimming (Approach 2 of the brainstorm) beyond the standalone research task. The other items (opt-in runtimes, disable Claude Code background updater, drop gopls by default) are deferred.
- In-container nicing / MCP server lazy-start (Approach 3 of the brainstorm). Tracked as future work if needed.
- Changing existing users' explicit `cpus` / `memory` values in `claustro.yaml`. Smart defaults apply only when the field is empty.
- macOS launchd / Linux systemd integration for the daemon. The daemon is launched on demand by `claustro up` and exits when no claustro containers remain.
- Persistent state across daemon restarts (idle timers reset on restart — acceptable since a restart only happens after `claustro up` and the next 30 s tick re-baselines).

## Design

### 1. Host-Aware Resource Defaults

#### Detection

New package `internal/sysinfo/`:

```go
type Host struct {
    CPUs        int   // runtime.NumCPU()
    MemoryBytes int64 // sysctl hw.memsize on Darwin; /proc/meminfo on Linux
}

func Detect() (*Host, error)
```

- `runtime.NumCPU()` returns the cores macOS sees (claustro runs natively on the Mac, not inside the VM), so this is the right number to budget against.
- Memory: `sysctl -n hw.memsize` on Darwin via `os/exec`; read `MemTotal` from `/proc/meminfo` on Linux. No third-party dependency.
- On error (extremely rare, e.g. sysctl missing), `Detect` returns a default `Host{CPUs: 4, MemoryBytes: 8 GiB}` and a sentinel error the caller can log-and-ignore. Callers always get a usable `*Host`.

#### Computation

```go
// Internal to internal/container/container.go (or a new sub-file).
func smartCPUs(h *sysinfo.Host) int64 {  // returns nano-CPUs
    cores := h.CPUs / 4
    if cores < 2 {
        cores = 2
    }
    return int64(cores) * nanosecondsPerCPU
}

func smartMemory(h *sysinfo.Host) int64 {  // returns bytes
    quarter := h.MemoryBytes / 4
    eight := int64(8) * 1024 * 1024 * 1024
    if quarter < eight {
        return quarter
    }
    return eight
}
```

Examples (host → smart cap):

| Host          | Cores | Memory | Smart CPU | Smart Memory |
|---------------|-------|--------|-----------|--------------|
| M1 8 GiB      | 8     | 8 GiB  | 2         | 2 GiB        |
| M1 Pro 16 GiB | 10    | 16 GiB | 2         | 4 GiB        |
| M2 Pro 32 GiB | 12    | 32 GiB | 3         | 8 GiB        |
| M3 Max 64 GiB | 16    | 64 GiB | 4         | 8 GiB        |

#### Wiring

In `internal/container/container.go`, replace the bare `defaultNanoCPUs` / `defaultMemory` constants. `parseNanoCPUs("")` and `parseMemory("")` route through `smartCPUs` / `smartMemory` using a `*sysinfo.Host` injected via `CreateOptions` (so the function stays testable). The host is detected once in `cmd/claustro/up.go` and passed down via `CreateOptions.Host`.

Surface in `claustro up` output when defaults applied:

```
Sandbox started: claustro-myapp_calm_river
  Resources: 2 CPU / 4 GiB (host-aware default; override in claustro.yaml)
  Run: claustro shell --name calm_river
  ...
```

### 2. Idle Auto-Pause Daemon (`claustrod`)

#### Binary and package layout

- New binary: `cmd/claustrod/main.go` (~80 LoC). Single subcommand: `claustrod run`. Reads no flags besides the standard `--log-level`.
- New package: `internal/daemon/`:
  - `daemon.go` — main poll loop, signal handling, pidfile management.
  - `decide.go` — pure decision function `Decide(state map[string]Track, containers []ContainerView, now time.Time, timeout time.Duration) (toPause []string)` for unit testing.
  - `state.go` — in-memory `map[containerID]Track{lastActive time.Time}`. No persistence.
  - `launch.go` — `EnsureRunning() error` — checks `~/.claustro/claustrod.pid`, sends signal 0 to verify the process is alive, fork-execs a detached `claustrod run` if not.

#### Loop

Every 30 seconds:

1. List all containers with label `claustro.project=*` via `cli.ContainerList`.
2. If zero, write `claustrod: no claustro containers, exiting` to log, remove the pidfile, exit 0.
3. For each running container:
   - If `c.State == "paused"`, skip (nothing to do).
   - Inspect for exec IDs: `cli.ContainerInspect(c.ID).ExecIDs`.
   - For each exec ID, `cli.ContainerExecInspect(execID).Running` — count the running ones.
   - If running > 0, `state[c.ID].lastActive = now` and continue.
   - Else if `c.ID` is newly observed (not in `state`), `state[c.ID].lastActive = now` (grace period — never pause on first sighting) and continue.
   - Else if `now - state[c.ID].lastActive >= timeout`, append `c.ID` to `toPause`.
4. Read each project's `pause.idle_timeout` and `pause.enabled` from its `claustro.yaml` (host-resolved via the container's `claustro.project_path` label).
   - If `pause.enabled == false` for that project, skip the container.
   - The effective timeout is `PauseConfig.Timeout()` for that project — defined to be `IdleTimeout` when set, else the built-in default of 5 minutes. There is no separate global override; per-project config is the single source of truth.
5. For each `c.ID` in `toPause`: `cli.ContainerPause(c.ID)` and `cli.ContainerPause(siblingID)` for every MCP SSE sibling identified by label `claustro.parent=<c.ID>`.

Per-poll work is bounded: `O(containers + execs)`, both small numbers.

#### Pidfile

`~/.claustro/claustrod.pid` (created with `0600`). Contains decimal PID. On startup the daemon takes an exclusive `flock` on the file. If `flock` fails, another instance is alive — exit immediately. On normal exit, remove the file.

#### Logging

Daemon writes to `~/.claustro/claustrod.log` (rotated by simple size cap, 1 MiB → rotate to `.log.1`, drop `.log.2`). Levels: info for pause/unpause, warn for transient errors, error for terminal conditions.

### 3. Auto-Resume on Attach

In `cmd/claustro/agent.go`, `cmd/claustro/shell.go`, and `cmd/claustro/exec.go`, after `container.FindByIdentity` returns `c`:

```go
inspect, err := cli.ContainerInspect(ctx, c.ID)
if err != nil {
    return fmt.Errorf("inspecting container: %w", err)
}
if inspect.State.Paused {
    if err := container.Unpause(ctx, cli, c.ID); err != nil {
        return fmt.Errorf("unpausing container: %w", err)
    }
    for _, sib := range mcpSiblings(ctx, cli, c.ID) {
        _ = container.Unpause(ctx, cli, sib.ID) // best-effort
    }
}
```

`container.Unpause` is a one-line wrapper around `cli.ContainerUnpause`. Same for `container.Pause` (used by the daemon).

The MCP sibling discovery already exists in `internal/mcp/sse.go` (`StopSSESiblings` lists by parent ID); a new `ListSSESiblings` returning `[]containertypes.Summary` is extracted from that path.

### 4. Configuration

Extend `internal/config/config.go`:

```yaml
# claustro.yaml
pause:
  enabled: true        # default; false opts this project out of auto-pause
  idle_timeout: 5m     # default
```

`PauseConfig` struct:

```go
type PauseConfig struct {
    Enabled     *bool          `yaml:"enabled"`      // nil = true
    IdleTimeout time.Duration  `yaml:"idle_timeout"` // 0 = 5m
}

func (p PauseConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }
func (p PauseConfig) Timeout() time.Duration {
    if p.IdleTimeout == 0 { return 5 * time.Minute }
    return p.IdleTimeout
}
```

The daemon resolves per-container settings by reading the project `claustro.yaml` from the path stored in the container's existing `claustro.project_path` label.

### 5. Standalone Research Task — debian-slim vs ubuntu:24.04

A separate file `docs/research/2026-04-19-debian-slim-vs-ubuntu.md` captures the experiment design, methodology, and result template. Owner: Peter, with assistance from claude. Output: a written go/no-go recommendation. If favorable, a separate spec/PR will plan the swap.

This research item ships in the same PR as this spec (so the template is on disk), but is not part of the implementation tasks for this feature.

## Files Changed (Implementation)

| File | Action | Purpose |
|------|--------|---------|
| `internal/sysinfo/sysinfo.go` | Create | Host CPU/memory detection |
| `internal/sysinfo/sysinfo_test.go` | Create | Table-driven detection tests |
| `internal/container/container.go` | Modify | Route empty CPUs/Memory through `smartCPUs`/`smartMemory`; add `Pause`/`Unpause` wrappers |
| `internal/container/container_test.go` | Modify (or create) | `smartCPUs` / `smartMemory` table tests |
| `internal/config/config.go` | Modify | Add `PauseConfig` |
| `internal/config/config_test.go` | Modify | YAML parse + defaults tests for `PauseConfig` |
| `internal/daemon/daemon.go` | Create | Poll loop + lifecycle |
| `internal/daemon/decide.go` | Create | Pure decision function |
| `internal/daemon/state.go` | Create | In-memory tracking map |
| `internal/daemon/launch.go` | Create | `EnsureRunning` — pidfile check + fork-exec |
| `internal/daemon/decide_test.go` | Create | Pure tests for `Decide` |
| `cmd/claustrod/main.go` | Create | Daemon entrypoint |
| `cmd/claustro/up.go` | Modify | Detect host, pass to `CreateOptions`, call `daemon.EnsureRunning` after start, surface smart caps in success output |
| `cmd/claustro/agent.go` | Modify | Auto-resume before exec (extracted helper `unpauseIfPaused`) |
| `cmd/claustro/shell.go` | Modify | Same |
| `cmd/claustro/exec.go` | Modify | Same |
| `internal/mcp/sse.go` | Modify | Add `ListSSESiblings` (parallel to existing `StopSSESiblings`) |
| `docs/research/2026-04-19-debian-slim-vs-ubuntu.md` | Create | Research stub (separate workstream) |

## Error Handling

| Case | Behavior |
|------|----------|
| Daemon fails to fork at `claustro up` | Log warn (`claustrod could not start: %v — sandboxes will run without auto-pause`); `claustro up` continues. |
| Daemon already running | `EnsureRunning` returns silently. |
| Stale pidfile | Overwrite, start new daemon. |
| `ContainerPause` fails | Daemon logs warn, sets `lastActive = now` to back off, retries next tick. |
| `ContainerUnpause` fails (in attach commands) | Return wrapped error to user — exec would fail anyway. |
| Container removed between `list` and `inspect` | Skip (catches `errdefs.NotFound`). |
| MCP SSE sibling discovery fails | Pause/unpause parent only; warn for siblings. They're orphaned if parent is gone. |
| Host detection returns 0 CPUs / 0 memory | Fall back to current hardcoded defaults (4 CPU / 8 GiB). |
| `pause.idle_timeout` parses badly | Config validation error at `claustro up` (load-time). |
| Docker socket disappears | Daemon exits cleanly. Re-launched by next `claustro up`. |

**Design principle:** auto-pause is **best-effort**, not correctness-critical. Failures fall back to "container stays running" — no worse than today. The only must-succeed path is **unpause-on-attach**.

## Testing

### Unit tests (no Docker)

- `TestSmartCPUs_Table` — covers 4/8/10/12/16-core hosts; invariant `result >= 2`.
- `TestSmartMemory_Table` — same for memory; invariant `result <= 8 GiB`.
- `TestSysinfo_DetectReturnsUsableHost` — `Detect()` always returns non-nil host with positive fields.
- `TestResolveCaps_RespectsExplicitConfig` — `claustro.yaml` with `cpus: "8"` → unchanged.
- `TestResolveCaps_AppliesSmartDefaultsWhenEmpty` — empty config → smart values applied.
- `TestPauseConfig_Defaults` — `enabled` defaults to `true`, `idle_timeout` to `5m`.
- `TestPauseConfig_OptOut` — `enabled: false` parses correctly.
- `TestDaemonDecide_NoExecPastTimeout_Pauses` — pure decision function.
- `TestDaemonDecide_ActiveExec_DoesNotPause` — running exec resets timer.
- `TestDaemonDecide_AlreadyPaused_Skipped`.
- `TestDaemonDecide_NewlyObservedContainer_GetsGracePeriod`.

### Integration tests (`//go:build integration`, requires Docker)

- `TestDaemon_PausesIdleContainer` — up sandbox; no exec; with test override `idle_timeout=2s`, assert state becomes `paused`.
- `TestDaemon_UnpauseOnAttach` — pause manually; run `claustro exec -- true`; assert state `running`.
- `TestDaemon_ExitsWhenNoContainers` — burn all sandboxes; assert daemon process exits within 60 s (`pgrep claustrod`).

### Manual smoke (pre-merge)

1. Two sandboxes running, one active. Leave the second idle 5 min. In Activity Monitor, confirm OrbStack CPU drops noticeably after auto-pause.
2. `claustro shell --name <paused-sandbox>` — confirm instant attach.
3. Burn all sandboxes — confirm `claustrod` exits within 60 s.

## Out of Scope

- Image slimming (other than the standalone research task on debian-slim).
- Disable in-container Claude Code background updater.
- Make Go/Rust/Python opt-in.
- In-container nicing.
- MCP server lazy-start.
- launchd/systemd integration for the daemon.

## Open Questions

None at spec-approval time.
