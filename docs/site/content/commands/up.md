---
title: claustro up
weight: 2
---

# claustro up

Create and start a sandbox container.

## Usage

```bash
claustro up [flags]
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--name` | Sandbox name | auto-generated |
| `--workdir` | Working directory | project root |
| `--mount` | Additional bind mount (repeatable) | none |
| `--env` | Environment variable KEY=VALUE (repeatable) | none |
| `--readonly` | Mount source read-only | false |
| `--firewall` | Enable egress firewall | false |
| `--isolated-state` | Use Docker volume for ~/.claude | false |

## Examples

```bash
claustro up
claustro up --name api --workdir ./services/api
claustro up --mount ./libs:/workspace/libs:ro --firewall
```
