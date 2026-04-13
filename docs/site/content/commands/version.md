---
title: claustro version
weight: 16
---

# claustro version

Print the version of claustro, including the commit hash and build date.

## Usage

```bash
claustro version
```

## Output

```
claustro v0.1.0 (commit: abc1234, built: 2026-03-30)
```

The version, commit, and date are injected at build time via ldflags. Development builds show `claustro dev (commit: none, built: unknown)`.
