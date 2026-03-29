---
title: claustro init
weight: 1
---

# claustro init

Initialize a new claustro project with an interactive setup wizard.

## Usage

```bash
claustro init [flags]
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--project` | Project name | directory basename |
| `--languages` | Comma-separated: go,rust,python | all enabled |
| `--tools` | Comma-separated: dev,build | all enabled |
| `--mcp` | Comma-separated: filesystem,memory,fetch | all enabled |
| `--cpus` | CPU limit | 4 |
| `--memory` | Memory limit | 8G |
| `--firewall` | Enable egress firewall | false |
| `--readonly` | Mount source read-only | false |
| `-y, --yes` | Accept all defaults | false |

## Examples

```bash
claustro init
claustro init -y
claustro init --languages go,python --cpus 8 --memory 16G
```
