---
title: Custom Image
weight: 4
---

# Custom Image Configuration

## Language selection

Choose which language runtimes to include. Node.js is always installed (required for Claude Code).

```yaml
image:
  languages:
    node: true
    go: true
    rust: false
    python: true
```

## Tool groups

```yaml
image:
  tools:
    dev: true
    build: true
```

## Extra Dockerfile steps

```yaml
image:
  extra:
    - run: apt-get update && apt-get install -y ffmpeg
    - run: pip install black ruff
```

## Using a completely custom image

```yaml
image: my-registry/my-image:latest
```

The custom image must have Claude Code pre-installed.
