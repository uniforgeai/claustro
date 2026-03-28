# Change: Validate Mount Security

## Summary

Add a `claustro validate` command (or `claustro doctor --security`) that inspects a sandbox's resolved mount configuration and container security settings, flagging any mounts that expose sensitive host directories or weaken container isolation.

## Motivation

claustro bind-mounts host paths into containers. The default mounts are carefully scoped, but users can add arbitrary mounts via `claustro.yaml` or `--mount` flags. A misconfigured mount — e.g., mounting `$HOME`, `/etc`, `/var/run/docker.sock`, or a credentials directory — can silently expose sensitive data to the sandbox. Since Claude Code runs with `--dangerously-skip-permissions`, anything mounted is fully accessible.

There is no feedback loop today: if a user accidentally mounts `~/` instead of `~/projects/my-app`, nothing warns them. A validation command that audits the resolved mount set before or after `up` closes this gap.

## Behavior

- **WHEN** the user runs `claustro validate [name]`
- **THEN** the CLI resolves the full mount configuration (defaults + config file + flags)
- **AND** checks each mount against a set of security rules
- **AND** prints a report of findings with severity (error/warning/info)

Example output:

```
claustro validate

  Mount Security
  ✗  BLOCK   /var/run/docker.sock → /var/run/docker.sock  — Docker socket must never be mounted
  ✗  ERROR   /home/peter → /home/sandbox                  — Home directory is too broad; mount specific subdirectories
  ⚠  WARN    /home/peter/.ssh → /home/sandbox/.ssh (rw)   — SSH directory should be read-only
  ✓  OK      /home/peter/projects/my-app → /workspace
  ✓  OK      /home/peter/.claude → /home/sandbox/.claude
  ✓  OK      /home/peter/.gitconfig → /home/sandbox/.gitconfig (ro)

  Container Security
  ✓  OK      no-new-privileges: true
  ✓  OK      privileged: false
  ✓  OK      no added capabilities
  ✓  OK      resource limits set (4 CPUs, 8 GB)

  5/7 checks passed. 1 blocked, 1 warning.
```

## Security Rules

### Mount rules (blocked — hard errors)

| Rule | Description |
|------|-------------|
| No Docker socket | `/var/run/docker.sock` must never be mounted (spec requirement) |
| No `/etc` | Exposes system configuration, shadow passwords, sudoers |
| No `/proc` or `/sys` | Kernel interfaces must not be bind-mounted |
| No root filesystem | `/` as source path is never acceptable |

### Mount rules (errors — strongly discouraged)

| Rule | Description |
|------|-------------|
| No home directory | `$HOME` or `/home/<user>` as source — too broad, mount specific subdirs |
| No `/var` | Exposes logs, mail, system state |
| No known credential paths | `~/.aws`, `~/.kube`, `~/.config/gcloud`, `~/.vault-token`, etc. |
| No `/tmp` or `/var/tmp` | Shared temp dirs can leak data between processes |

### Mount rules (warnings)

| Rule | Description |
|------|-------------|
| SSH dir should be ro | `~/.ssh` mounted as rw is risky; prefer ro or agent forwarding |
| Broad mounts | Source path with fewer than 3 path components gets a warning |
| Writable when ro would suffice | Known config paths (`.gitconfig`, `.npmrc`) mounted rw |

### Container security checks

| Check | Expected |
|-------|----------|
| `no-new-privileges` | Must be `true` |
| Privileged mode | Must be `false` |
| Added capabilities | Should be empty |
| Resource limits | CPU and memory limits should be set |
| User | Should be non-root (uid != 0) |

## Modes

1. **Pre-flight (default)**: Resolve config and validate mounts *before* starting a container. Works with `claustro validate` even if no sandbox is running.
2. **Live inspection**: If a sandbox is running, inspect the actual container configuration via Docker API and validate that too. Use `claustro validate --live [name]`.

## Scope

- New `validate.go` command in `cmd/claustro/`
- New `internal/validate/` package with rule engine and mount/security checks
- Rules are defined in code (not config) — they enforce the spec's hard constraints
- Integrates with existing `internal/mount` and `internal/config` packages

## Out of Scope

- Auto-fixing mount issues (just report)
- Network/firewall validation (separate concern, future work)
- Image content scanning (e.g., CVE scanning)
- Runtime behavior monitoring
