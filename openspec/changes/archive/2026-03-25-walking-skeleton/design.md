## Context

The POC confirmed that the bind-mount approach works but revealed three implementation constraints that shape the design:
1. `~/.claude.json` lives at `$HOME/.claude.json` (not inside `~/.claude/`) and must be mounted separately
2. Claude Code indexes projects by absolute path — running from `/workspace` creates a separate project entry from the host's path; a host-path symlink inside the container is the workaround
3. Ubuntu 24.04 has uid 1000 occupied; the sandbox user must use uid 9999

This design covers the full walking skeleton: Go module setup, internal package architecture, embedded Dockerfile, and the five CLI commands.

## Goals / Non-Goals

**Goals:**
- Minimal working CLI: `up`, `shell`, `claude`, `burn`, `ls`
- Correct bind mounts: source, `~/.claude`, `~/.claude.json`
- Host-path symlink applied at container startup for project state continuity
- Docker SDK only — no shelling out
- Non-root sandbox user (uid 9999)
- Sandbox identity from CWD basename + `--name` flag

**Non-Goals:**
- `claustro.yaml` config file (M2)
- Named sandboxes beyond `--name` flag (M2)
- Egress firewall (M3)
- MCP server support (M3)
- Resource limits, `--readonly`, `--mount`, `--workdir` flags (M2)
- Cross-compilation / release binaries (M4)

## Decisions

### 1. Embedded Dockerfile via `embed.FS`

The Dockerfile is embedded into the binary at compile time using Go's `embed` package. At `up` time, the embedded Dockerfile is written to a temp directory and passed to the Docker SDK image build.

**Why over generated/templated Dockerfile:** Simpler, predictable, user-inspectable in the repo. Can always add templating later.

**Tradeoff:** One-size-fits-all image (~1-2 GB). Acceptable for M1; M2 can add `--image` override.

### 2. Host-path symlink via entrypoint wrapper script

The container is started with a minimal shell entrypoint (`/usr/local/bin/claustro-init`) that:
1. Runs as root
2. Creates `mkdir -p <parent-of-host-path>` and `ln -sf /workspace <host-path>` inside the container
3. `chown`s the symlink tree to `sandbox:sandbox`
4. `exec`s `sleep infinity` (or the user's command) as the sandbox user via `su -c`

The host project path is passed in as an environment variable (`CLAUSTRO_HOST_PATH`).

**Why over bind-mounting the host path directly:** The host path only exists on the host — we can't mount it. A symlink is the only way to make `/Users/pepusz/code/foo` resolvable inside the container.

**Why over `CLAUDE_CONFIG_DIR`:** That env var overrides the config dir, not the project path. It doesn't fix the project index key.

### 3. Package structure

```
cmd/claustro/
  main.go          # cobra root command wiring
  up.go            # `claustro up`
  shell.go         # `claustro shell`
  claude.go        # `claustro claude`
  burn.go          # `claustro burn`
  ls.go            # `claustro ls`

internal/
  identity/        # sandbox naming: project slug + name → all resource names
  image/           # Dockerfile embed + Docker SDK image build
  mount/           # bind mount slice assembly
  container/       # Docker SDK: create, start, stop, remove, exec, list
```

Commands are thin: parse flags → call internal packages → print result. No business logic in `RunE`.

### 4. Docker resource naming

All Docker resources follow the naming spec to avoid collisions:
- Container: `claustro-{project}-{name}`
- Network: `claustro-{project}-{name}-net`
- Volumes: `claustro-{project}-{name}-{purpose}` (npm-cache, pip-cache etc. — M2)

Project slug: CWD directory basename, lowercased, non-alphanumeric replaced with `-`.

### 5. `~/.claude.json` mount — conditional

The file is only mounted if it exists on the host. If absent (new user), the container starts without it and Claude Code will create it on first run — inside the container's `$HOME`, which is bind-mounted as `~/.claude/` (the directory). Since `~/.claude.json` is NOT inside `~/.claude/`, a new file created inside the container won't persist. Documented as a known limitation for M1.

### 6. `claustro ls` scope

`ls` without flags shows only sandboxes for the current project (matching container name prefix `claustro-{project}-`). `--all` shows everything with `claustro-` prefix. Uses Docker SDK `ContainerList` with label filters.

All containers created by claustro get a label `claustro.project={project}` for clean filtering.

## Risks / Trade-offs

- **Symlink fragility** → The symlink approach works but is invisible to the user. If they're confused why `/Users/pepusz/code/foo` exists inside the container, it might be surprising. Mitigation: document in README, log it during `up`.
- **Image build time** → First `up` builds a ~1-2 GB image with all runtimes. Could take 3-5 minutes. Mitigation: print a progress indicator; subsequent runs use the cached image.
- **`~/.claude.json` not persisted if created inside container** → If the user runs `claude login` inside a fresh container and `~/.claude.json` doesn't exist on the host, the new `.claude.json` won't be on the host. Mitigation: document; full fix in M2 via additional mount.
- **Single default sandbox** → M1 only supports `--name` flag for identity, no `claustro.yaml`. Fine for walking skeleton scope.

## Open Questions

- Should `claustro up` auto-open a shell after starting, or just print the container name and exit? (Lean: just print — keep commands composable.)
- Should we add `--follow` / `-f` to `ls` for a live watch? (Defer to M2.)
