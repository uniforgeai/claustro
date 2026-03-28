# claustro — Project Context

## Identity

- **Name:** claustro
- **Binary:** `claustro`
- **Repo:** uniforgeai/claustro (GitHub)
- **Author:** Peter @ Uniforge GmbH
- **Status:** Greenfield — spec complete, implementation not started

## Purpose

A Go CLI that manages disposable Docker containers for running Claude Code safely against local source code. It provides container lifecycle management (`up`, `burn`, `shell`, `claude`, `exec`), per-project sandbox isolation, egress firewall, and MCP server support. Source is always bind-mounted from the host — never copied into the container.

## Tech Stack

- Go 1.23+, Cobra (CLI), Viper (config)
- Docker SDK for Go — no docker CLI shelling, ever
- Testify for testing, golangci-lint for linting
- `log/slog` for structured logging, `fmt.Errorf` with `%w` for errors

## Architecture

```
cmd/claustro/       → CLI entrypoint and Cobra commands
internal/config     → claustro.yaml + Viper config loading
internal/identity   → sandbox naming: {project}_{name} derived from CWD
internal/container  → Docker SDK wrapper: create, start, stop, remove, exec
internal/image      → image build/pull/rebuild logic
internal/network    → bridge network create/remove, isolation from project networks
internal/mount      → bind mount assembly (source, ~/.claude, additional mounts)
internal/firewall   → iptables egress rules (optional)
internal/mcp        → MCP server lifecycle (stdio pre-install, SSE sibling containers)
```

## Conventions

- **Commits:** Conventional Commits (`feat:`, `fix:`, `chore:`, etc.), linear history, squash merges
- **Go layout:** Standard (`cmd/`, `internal/`, `pkg/`)
- **Package naming:** short, descriptive, no `utils`/`helpers`
- **Error handling:** `fmt.Errorf("doing X: %w", err)` — always wrap, never swallow
- **Testing:** Testify assert/require, table-driven tests, `//go:build integration` for Docker-dependent tests
- **Docker rule:** All Docker operations via SDK. Never `exec.Command("docker", ...)`

## Domain Vocabulary

| Term | Meaning |
|---|---|
| sandbox | A disposable Docker container managed by claustro |
| project | The host directory (and its source code) that a sandbox targets |
| identity | The unique key for a sandbox: `{project}_{name}` (e.g., `my-saas_default`) |
| burn | Remove a container but keep the image and cache volumes |
| nuke | Remove a container AND its cache volumes |
| workspace | The `/workspace` path inside the container where source is mounted |
| claustro.yaml | Per-project configuration file for claustro |
| MCP server | Model Context Protocol server — either stdio (in-container) or SSE (sibling container) |

## Milestones

| Milestone | Scope |
|---|---|
| M1 | Core CLI + container lifecycle: `up`, `burn`, `nuke`, `rebuild`, `shell`, `claude`, `exec`, `status`, `logs`, `ls` |
| M2 | Multi-sandbox + `claustro.yaml` config, `--workdir`, `--mount`, `.env` support |
| M3 | Egress firewall (iptables), stdio MCP pre-install, SSE MCP sibling containers |
| M4 | Distribution: Homebrew formula, cross-platform binaries, docs site, CI/CD (GitHub Actions for tests, release, versioning) |
