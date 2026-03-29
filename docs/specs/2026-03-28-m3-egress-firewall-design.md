# M3 Egress Firewall — Design Spec

> **Status:** Draft
> **Date:** 2026-03-28

## Overview

The egress firewall restricts outbound network traffic from sandbox containers to a curated allowlist of domains. It uses iptables rules applied inside the container's network namespace via Docker SDK `ContainerExec`, with the `NET_ADMIN` capability granted at container creation. This approach works across Docker Desktop, OrbStack, and colima without requiring host-level network changes.

---

## How It Works

### Mechanism

When the firewall is enabled, claustro:

1. Adds `NET_ADMIN` capability to the container's `HostConfig.CapAdd` during creation
2. After the container starts, runs iptables commands via Docker SDK `ContainerExecCreate`/`ContainerExecAttach` (non-interactive, as root)
3. Rules are applied in a specific order to build a whitelist-then-drop policy

### Rule Order

Rules are applied as OUTPUT chain appends (`-A OUTPUT`) in this order:

1. **Established/related connections** — allow return traffic for already-accepted connections
   ```
   -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
   ```

2. **Loopback** — allow all traffic on the loopback interface
   ```
   -A OUTPUT -o lo -j ACCEPT
   ```

3. **DNS** — allow DNS resolution (required for domain-based allowlisting)
   ```
   -A OUTPUT -p udp --dport 53 -j ACCEPT
   -A OUTPUT -p tcp --dport 53 -j ACCEPT
   ```

4. **Docker internal networks** — allow communication with Docker infrastructure (DNS resolver, gateway)
   ```
   -A OUTPUT -d 172.16.0.0/12 -j ACCEPT
   -A OUTPUT -d 192.168.0.0/16 -j ACCEPT
   -A OUTPUT -d 10.0.0.0/8 -j ACCEPT
   ```

5. **Allowlisted domains** — for each domain, resolve to IP addresses and add per-IP rules
   ```
   -A OUTPUT -d <resolved-ip> -j ACCEPT
   ```

6. **Default DROP** — set the OUTPUT chain policy to DROP
   ```
   -P OUTPUT DROP
   ```

### Why iptables Inside the Container

- Works with all Docker runtimes (Docker Desktop, OrbStack, colima) — no host iptables manipulation needed
- `NET_ADMIN` capability is scoped to the container's own network namespace
- Rules survive container pause/unpause but not recreate (which is fine — claustro reapplies on start)
- No need to manage Docker network plugins or custom drivers

---

## Default Allowlist

When the firewall is enabled, these domains are always allowed (in addition to any user-configured domains):

| Domain | Purpose |
|--------|---------|
| `api.anthropic.com` | Claude API |
| `registry.npmjs.org` | npm packages |
| `pypi.org` | Python packages |
| `github.com` | Git operations |
| `api.github.com` | GitHub API (gh CLI) |
| `archive.ubuntu.com` | Ubuntu packages |
| `security.ubuntu.com` | Ubuntu security updates |

---

## Custom Domains

Users add custom domains via `firewall.allow` in `claustro.yaml`:

```yaml
firewall:
  enabled: true
  allow:
    - my-internal-registry.example.com
    - s3.amazonaws.com
```

Custom domains are appended to the default allowlist. They follow the same resolve-to-IP approach.

---

## Config Resolution Order

The firewall enabled/disabled state follows the standard resolution order (last wins):

1. **Spec default:** `false` (firewall disabled)
2. **Config defaults:** `defaults.firewall` in `claustro.yaml`
3. **Config firewall section:** `firewall.enabled` in `claustro.yaml`
4. **CLI flag:** `--firewall` on `claustro up`

The allowlist only comes from the config file (`firewall.allow`). There is no CLI flag for adding individual domains.

---

## Container Creation Changes

When firewall is enabled, the `container.Create` function must add `NET_ADMIN` to the host config:

```go
hostCfg := &containertypes.HostConfig{
    // ... existing fields ...
    CapAdd: []string{"NET_ADMIN"},
}
```

This is the only change to container creation. The capability is needed so iptables commands succeed inside the container.

When firewall is disabled, no capability is added (current behavior).

---

## Firewall Application Flow

After `container.Start()` returns, the firewall is applied:

1. Build the rule set: base rules + resolved IPs for default domains + resolved IPs for custom domains
2. For each iptables command, run it via Docker SDK exec (non-interactive, user `root`, no TTY)
3. Verify by running `iptables -L OUTPUT -n` and checking the policy is DROP
4. Log the number of rules applied and any domains that failed to resolve

### Exec Configuration

Each iptables command runs as a separate exec with:
- `User: "root"` (iptables requires root)
- `Tty: false`
- `AttachStdout: true`, `AttachStderr: true` (to capture errors)
- `Cmd: []string{"iptables", ...args}`

---

## Error Handling

### iptables Not Found

If iptables is not installed in the image, the exec will fail with a non-zero exit code. This is a fatal error — the container should be stopped and removed, and the user told to rebuild the image.

**Mitigation:** Add iptables to the Dockerfile so this cannot happen with the standard image.

### DNS Resolution Failure

If a domain cannot be resolved to any IP address:
- Log a warning with the domain name
- Skip adding rules for that domain
- Continue applying rules for remaining domains
- Do NOT fail the entire firewall setup — partial protection is better than none

### iptables Command Failure

If an individual iptables rule fails to apply:
- Log a warning with the command and error output
- Continue applying remaining rules
- After all rules are attempted, if the DROP policy was not set, return an error (this means the firewall is not active)

### Container Already Running (Reattach)

If `ensureRunning` finds the container is already up, the firewall is NOT reapplied. Rules persist inside the running container. If the user needs to refresh rules (e.g., after config change), they must `burn` and `up` again.

---

## Testing Strategy

### Unit Tests (no Docker required)

Package `internal/firewall`:

- **Rule generation:** Given a list of domains and resolved IPs, verify the correct iptables argument slices are produced in the right order
- **Default allowlist:** Verify the default domain list is complete
- **Merge logic:** Verify custom domains are appended to defaults without duplicates
- **DNS resolution mock:** Use a resolver interface so tests can inject known IP results

### Integration Tests (Docker required, `//go:build integration`)

- Create a container with `NET_ADMIN`, apply firewall rules, verify `iptables -L OUTPUT -n` shows DROP policy
- Verify allowed domain traffic succeeds (curl to api.anthropic.com)
- Verify blocked domain traffic fails (curl to a non-allowlisted host)

---

## Files Touched

| File | Changes |
|------|---------|
| `internal/image/Dockerfile` | Add `iptables` package |
| `internal/firewall/firewall.go` | New package: rule generation and application |
| `internal/firewall/firewall_test.go` | Unit tests for rule generation |
| `internal/config/resolve.go` | Add `Firewall *bool` to `CLIOverrides`, wire resolution |
| `cmd/claustro/up.go` | Add `--firewall` flag, call firewall after start |
| `internal/container/container.go` | Accept firewall flag to conditionally add `NET_ADMIN` |

## Out of Scope

- Per-sandbox allowlist overrides (all sandboxes in a project share the same allowlist)
- IP-based allowlisting (only domain names)
- Ingress filtering (containers do not expose ports by default)
- Rate limiting or bandwidth throttling
- Runtime rule updates without container restart
