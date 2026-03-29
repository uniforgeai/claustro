---
title: Quick Start
weight: 2
---

# Quick Start

## 1. Initialize your project

```bash
cd ~/projects/my-app
claustro init
```

This walks you through a setup wizard and creates `claustro.yaml`. Use `claustro init -y` to accept all defaults.

## 2. Start a sandbox

```bash
claustro up
```

This builds the Docker image (first run only) and starts a sandbox container with your source code mounted at `/workspace`.

## 3. Launch Claude Code

```bash
claustro claude
```

Claude Code starts inside the sandbox with `--dangerously-skip-permissions` — safe because it's in a disposable container.

## 4. Clean up

```bash
claustro burn      # stop and remove container (keeps image)
claustro nuke      # also remove cache volumes
```

## Multiple sandboxes

```bash
claustro up --name api
claustro up --name web
claustro claude api
claustro claude web
claustro ls            # list running sandboxes
```
