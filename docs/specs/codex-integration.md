# Codex CLI Integration

## Overview

Add OpenAI Codex CLI to the claustro sandbox image as a second coding agent alongside Claude Code. Codex is installed by default and can be disabled via config. Host credentials and config are bind-mounted into the container so users can authenticate via API key, device code flow, or pre-existing OAuth tokens.

## Motivation

Codex has an official MCP plugin for Claude Code, enabling delegation between the two agents inside a sandbox. Users may also want to run Codex independently or in parallel with Claude Code. Making both available by default maximises the sandbox's utility as a multi-agent coding environment.

## Design

### 1. Image Config Toggle

Add an `AgentsConfig` struct to `ImageBuildConfig` following the existing opt-out pattern (nil pointer = enabled):

```yaml
# claustro.yaml
image:
  agents:
    codex: true  # default; set false to exclude from image
```

- New struct: `AgentsConfig` with field `Codex *bool`
- New method: `IsAgentEnabled(agent string) bool` on `ImageBuildConfig`
- Claude Code remains unconditional — it is the core of the product and has no toggle.

### 2. Dockerfile Template

When Codex is enabled, install via npm (Node.js is always present in the image):

```dockerfile
{{- if .Agents.Codex }}
# --- Codex CLI ---
RUN npm install -g @openai/codex
{{- end }}
```

Placement: after the Node.js / language runtime block, before dev tools.

### 3. Host Config Mounts

Mount `~/.codex/` from the host into the container, mirroring the existing `~/.claude/` pattern:

| Host path | Container path | Mode | Condition |
|-----------|---------------|------|-----------|
| `~/.codex/` | `/home/sandbox/.codex/` | read-write | exists on host AND `isolatedState=false` |

Implementation in `internal/mount/mount.go`:
- Add `containerCodexDir` constant: `/home/sandbox/.codex`
- Add `addCodexMounts()` function following `addClaudeMounts()` pattern
- Called from `Assemble()` after `addClaudeMounts()`
- Guard with `os.Stat` — skip silently if `~/.codex/` does not exist on host
- Skip when `isolatedState=true`

### 4. Environment Variables

Pass `OPENAI_API_KEY` from the host environment into the container if set. This supports API-key-based authentication without requiring a config file.

Implementation: add to the env var assembly logic in container creation, alongside existing variables like `ANTHROPIC_API_KEY`.

### 5. Authentication Flows

Codex supports three auth methods. All work inside the sandbox with this design:

| Method | How it works in sandbox |
|--------|------------------------|
| **API key** | `OPENAI_API_KEY` env var passed through from host |
| **Device code flow** | Works natively — browser-independent, user enters code at openai.com |
| **OAuth (ChatGPT sign-in)** | Tokens in `~/.codex/auth.json` carried in via bind mount; no new browser flow inside container |

### 6. Config Structure

The YAML config path for this feature:

```yaml
image:
  agents:
    codex: false  # opt-out example
```

Follows the same opt-out semantics as `languages`, `tools`, and `mcp_servers`: nil = enabled.

## Files to Change

| File | Change |
|------|--------|
| `internal/config/image_config.go` | Add `AgentsConfig`, `IsAgentEnabled()` |
| `internal/image/Dockerfile.tmpl` | Add Codex install block |
| `internal/image/template.go` | Pass `Agents` struct to template data |
| `internal/mount/mount.go` | Add `addCodexMounts()`, call from `Assemble()` |
| `internal/container/` (env assembly) | Add `OPENAI_API_KEY` passthrough |
| Tests for all of the above | |

## Out of Scope

- Codex MCP plugin configuration (user can set this up via their `~/.codex/config.toml`)
- Codex version pinning (use latest from npm, consistent with Claude Code auto-update approach)
- `claustro update` bug (deferred until reproducible after next release)
