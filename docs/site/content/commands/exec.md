---
title: claustro exec
weight: 9
---

# claustro exec

Run a command inside a running sandbox.

## Usage

```bash
claustro exec [name] -- <command> [args...]
```

## Flags

| Flag | Description |
|------|-------------|
| `--name` | Sandbox name |

## Examples

```bash
claustro exec -- go test ./...
claustro exec api -- npm run build
```
