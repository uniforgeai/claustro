# M4: Polish & Distribution — Design Spec

> **Status:** Approved
> **Date:** 2026-03-29
> **Milestone:** M4

---

## Overview

M4 completes claustro with distribution tooling, a configurable image system, an interactive setup wizard, config validation, and a documentation site. Seven items grouped into three workstreams.

---

## 1. Image Configurator + Init Wizard

### 1.1 Data Model — `image:` block in `claustro.yaml`

```yaml
image:
  languages:
    node: true      # always on, cannot be disabled
    go: true
    rust: false
    python: true
  tools:
    dev: true       # ripgrep, fd, fzf, jq, tree, htop, tmux
    build: true     # gcc, pkg-config, libssl-dev, make
  mcp_servers:
    filesystem: true
    memory: true
    fetch: true
```

**Rules:**
- `node` is always `true`. Validation rejects `node: false`.
- When `image:` block is absent, all languages/tools/mcp_servers default to `true` (current behavior — full polyglot image).
- The `image:` config controls what goes into the Dockerfile at build time.

### 1.2 Templated Dockerfile

The current static `internal/image/Dockerfile` (embedded via `//go:embed`) is replaced with a **Go template**.

Template sections:
1. **Base** (always) — Ubuntu 24.04, git, curl, ca-certificates, zsh, gnupg, iptables
2. **Node.js** (always) — Node LTS, npm, Claude Code
3. **Go** (optional) — Go latest stable
4. **Rust** (optional) — rustup + stable toolchain
5. **Python** (optional) — Python 3 + pip + venv
6. **Dev tools** (optional) — ripgrep, fd, fzf, jq, tree, htop, tmux
7. **Build essentials** (optional) — gcc, pkg-config, libssl-dev, make
8. **MCP servers** (optional, per-server) — filesystem, memory, fetch
9. **GitHub CLI** (always) — gh
10. **User setup** (always) — sandbox user, home, shell, entrypoint

`internal/image/` renders the template with `ImageConfig` values, then builds via Docker SDK as before. The rendered Dockerfile is never written to disk — it's passed directly to the Docker build context as an in-memory tar.

### 1.3 `claustro init` — Setup Wizard

Interactive step-by-step wizard using `charmbracelet/huh`:

1. **Project name** — text input, default: directory basename
2. **Languages** — multi-select: Go, Rust, Python (Node shown as always-on)
3. **Tool groups** — checkboxes: dev tools, build essentials
4. **MCP servers** — checkboxes: filesystem, memory, fetch
5. **Resource defaults** — CPUs (default: 4), memory (default: 8G)
6. **Firewall** — confirm: enable egress firewall? (default: no)
7. **Read-only** — confirm: mount source read-only? (default: no)
8. **Git config** — checkboxes: forward SSH agent, mount gitconfig, mount gh config
9. **Review & confirm** — show generated YAML, confirm write

**Output:** Writes `claustro.yaml` to the current directory.

**Flag overrides** — every wizard step can be pre-answered via flags:
- `--project NAME`
- `--languages go,python`
- `--tools dev,build`
- `--mcp filesystem,memory`
- `--cpus N`, `--memory SIZE`
- `--firewall`, `--readonly`
- `--yes` / `-y` — accept all defaults, skip prompts

`claustro init -y` generates a config with sensible defaults and no interaction.

If `claustro.yaml` already exists, prompt for confirmation before overwriting.

### 1.4 `claustro config` — Configuration Management

**Interactive subcommands** — each opens a `huh` form pre-filled with current values:

| Subcommand | Section |
|------------|---------|
| `claustro config languages` | Image language selection |
| `claustro config tools` | Image tool groups |
| `claustro config mcp` | MCP server toggles |
| `claustro config firewall` | Firewall enabled + allowed domains |
| `claustro config defaults` | CPUs, memory, readonly |
| `claustro config git` | Git integration toggles |

After confirmation, the relevant section of `claustro.yaml` is updated in-place.

**Get/set for scripting:**

```bash
# Get a value
claustro config get image.languages.go        # → true
claustro config get defaults.resources.cpus    # → 4

# Set a value
claustro config set defaults.resources.cpus 8
claustro config set image.languages.rust true
claustro config set firewall.enabled true
```

- `get` prints the value to stdout (machine-readable).
- `set` updates `claustro.yaml` in-place. Creates the file with defaults if missing.
- Dot-notation paths map to YAML structure.

### 1.5 New Dependency

```
github.com/charmbracelet/huh
```

From the Charm ecosystem (bubbletea, lipgloss, bubbles). Used for interactive prompts in `init` and `config` subcommands.

---

## 2. Release Pipeline

### 2.1 GoReleaser (`.goreleaser.yaml`)

**Cross-compilation targets:**
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

**Archives:**
- tar.gz for Linux
- zip for macOS
- Include README.md and LICENSE in archives

**Checksums:** SHA256 checksums file generated automatically.

**Changelog:** Auto-generated from conventional commit messages:
- `feat:` → "Features"
- `fix:` → "Bug Fixes"
- `chore:`, `docs:`, `test:`, `refactor:` → grouped accordingly

**Homebrew:** GoReleaser auto-pushes updated formula to `uniforgeai/homebrew-tap`.

### 2.2 GitHub Actions

**CI workflow (`.github/workflows/ci.yml`):**
- **Trigger:** push to main, pull requests to main
- **Steps:**
  1. Checkout
  2. Setup Go
  3. `go build ./...`
  4. `go test ./...`
  5. `golangci-lint run` (via `golangci/golangci-lint-action`)

**Release workflow (`.github/workflows/release.yml`):**
- **Trigger:** tag push matching `v*`
- **Steps:**
  1. Checkout
  2. Setup Go
  3. Run GoReleaser (`goreleaser/goreleaser-action`)
- **Secrets needed:** `HOMEBREW_TAP_TOKEN` — a PAT with write access to `uniforgeai/homebrew-tap`

### 2.3 Homebrew Tap

**Repository:** `uniforgeai/homebrew-tap`

**Installation:**
```bash
brew tap uniforgeai/tap
brew install claustro
```

**Formula:** Auto-generated by GoReleaser. Downloads the correct platform binary, verifies SHA256 checksum.

### 2.4 Release Flow

```
developer:
  git tag v0.1.0
  git push --tags

github actions (release.yml):
  → GoReleaser cross-compiles 4 targets
  → Creates GitHub Release with binaries + checksums + changelog
  → Pushes formula to uniforgeai/homebrew-tap

user:
  brew tap uniforgeai/tap
  brew install claustro
  # or: download binary from GitHub Releases
```

---

## 3. Config Validation

### 3.1 Validation at Load Time

`config.Load()` calls `Validate()` after YAML parsing. Returns `[]ValidationResult`:

```go
type ValidationResult struct {
    Field    string   // e.g., "sandboxes.api.mounts[0]"
    Message  string   // human-readable description
    Severity Severity // Error or Warning
}
```

**Checks:**

| Category | Check | Severity |
|----------|-------|----------|
| Type | Languages/tools/mcp_servers are booleans | Error |
| Type | CPUs parseable as number | Error |
| Type | Memory matches pattern `\d+[GMK]` | Error |
| Constraint | `image.languages.node` cannot be false | Error |
| Constraint | CPUs > 0, memory > 0 | Error |
| Format | Mounts match `host:container[:ro\|rw]` | Error |
| Format | Sandbox names are Docker-safe identifiers | Error |
| Format | Firewall allow entries are valid hostnames | Warning |
| Unknown | Unrecognized top-level or nested keys | Warning |

**Behavior:**
- Errors block command execution (return error from `Load()`).
- Warnings are logged via `slog.Warn()` but do not block.

### 3.2 `claustro validate` Command

Standalone command for CI and pre-commit:

```bash
$ claustro validate
claustro.yaml: valid (1 warning)
  warn: defaults.resources.cpus: "0" is unusually low

$ claustro validate
claustro.yaml: 2 errors found
  error: sandboxes.api.mounts[0]: invalid mount format "libs" (expected host:container[:mode])
  error: image.languages.node: cannot be disabled
```

- Exit code 0: valid (warnings OK).
- Exit code 1: errors found.
- If no `claustro.yaml` exists: prints "no claustro.yaml found" and exits 0.

### 3.3 Doctor Integration

The existing `claustro doctor` check "Config File" is extended to run full `Validate()`:

- Currently: only checks YAML parse succeeds.
- After: runs all validation checks. Reports errors as FAIL, warnings as WARN.

---

## 4. Documentation Site (Hugo)

### 4.1 Setup

- **Location:** `docs/site/`
- **Generator:** Hugo
- **Theme:** hugo-book (clean, docs-focused, sidebar navigation)
- **Build:** `hugo -s docs/site`
- **Local preview:** `hugo server -s docs/site`

### 4.2 Content Structure

```
docs/site/
├── hugo.toml
├── content/
│   ├── _index.md                    # Landing page
│   ├── getting-started/
│   │   ├── _index.md
│   │   ├── installation.md          # Homebrew, binary, go install
│   │   ├── quickstart.md            # First sandbox in 2 minutes
│   │   └── configuration.md         # claustro.yaml reference
│   ├── commands/
│   │   ├── _index.md
│   │   ├── init.md
│   │   ├── up.md
│   │   ├── burn.md
│   │   ├── shell.md
│   │   ├── claude.md
│   │   ├── exec.md
│   │   ├── ls.md
│   │   ├── nuke.md
│   │   ├── rebuild.md
│   │   ├── status.md
│   │   ├── logs.md
│   │   ├── config.md
│   │   ├── validate.md
│   │   └── doctor.md
│   ├── guides/
│   │   ├── _index.md
│   │   ├── monorepo.md              # Multi-sandbox monorepo setup
│   │   ├── firewall.md              # Egress firewall configuration
│   │   ├── mcp-servers.md           # Stdio + SSE MCP setup
│   │   └── custom-image.md          # Image configuration & extension
│   └── reference/
│       ├── _index.md
│       ├── claustro-yaml.md         # Full config file reference
│       └── environment-variables.md # All supported env vars
├── static/
└── themes/
```

### 4.3 Deployment

- **GitHub Pages** via GitHub Actions.
- Add a docs build/deploy step to CI that runs on push to main.
- Hugo builds static site → deploys to `gh-pages` branch or GitHub Pages artifact.

---

## New/Modified Files Summary

### New Files
| File | Purpose |
|------|---------|
| `cmd/claustro/init.go` | `claustro init` wizard command |
| `cmd/claustro/config_cmd.go` | `claustro config` command + subcommands |
| `cmd/claustro/validate.go` | `claustro validate` command |
| `internal/config/validate.go` | Config validation logic |
| `internal/wizard/` | Shared `huh` form builders for init + config |
| `.goreleaser.yaml` | GoReleaser configuration |
| `.github/workflows/ci.yml` | CI workflow |
| `.github/workflows/release.yml` | Release workflow |
| `.github/workflows/docs.yml` | Docs site deployment |
| `docs/site/` | Hugo documentation site |

### Modified Files
| File | Change |
|------|--------|
| `internal/image/Dockerfile` | Replaced with Go template |
| `internal/image/image.go` | Template rendering logic |
| `internal/config/config.go` | Add `ImageConfig` with languages/tools/mcp_servers |
| `internal/doctor/doctor.go` | Extend config check with full validation |
| `cmd/claustro/commands.go` | Register init, config, validate commands |
| `go.mod` | Add `charmbracelet/huh` dependency |

---

## Scope Boundaries

**In scope:**
- Everything described above.

**Out of scope (deferred):**
- JSON Schema generation for editor autocomplete (future enhancement)
- Windows support
- GPU passthrough
- Plugin/extension model beyond `image.extra` steps
- Docker Compose integration
