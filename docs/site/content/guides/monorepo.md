---
title: Monorepo Setup
weight: 1
---

# Monorepo Setup

Run multiple sandboxes targeting different parts of a monorepo.

## Named sandboxes

```yaml
sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
      - ./proto:/workspace/proto:ro
    env:
      DATABASE_URL: postgresql://localhost:5432/dev

  web:
    workdir: ./packages/frontend
    env:
      API_URL: http://localhost:3000
```

## Running

```bash
claustro up --name api
claustro up --name web
claustro claude api
claustro claude web
claustro ls
```
