---
title: Environment Variables
weight: 2
---

# Environment Variables

## Host environment

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Forwarded to sandbox for Claude Code auth |
| `DOCKER_HOST` | Docker daemon connection (if non-default) |

## Sandbox environment

| Variable | Description |
|----------|-------------|
| `HOME` | `/home/sandbox` |
| `PATH` | Includes Go, Rust, npm, pip paths |

## Custom variables

Pass via CLI:

```bash
claustro up --env DATABASE_URL=postgresql://localhost:5432/dev
```

Or in `claustro.yaml`:

```yaml
sandboxes:
  api:
    env:
      DATABASE_URL: postgresql://localhost:5432/dev
```

Or via `.env` file in the project root.
