# Version Command

**Status:** TODO
**Date:** 2026-03-30

## Problem

Users installing claustro via Homebrew or binary download have no way to check which version they're running.

## Requirements

- Add a `claustro version` command that prints the current version, commit hash, and build date
- Version info should be injected at build time via ldflags in GoReleaser
- Output format: `claustro v0.1.0 (commit: abc1234, built: 2026-03-30)`

## Implementation Notes

- GoReleaser already supports ldflags — wire up `main.version`, `main.commit`, `main.date`
- Keep the command thin: parse nothing, print version, exit
