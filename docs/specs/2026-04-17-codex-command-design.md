# `claustro codex` Command — Design

> **Status:** Draft
> **Date:** 2026-04-17
> **Author:** Peter (Uniforge GmbH)

## Overview

Add a first-class `claustro codex` subcommand that launches OpenAI's Codex CLI inside a running sandbox — symmetric to the existing `claustro claude`. Running `claustro claude` and `claustro codex` from separate terminals against the same project reuses the same container via independent `docker exec` sessions; no second container is created.

Codex support in the image (install, `~/.codex/` mount, `OPENAI_API_KEY` passthrough) already landed in PR #34. This spec covers only the command surface.

## Motivation

After #34, using Codex requires `claustro shell` followed by typing `codex`, or `claustro exec -- codex …`. Both are awkward for interactive use. A dedicated subcommand mirrors the `claustro claude` UX and makes parallel two-agent workflows frictionless.

## Non-Goals

- Host-side bin shims (`codex` / `claude` commands on the user's `$PATH` that auto-attach). Out of scope; can be added later.
- Generic `claustro agent <name>` dispatcher. Out of scope.
- Image-level changes. Already done in #34.
- Codex MCP plugin wiring. Out of scope.

## Design

### Command Surface

```
claustro codex [--name <sandbox>] [-- <extra codex args>]
```

- `--name <sandbox>` — target a specific sandbox; optional, same semantics as `claustro claude --name`.
- Args after `--` are forwarded verbatim to the in-container `codex` invocation.
- `SetInterspersed(false)` so cobra does not try to parse codex flags as its own.

### Default Arguments

`claustro codex` invokes inside the container:

```
codex --dangerously-bypass-approvals-and-sandbox [<extra args>...]
```

Rationale: the container *is* an externally sandboxed environment, and the codex CLI's own help documents this flag as "Intended solely for running in environments that are externally sandboxed." This mirrors how `claustro claude` defaults to `claude --dangerously-skip-permissions`.

### Shared Helper

Both `claude.go` and `codex.go` become thin wrappers around a shared helper in `cmd/claustro/agent.go`:

```go
type AgentSpec struct {
    Name        string   // binary name inside the container ("claude", "codex")
    ConfigKey   string   // "" for claude (always on), "codex" for the disabled-check
    DefaultArgs []string
    DisplayName string   // human-friendly, used in error/hint text
}

func runAgent(ctx context.Context, nameFlag string, spec AgentSpec, extraArgs []string) error
```

`runAgent` consolidates the logic currently in `runClaude`:

1. Resolve identity via `identity.FromCWD(nameFlag)`.
2. If `nameFlag` is empty, list containers by project and:
   - 0 running → fall through to auto-up,
   - 1 running → auto-select it,
   - 2+ running → return error listing names.
3. Call `ensureRunning(ctx, cli, id, nameWasEmpty, quiet=true, overrides)`. Signature is extended to also return the loaded `*config.Config` so step 4 can use it without re-loading.
4. If `spec.ConfigKey != ""` and `cfg.ImageBuild.IsAgentEnabled(spec.ConfigKey)` is false, return the disabled-agent error (see Error Handling).
5. Resolve the container with `container.FindByIdentity`.
6. `container.Exec(c.ID, append([]string{spec.Name}, append(spec.DefaultArgs, extraArgs...)...), ExecOptions{Interactive: true, ClipboardSockDir: …})`.

`newClaudeCmd` and `newCodexCmd` each construct their spec as a package-level var and delegate.

Typed enums (`type Agent string`) are intentionally avoided — specs are constructed in exactly two call sites and passed only to `runAgent`; a string literal here is not a source of bugs, while introducing a type would either require casting at the `internal/config` boundary or propagating the new type across packages for no real safety gain.

### `ensureRunning` Signature Change

Current:

```go
func ensureRunning(ctx, cli, id, nameWasEmpty, quiet, overrides) (*identity.Identity, bool, error)
```

Changed to surface the resolved config (already loaded internally):

```go
func ensureRunning(ctx, cli, id, nameWasEmpty, quiet, overrides) (*identity.Identity, *config.Config, bool, error)
```

`runUp` is the only other caller and is updated trivially.

### Parallelism, One Container

Falls out of the existing design with no additional work:

- First invocation (from any terminal) of `claustro claude` or `claustro codex` in a project with no sandbox → auto-up creates one container.
- Subsequent invocations of either command, from any terminal, observe `existing != nil && Up` in `ensureRunning`, return `alreadyRunning=true`, and proceed to `ContainerExecAttach` — each a fresh exec session.
- N terminals running any mix of the two agents → N exec sessions into one container.

This is already true for `claustro claude` today; adding `claustro codex` via the same path inherits it.

### Error Handling

| Case | Behavior |
|------|----------|
| No sandbox running, auto-up succeeds | `Starting sandbox <name>...` to stderr, proceed to exec. Unchanged. |
| Codex disabled in `claustro.yaml` | Return: `codex is disabled in claustro.yaml (image.agents.codex: false). Enable it and run 'claustro rebuild', or run 'claustro shell' to use other tools.` — before any exec attempt. |
| Multiple sandboxes running, `--name` missing | Return: `multiple sandboxes running, specify --name:\n  <names>`. Unchanged from `claustro claude`. |
| Auto-up fails | Wrapped error from `ensureRunning` propagates. Unchanged. |
| Codex enabled in config but binary missing from image (stale image) | Not pre-checked. `container.Exec` surfaces `exit code 127` from the shell. Fix is `claustro rebuild`. Adding a probe would be noise. |

### Output Hints

`runUp` currently prints:

```
Sandbox started: <name>
  Run: claustro shell --name <name>
  Run: claustro claude --name <name>
```

Add a third line:

```
  Run: claustro codex --name <name>
```

And the same for the branch where the name was explicit.

### Files Changed

| File | Change |
|------|--------|
| `cmd/claustro/agent.go` | **NEW** — `AgentSpec`, `runAgent`, `buildAgentCmd` helper, `claudeSpec` and `codexSpec` vars |
| `cmd/claustro/claude.go` | Refactor — `newClaudeCmd` constructs flags and delegates to `runAgent(claudeSpec)` |
| `cmd/claustro/codex.go` | **NEW** — `newCodexCmd` constructs flags and delegates to `runAgent(codexSpec)` |
| `cmd/claustro/commands.go` | Register `newCodexCmd()` |
| `cmd/claustro/up.go` | `ensureRunning` returns `*config.Config`; add `claustro codex` hint to success output |
| `cmd/claustro/agent_test.go` | **NEW** — tests for `buildAgentCmd`, spec contents, and disabled-agent check |

### Testing

**Unit tests** in `cmd/claustro/agent_test.go`:

- `TestBuildAgentCmd_Order` — `buildAgentCmd(spec, extraArgs)` returns `[spec.Name, spec.DefaultArgs..., extraArgs...]`.
- `TestAgentSpec_Codex` — `codexSpec` has `Name=codex`, `ConfigKey=codex`, default args contain `--dangerously-bypass-approvals-and-sandbox`.
- `TestAgentSpec_Claude` — `claudeSpec` has `Name=claude`, `ConfigKey=""`, default args contain `--dangerously-skip-permissions`.
- `TestCheckAgentEnabled_DisabledReturnsError` — with `cfg.ImageBuild.Agents.Codex = ptr(false)`, the pure check helper returns the expected error; with nil, it returns nil.

Testable without Docker by extracting:
- `buildAgentCmd(spec AgentSpec, extraArgs []string) []string` — pure.
- `checkAgentEnabled(cfg *config.Config, spec AgentSpec) error` — pure.

**Not adding:**

- A Docker-backed integration test for `claustro codex` — duplicates the existing `claustro claude` exec path; zero new coverage for agent-specific logic.
- Manual end-to-end verification (open two terminals, `claustro claude` + `claustro codex`, `claustro ls` shows one container) is recommended before merging but not codified as an automated test.

**Pre-merge verification** per `CLAUDE.md`:

```
go build ./...
go test ./...
golangci-lint run
```

## Out of Scope

- Host-side shims (`codex`/`claude` on `$PATH`).
- Generic `claustro agent <name>` dispatcher.
- Typed enum for agent names (YAGNI at two call sites).
- Image-level changes (codex install, mounts, env) — landed in #34.
- Probing the image for a missing codex binary when config says it should be present.

## Open Questions

None at spec-approval time.
