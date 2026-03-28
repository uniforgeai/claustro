## Context

The claustro image pipeline today: `internal/image/image.go` embeds `Dockerfile` and `claustro-init` at compile time, builds a single `claustro:latest` image, and every sandbox uses that image. There is no per-project image variant and no config-driven customization.

`internal/config/` does not yet exist — it will be created as part of M2. This change introduces the `claustro.yaml` schema and config loading as a prerequisite for image extensions.

## Goals / Non-Goals

**Goals:**
- Add `ccstatusline` to the base image npm installs — zero user effort required.
- Allow `claustro.yaml` to declare `image.extra` RUN steps appended as a thin layer over `claustro:latest`.
- Keep the base image unchanged for projects that have no `claustro.yaml` or no `image.extra` block.
- Spec the adaptive image feature clearly enough that it can be implemented in a future change.

**Non-Goals:**
- Replacing or rebuilding the base image per project (too slow, wrong direction).
- Parsing the base Dockerfile or doing any Dockerfile manipulation beyond appending a `FROM` + `RUN` extension layer.
- Implementing the adaptive image feature in this change (needs more design work).
- Multi-stage builds, BuildKit secrets, or other advanced Docker build features.

## Decisions

### ccstatusline: add to the base npm install step

`ccstatusline` is installed alongside `@anthropic-ai/claude-code` in a single `RUN npm install -g` layer. This keeps the layer count low and ensures ccstatusline is always available in every sandbox. No new Dockerfile step needed.

```dockerfile
RUN npm install -g @anthropic-ai/claude-code ccstatusline
```

### image.extra: extension layer via a generated Dockerfile

When `claustro.yaml` contains an `image.extra` block, claustro generates a minimal `Dockerfile.ext` in memory:

```dockerfile
FROM claustro:latest
RUN <step1>
RUN <step2>
```

This is built as a new image tagged `claustro-{project}:latest`. Sandboxes for that project use `claustro-{project}:latest` instead of `claustro:latest`. The base image is never modified.

**Why a separate image tag rather than build args?** Build args require re-baking the entire base image, which is slow. A thin `FROM claustro:latest` + `RUN` layer is fast and only re-builds the delta. Docker layer caching handles the rest.

**Image tag format**: `claustro-{project}:latest` (e.g. `claustro-myapp:latest`). This is derived from the sandbox identity's `Project` slug — consistent with container and network naming.

### claustro.yaml schema for image.extra

```yaml
image:
  extra:
    - run: apt-get install -y ffmpeg
    - run: pip install black ruff
    - run: npm install -g prettier
```

Each entry under `extra` is a `run` key containing a shell command that becomes a Dockerfile `RUN` step. This is intentionally simple — no `COPY`, no `ADD`, no `ARG`. The escape hatch for arbitrary Dockerfile instructions is forking the image entirely.

### config loading: internal/config package

`internal/config` will use Viper to load `claustro.yaml` from the project root (same directory as the CWD when claustro is invoked). The loaded config is passed into `up.go` to inform image selection.

Key type:
```go
type Config struct {
    Image ImageConfig `mapstructure:"image"`
}

type ImageConfig struct {
    Extra []ExtraStep `mapstructure:"extra"`
}

type ExtraStep struct {
    Run string `mapstructure:"run"`
}
```

### up command: pick base vs. extended image

`runUp` loads config from CWD. If `config.Image.Extra` is non-empty:
1. Call `image.EnsureExtended(ctx, cli, project, config.Image.Extra, os.Stdout)` — builds `claustro-{project}:latest` if not present.
2. Pass the extended image name to `container.Create`.

If `config.Image.Extra` is empty (or no `claustro.yaml`), behaviour is unchanged: uses `claustro:latest`.

`EnsureExtended` checks if `claustro-{project}:latest` exists **and** whether its label `claustro.ext-hash` matches a hash of the `extra` steps. If the hash differs, the extension image is rebuilt. This prevents stale extension images after `claustro.yaml` changes.

### rebuild command: rebuild extension image too

`claustro rebuild` rebuilds `claustro:latest`. If `claustro-{project}:latest` exists, it should also be rebuilt (it depends on the base). `rebuild` will optionally rebuild the extension image by calling `image.BuildExtended` after the base rebuild.

---

## Future: Adaptive Image (spec only — do not implement yet)

This section captures the design intent so it can be properly scoped and tasked in a future change.

**Goal**: Detect which runtimes a project actually uses and build a leaner image that includes only those.

**Detection heuristics**:
| Signal | Runtime |
|--------|---------|
| `*.go` files or `go.mod` | Go |
| `Cargo.toml` or `*.rs` files | Rust |
| `package.json` | Node.js |
| `requirements.txt`, `pyproject.toml`, `.python-version`, `*.py` | Python |

**Approach**: A `claustro scan` subcommand (or automatic scan during `up`) walks the project directory shallowly (top-level + 1 level deep), applies the heuristics, and writes a `.claustro-runtimes` cache file. The image build uses the detected runtimes to select which Dockerfile stages to include.

**Open questions before implementing**:
- Should this produce a separate image per runtime combination, or use BuildKit `--target` stages?
- How does this interact with `image.extra`? (Extensions might assume a runtime is present.)
- What is the re-scan trigger? (File changes, manual `claustro scan`, or always-on during `up`?)
- What is the UX for "I need Go but my project doesn't have Go files yet"? (Override in `claustro.yaml`?)
- What is the build-time cost of N separate Dockerfile stages vs. a single polyglot image?

**Decision**: Do not implement until these questions are answered. Track as a separate change after M2.

---

## Risks / Trade-offs

- **Extension image stale detection via hash**: The `claustro.ext-hash` label approach requires that `extra` steps be deterministic and order-sensitive. Reordering steps in `claustro.yaml` will trigger a rebuild even if the net effect is identical. This is acceptable — determinism is more important than avoiding spurious rebuilds.
- **Extension image cleanup**: `claustro nuke` and `claustro burn` only remove containers, not images. Extension images (`claustro-{project}:latest`) will accumulate unless the user manually prunes. A future `claustro prune` command can address this.
- **`claustro.yaml` not yet loaded in M1 commands**: `up` currently doesn't read any config. Adding config loading is an additive change; commands without `claustro.yaml` present will behave identically to today.
