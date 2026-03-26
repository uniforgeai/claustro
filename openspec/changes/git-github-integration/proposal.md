## Why

Claude Code's primary value is autonomous coding — writing code, committing changes, opening pull requests. Inside a claustro sandbox today, `git commit` produces commits with no author identity, `git push` fails silently, and `gh pr create` is unavailable. This makes the autonomous workflow incomplete: Claude can write code but cannot ship it.

Three missing pieces:

1. **Git identity**: `~/.gitconfig` is not mounted, so commits inside the container have no `user.name` / `user.email`. Any `git commit` from an agent produces anonymous commits.

2. **Git remote auth**: SSH private keys are not available inside the container, and macOS keychain credential helpers are host-side binaries that cannot be called from within Docker. `git push` to any remote fails.

3. **GitHub CLI**: `gh` is not installed in the image and `~/.config/gh/` (which holds the auth token) is not mounted. Claude Code cannot open PRs, review issues, or interact with GitHub beyond raw git operations.

## What Changes

### Credential forwarding (mount.go + container.Create)

Mount the following from the host into every sandbox by default (with per-key opt-outs via `sandbox.yaml`):

| Host path | Container path | Mode | Sensitivity |
|-----------|---------------|------|-------------|
| `~/.gitconfig` | `/home/sandbox/.gitconfig` | read-only | low — name, email, aliases |
| `~/.config/gh/` | `/home/sandbox/.config/gh/` | read-write | medium — GitHub PAT |
| SSH socket (`$SSH_AUTH_SOCK`) | forwarded via bind mount | — | low — no key material in container |

SSH keys themselves (`~/.ssh/`) are **not mounted by default** — SSH agent forwarding is preferred because it keeps private key material on the host. If `SSH_AUTH_SOCK` is not available and the user opts in via `sandbox.yaml`, `~/.ssh/` can be mounted read-only.

### GitHub CLI in the base image

Add `gh` (GitHub CLI) to the base Dockerfile so `gh pr create`, `gh issue list`, etc. are available without any user setup.

### `sandbox.yaml` controls

```yaml
# sandbox.yaml
git:
  forward_agent: true        # forward SSH_AUTH_SOCK if present (default: true)
  mount_gitconfig: true      # mount ~/.gitconfig read-only (default: true)
  mount_gh_config: true      # mount ~/.config/gh read-write (default: true if dir exists)
  mount_ssh_dir: false       # mount ~/.ssh read-only — fallback if no agent (default: false)
```

## Capabilities

### New Capabilities

- `git-credential-forwarding`: Host git identity and GitHub auth flow transparently into the sandbox without exposing private key material.
- `github-cli`: `gh` is available in every sandbox for PR creation, issue management, and GitHub API access.

### Modified Capabilities

- `source-mounting`: `mount.Assemble()` extended to conditionally include git-related mounts based on host path existence and `sandbox.yaml` config.
- `base-image`: Dockerfile updated to install `gh` CLI.

## Milestone

M2 — depends on `sandbox.yaml` config infrastructure (M2 scope).

## Impact

- `internal/mount/mount.go`: Add SSH agent socket forwarding + gitconfig + gh config mounts.
- `internal/config/config.go`: Add `GitConfig` struct to `Config` for the `git:` block in `sandbox.yaml`.
- `internal/image/Dockerfile`: Add `gh` CLI installation.
- `internal/container/container.go`: Forward `SSH_AUTH_SOCK` as an env var + socket bind mount when present.
- No new external Go dependencies required.
