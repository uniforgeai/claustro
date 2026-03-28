## Why

The claustro base image (`claustro:latest`) is a fixed, opinionated polyglot environment. Three problems need addressing:

1. **Missing tool**: `ccstatusline` (Claude Code status line) is not in the base image. Every user has to install it manually inside each sandbox, which defeats the "disposable and instantly usable" promise.

2. **No extensibility**: There is no way for a project to say "also install this tool" or "add this apt package" without forking the Dockerfile. Power users hit this wall immediately when their workflow requires additional dependencies.

3. **Image bloat vs. usefulness tension**: The base image installs Go, Rust, Python, and Node regardless of what the project actually uses. A pure JS project carries 600 MB of Go and Rust it will never need. Conversely, the image might be missing the exact runtime version a specific project requires. A smarter image build — one that installs only what the codebase needs — would be leaner, faster to pull, and more relevant.

## What Changes

### Immediate (this change)

- **Add `ccstatusline`** to the base Dockerfile's global npm installs so it is available in every sandbox without manual setup.

- **`claustro.yaml` image extensions** (`image.extra`): A new `image.extra` section in `claustro.yaml` lets projects declare additional Dockerfile `RUN` steps that are appended when building a project-specific image layer. This is the escape hatch for "my project needs `ffmpeg`" or "add this pip package."

### Future (spec only — revisit before implementing)

- **Adaptive image based on codebase scan**: claustro detects which runtimes are actually present in the project (`.go` files → Go, `Cargo.toml` → Rust, `package.json` → Node, `requirements.txt` / `.python-version` → Python) and builds a leaner image that includes only those runtimes. This feature needs more design work before implementation — see the "Future: Adaptive Image" section in design.md.

## Capabilities

### New Capabilities

- `image-extensions`: Per-project `claustro.yaml` `image.extra` blocks append custom Dockerfile RUN steps to a project-local image layer, enabling per-project image customization without modifying the shared base.

### Modified Capabilities

- `base-image`: Base Dockerfile updated to include `ccstatusline` alongside `@anthropic-ai/claude-code` in the global npm install step.

## Impact

- `internal/image/Dockerfile`: Add `ccstatusline` to npm install line.
- `internal/config/`: New `claustro.yaml` schema fields for `image.extra`.
- `internal/image/`: New `BuildExtended` function that layers project-specific extensions over the base image.
- `cmd/claustro/up.go`: Use `BuildExtended` when `image.extra` is present in config.
- No new external Go dependencies required.
