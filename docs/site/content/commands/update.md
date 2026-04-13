---
title: claustro update
weight: 17
---

# claustro update

Update claustro to the latest version. Automatically detects how claustro was installed and uses the appropriate update method.

## Usage

```bash
claustro update
```

## Install method detection

| Method | Detection | Update action |
|--------|-----------|---------------|
| Homebrew | Binary path contains `/Cellar/` or `/homebrew/` | `brew upgrade claustro` |
| go install | Binary path contains `/go/bin/` or `/gopath/bin/` | `go install github.com/uniforgeai/claustro/cmd/claustro@latest` |
| Unknown | Fallback | Prints link to GitHub releases page |

## Background update reminders

claustro automatically checks for new versions every 24 hours. When a newer version is available, a non-intrusive reminder is printed to stderr after command execution:

```
A new version of claustro is available: v0.1.0 -> v0.2.0
Run `claustro update` to upgrade.
```

The check is non-blocking and cached in `~/.config/claustro/update-check.json`. Development builds are never nagged.
