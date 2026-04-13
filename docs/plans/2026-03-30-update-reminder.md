# Update Reminder

**Status:** DONE
**Date:** 2026-03-30

## Problem

Users running outdated versions of claustro may miss important fixes or features and have no prompt to upgrade.

## Requirements

- Periodically check for newer releases on GitHub (e.g. via GitHub Releases API)
- Display a non-intrusive reminder when a newer version is available
- Do not block or delay normal command execution
- Cache the check result locally to avoid hitting the API on every invocation

## Open Questions

- How often should the check run? (e.g. once per day, once per session)
- Where to cache the last-check timestamp? (`~/.config/claustro/` or `~/.claustro/`)
- Should the check be opt-out via config or env var?
- Should it be a background goroutine or a pre-run hook?
