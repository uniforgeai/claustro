---
title: Egress Firewall
weight: 2
---

# Egress Firewall

Restrict outbound network access from sandboxes.

## Enable

```bash
claustro up --firewall
```

Or in `claustro.yaml`:

```yaml
firewall:
  enabled: true
  allow:
    - custom-registry.company.com
    - api.openai.com
```

## Default whitelist

When enabled, the following domains are always allowed:
- `api.anthropic.com` — Anthropic API
- `registry.npmjs.org` — npm
- `pypi.org` — Python packages
- `github.com` — Git operations
- `archive.ubuntu.com`, `security.ubuntu.com` — System updates

Docker internal networks (172.16.0.0/12, 192.168.0.0/16, 10.0.0.0/8) are always allowed for compose-sibling services.
