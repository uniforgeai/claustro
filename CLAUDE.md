# CLAUDE.md — claustro

## Project

claustro is a Go CLI tool that manages disposable Docker containers for running Claude Code safely against local source code. Source stays on the host (bind-mounted), containers are cheap to burn and respawn.

**Authoritative spec:** `docs/specs/spec.md` — read it before making non-trivial changes.

## Tech Stack

- **Language:** Go 1.23+
- **CLI framework:** Cobra + Viper
- **Docker:** Docker SDK for Go (`github.com/docker/docker/client`)
- **Testing:** `go test` + Testify (`github.com/stretchr/testify`)
- **Logging:** `log/slog` (structured)
- **Error handling:** `fmt.Errorf` with `%w` wrapping
- **Linting:** golangci-lint

## Project Structure

```
cmd/claustro/         # main.go entrypoint and Cobra commands
internal/             # private packages (container, config, identity, image, network, firewall, mcp)
pkg/                  # public packages (if any emerge)
docs/                 # specs, plans, archive, project context
```

## Commands

```bash
go build -o bin/claustro ./cmd/claustro
go test ./...
golangci-lint run
```

Cross-compilation targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.

## Git Conventions

Conventional Commits, linear history, squash merges to main.

```
feat: add sandbox burn command
fix: correct volume naming for multi-sandbox
chore: update golangci-lint config
test: add integration tests for firewall
refactor: extract container naming into identity package
```

## Hard Constraints

1. **MUST use Docker SDK for Go** for all container/image/network/volume operations. NEVER shell out to `docker`, `docker compose`, or `docker-compose`.
2. **NEVER mount `/var/run/docker.sock`** into sandbox containers.
3. **NEVER handle, store, intercept, or proxy Anthropic credentials.** Auth is the user's responsibility.

## Code Standards

- All Docker operations go through the Docker SDK. No `exec.Command("docker", ...)`. Ever.
- Errors: wrap with `fmt.Errorf("context: %w", err)`. Never discard errors silently.
- Logging: use `log/slog`. No `fmt.Println` or `log.Printf` in library code.
- Naming: follow Go conventions. Package names are short, lowercase, no underscores.

## Testing Requirements

- Every new exported function or method must have tests.
- Use Testify (`github.com/stretchr/testify/assert` and `require`).
- Table-driven tests for functions with multiple input variations.
- Integration tests that need Docker must be gated with `//go:build integration` build tag.
- Run `go test ./...` before submitting any work.

## Package Patterns

- Cobra command files live in `cmd/claustro/`. One file per command (`up.go`, `burn.go`, `shell.go`, etc.).
- Business logic lives in `internal/` packages, not in command files. Commands parse flags, call internal packages, and handle output only.
- Configuration loading via Viper in `internal/config`.
- Docker client interactions in `internal/container`, `internal/image`, `internal/network`.
- Sandbox identity and naming logic in `internal/identity`.

## Patterns to Avoid

- Do not put business logic in Cobra `RunE` functions. Keep them thin.
- Do not create `utils`, `helpers`, or `common` packages. Name packages by what they do.
- Do not use `init()` functions.
- Do not use global mutable state. Pass dependencies explicitly.
- Do not add third-party dependencies without justification. The stdlib is usually enough.

## Before Submitting Work

1. `go build ./...` passes with no errors.
2. `go test ./...` passes.
3. `golangci-lint run` passes with no new warnings.
4. Commit messages follow Conventional Commits format.
5. No secrets, API keys, or credentials in committed code.
6. If the change touches a spec requirement, verify behavior matches `docs/specs/spec.md`.

## Development Workflow

### Branching

Always create a new branch before starting any work. Never commit directly to `main`.

```bash
git checkout -b feat/sandbox-lifecycle
git checkout -b fix/volume-naming
```

### Git Worktrees

Use git worktrees for parallel or isolated work. The `superpowers:using-git-worktrees` skill handles setup automatically.

### Development Flow

This project uses Superpowers skills for structured development in two phases:

**Phase 1 — Think (user in the loop):**
1. `superpowers:brainstorming` — research, explore codebase, design
2. Write spec to `docs/specs/` and plan to `docs/plans/`
3. User reviews and approves

**Phase 2 — Build (agents run autonomously):**
1. `superpowers:using-git-worktrees` — isolate work
2. `superpowers:subagent-driven-development` or `superpowers:executing-plans` — implement tasks
3. `superpowers:test-driven-development` — write tests before implementation
4. `superpowers:verification-before-completion` — verify before claiming done
5. Create PR for user review
6. After merge, move completed plan to `docs/archive/`

**Superpowers output locations (overrides skill defaults):**
- Specs: `docs/specs/`
- Plans: `docs/plans/`
- Archive: `docs/archive/`

**Skip the ceremony for small, straightforward changes.** Quality practices (TDD, verification, worktrees) always apply.
