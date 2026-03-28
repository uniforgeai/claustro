# Change: Auto-Up on Claude Command

## Summary

When the user runs `claustro claude`, automatically bring up a sandbox if none is running, instead of failing with an error. This removes a friction step — users shouldn't need to remember to run `up` before `claude`.

## Motivation

The most common workflow is: `claustro claude`. If the container isn't running, the user currently gets an error and has to manually run `claustro up` first. This is unnecessary friction. The `claude` command should silently call `up` logic when no sandbox exists, then proceed to launch Claude Code.

## Behavior

- **GIVEN** no sandbox is running for the current project
- **WHEN** the user runs `claustro claude`
- **THEN** a sandbox is silently created and started (same as `claustro up`)
- **AND** Claude Code is launched inside it
- **AND** the auto-up output is minimal (no banner, just a brief note like "Starting sandbox...")

- **GIVEN** a sandbox is already running
- **WHEN** the user runs `claustro claude`
- **THEN** behavior is unchanged — Claude Code launches in the existing sandbox

- **GIVEN** `--name` is specified and that sandbox is not running
- **WHEN** the user runs `claustro claude --name foo`
- **THEN** a sandbox named `foo` is created and started, then Claude Code launches in it

## Scope

- Modify `claude.go` to detect "no running sandbox" and invoke up logic before exec
- Extract shared up logic so it can be called from both `up.go` and `claude.go`
- Keep the auto-up output quiet (suppress the banner/help text that `up` normally prints)

## Out of Scope

- Changing `shell` or `exec` commands to auto-up (can be a follow-up)
- Adding a `--no-auto-up` flag (premature — add if users ask)
