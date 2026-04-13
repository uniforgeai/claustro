# Session Save & Resume — Design Spec

> **Status:** Draft
> **Date:** 2026-04-06
> **Milestone:** TBD (post-M5)

## Problem

When a user exits Claude Code inside a claustro sandbox (`/exit`), the session ID is printed but lost. Resuming that session requires the user to manually copy the session ID and pass it to `claude --resume <id>`. This is friction-heavy, especially when juggling multiple sandboxes or returning to work after a break.

## Goal

claustro should automatically capture session IDs on exit and provide a simple way to resume any previous session when starting Claude Code again.

## User Stories

### US-1: Auto-capture session on exit

- **GIVEN** a user runs `claustro claude` and later exits the session
- **WHEN** Claude Code prints a session ID at `/exit`
- **THEN** claustro captures and stores the session ID along with metadata (sandbox name, timestamp, project)

### US-2: Resume last session

- **GIVEN** stored sessions exist for the current sandbox
- **WHEN** the user runs `claustro claude --resume` (no ID)
- **THEN** the most recent session is resumed automatically

### US-3: Resume specific session by ID

- **GIVEN** stored sessions exist for the current sandbox
- **WHEN** the user runs `claustro claude --resume <session-id>`
- **THEN** that specific session is resumed

### US-4: Interactive session picker

- **GIVEN** multiple stored sessions exist
- **WHEN** the user runs `claustro claude --resume` and there are multiple sessions
- **THEN** an interactive picker is shown with session ID, quit time, and sandbox name
- **AND** the user can select which session to resume

### US-5: List sessions

- **GIVEN** stored sessions exist
- **WHEN** the user runs `claustro sessions` (or `claustro claude --list-sessions`)
- **THEN** all stored sessions are listed with ID, sandbox name, project, and quit time

## High-Level Design

### Session capture

When `claustro claude` runs Claude Code inside the container, claustro wraps the exec and captures stdout/stderr on exit. If the output contains a session ID pattern (to be determined — Claude Code's exact format needs investigation), claustro stores it.

### Session storage

```
~/.config/claustro/sessions.json
```

```json
[
  {
    "session_id": "abc123-def456",
    "project": "claustro",
    "sandbox": "default",
    "exited_at": "2026-04-06T14:30:00Z"
  }
]
```

Scoped per project + sandbox. Older sessions can be pruned (e.g., keep last 20 per sandbox).

### Resume flow

`claustro claude --resume [session-id]` translates to:

```
claude --resume <session-id> --dangerously-skip-permissions
```

inside the container.

## Open Questions

1. **Session ID format** — What exactly does Claude Code print on `/exit`? Need to capture the exact pattern for reliable parsing.
2. **Capture mechanism** — Pipe stdout through a tee-like mechanism, or parse exit output? Must not interfere with the interactive TTY.
3. **Session validity** — Can a session expire or become invalid? Should claustro validate before attempting resume?
4. **Cross-sandbox sessions** — Should a session started in sandbox `api` be resumable from sandbox `web`? (Probably not, but worth deciding.)
5. **`--continue` vs `--resume`** — Claude Code may have `--continue` for the last session. Should claustro expose both patterns?
6. **Storage location** — `~/.config/claustro/sessions.json` or inside `~/.claude/` (which is bind-mounted and shared)?
7. **Pruning policy** — How many sessions to keep? Time-based or count-based?

## Out of Scope (for now)

- Session naming/tagging (e.g., `claustro claude --resume "refactor auth"`)
- Session sharing between users
- Session export/import
- Automatic resume on `claustro up` (always starts fresh unless `--resume`)
