# Change: Doctor Command

## Summary

Add a `claustro doctor` command that checks the health of the host environment and sandbox prerequisites. It validates Docker, Git, GitHub CLI, SSH agent, clipboard bridge, and other integrations — reporting what works, what's broken, and how to fix it.

## Motivation

claustro depends on several host-level integrations (Docker, git config, SSH agent forwarding, `gh` CLI auth, clipboard socket). When something breaks, users get cryptic errors deep in sandbox creation or exec. A single `claustro doctor` command that validates everything upfront saves debugging time and makes the tool approachable for new users.

## Behavior

- **WHEN** the user runs `claustro doctor`
- **THEN** the CLI runs a series of checks and prints a report

Example output:

```
claustro doctor

  Docker Engine    ✓  Docker 27.5.1, API 1.47
  Docker Socket    ✓  /var/run/docker.sock accessible
  Base Image       ✓  claustro-base:latest (built 2h ago)
  Git Config       ✓  ~/.gitconfig found
  SSH Agent        ✓  SSH_AUTH_SOCK set, 2 keys loaded
  GitHub CLI       ✗  gh auth token expired — run: gh auth login
  Clipboard        ✓  Host clipboard accessible
  Config File      ✓  claustro.yaml found (project: my-saas)

  6/7 checks passed. 1 issue found.
```

## Checks

| Check | What it validates | Fix hint |
|-------|-------------------|----------|
| Docker Engine | Docker daemon reachable via SDK, version ≥ 24.0 | Install/start Docker |
| Docker Socket | `/var/run/docker.sock` exists and is accessible | Check Docker Desktop / permissions |
| Base Image | `claustro-base:latest` exists locally | Run `claustro rebuild` |
| Git Config | `~/.gitconfig` exists | Run `git config --global user.name/email` |
| SSH Agent | `SSH_AUTH_SOCK` set and agent has keys | Run `ssh-add` |
| GitHub CLI | `gh auth status` succeeds, `oauth_token` present in `~/.config/gh/hosts.yml` | Run `gh auth login` on the host (token persists into sandboxes via bind mount) |
| Clipboard | Host clipboard is accessible (platform-specific) | Install xclip/pbcopy or check Wayland |
| Config File | `claustro.yaml` exists in CWD (optional, info only) | Not required — uses defaults |

## Scope

- New `doctor.go` command in `cmd/claustro/`
- New `internal/doctor/` package with individual check functions
- Each check returns a result struct: `{Name, Status, Detail, FixHint}`
- Checks run sequentially (some depend on Docker being available)

## Out of Scope

- Auto-fixing issues (just report and suggest)
- Network connectivity checks (egress firewall is M3)
- MCP server health checks (M3)
