---
title: Configuration
weight: 3
---

# Configuration

claustro uses a `claustro.yaml` file in your project root. Generate one with `claustro init` or create it manually.

## Minimal example

```yaml
project: my-app
```

## Full example

```yaml
project: my-app

image:
  languages:
    node: true
    go: true
    rust: false
    python: true
  tools:
    dev: true
    build: true
  mcp_servers:
    filesystem: true
    memory: true
    fetch: true

defaults:
  readonly: false
  resources:
    cpus: "4"
    memory: 8G

sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
    env:
      DATABASE_URL: postgresql://localhost:5432/dev

firewall:
  enabled: false
  allow:
    - custom-registry.company.com

git:
  forward_agent: true
  mount_gitconfig: true
  mount_gh_config: true
  mount_ssh_dir: false
```

See [claustro.yaml Reference](../reference/claustro-yaml/) for the full schema.
