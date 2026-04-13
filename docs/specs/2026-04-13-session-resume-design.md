# Session Resume — Design Spec

> **Status:** Approved
> **Date:** 2026-04-13
> **Milestone:** Post-M5

## Problem

When a user exits Claude Code inside a claustro sandbox, the session is lost unless they manually copy the session ID and pass it to `claude --resume <id>`. This is friction-heavy, especially when returning to work after a break or juggling multiple sandboxes.

## Goal

Add a `--resume` flag to `claustro claude` that discovers past sessions from Claude Code's own storage, presents them in an interactive TUI picker, and resumes the selected session — no session ID memorization required.

## Decisions

- **Capture strategy:** Read Claude Code's session files directly from `~/.claude/projects/` (option A from the original draft). No stdout interception or PTY tee-ing.
- **Scope:** Project-scoped. All sessions for the current project are shown regardless of which sandbox created them, since `~/.claude/` is shared across sandboxes via bind mount.
- **Picker:** Interactive TUI using bubbletea (charmbracelet). Arrow-key navigation, enter to select, esc to cancel.
- **Entry point:** `claustro claude --resume`. No standalone `claustro sessions` command for now.
- **Default behavior unchanged:** `claustro claude` without `--resume` always starts a fresh session.

## Claude Code Session Storage

Claude Code stores sessions at:

```
~/.claude/projects/{encoded-path}/*.jsonl
```

Where `{encoded-path}` is the absolute workspace path with `/` replaced by `-`. Inside claustro containers, the workspace is always `/workspace`, so the lookup path is:

```
~/.claude/projects/-workspace/
```

Each session is a JSONL file named by its UUID (e.g., `012b6cdd-0491-4ee1-8995-06c1f6978db7.jsonl`). Each line is a JSON record. Relevant record types:

- **First line:** contains `sessionId` (UUID) and `timestamp` (ISO-8601) — used for session start time
- **`custom-title` record:** `{"type":"custom-title","customTitle":"...","sessionId":"..."}` — human-readable title
- **File modification time:** used as "last activity" timestamp

The `--resume` flag in Claude Code expects a bare UUID matching the JSONL filename.

## Design

### Package: `internal/session`

Session discovery and parsing.

```go
// Session represents a discovered Claude Code session.
type Session struct {
    ID        string    // UUID (filename without .jsonl)
    Title     string    // from custom-title record, or "(untitled)"
    StartedAt time.Time // timestamp from first JSONL record
    UpdatedAt time.Time // file modification time
}

// List discovers sessions for the given project.
// claudeDir is the path to ~/.claude on the host.
// Returns sessions sorted by UpdatedAt descending (most recent first).
func List(claudeDir string) ([]Session, error)
```

Implementation:
1. Construct the path: `{claudeDir}/projects/-workspace/`
2. Glob for `*.jsonl` files
3. For each file:
   - Extract UUID from filename
   - Read the first JSON line → parse `timestamp` field → `StartedAt`
   - Scan lines for `{"type":"custom-title",...}` → extract `customTitle` → `Title`
   - Stat the file → `UpdatedAt`
4. Sort by `UpdatedAt` descending
5. Return the list

Parsing is intentionally minimal — we only read what we need for the picker and skip everything else. If Claude Code changes its conversation record format, this package is unaffected as long as the first-line timestamp and custom-title records remain.

### Package: `internal/picker`

Interactive TUI session picker using bubbletea.

```go
// PickSession presents an interactive picker and returns the selected session.
// Returns nil if the user cancels (esc/q).
func PickSession(sessions []session.Session) (*session.Session, error)
```

Display format:
```
Resume a session (claustro):

  ▸ refactoring auth middleware          2 hours ago    012b6cdd
    fix volume naming bug                yesterday      a3f8e21c
    add firewall tests                   3 days ago     7bc40155

  ↑/↓ navigate • enter select • esc cancel
```

Each row shows:
- Title (or "(untitled)" if no custom-title record)
- Relative timestamp from `UpdatedAt` (e.g., "2 hours ago", "yesterday", "3 days ago")
- Truncated session ID (first 8 characters)

Controls: `↑`/`↓` to navigate, `enter` to select, `esc`/`q` to cancel.

The picker is generic — it takes a session list and returns a selection. No business logic.

### CLI: `cmd/claustro/claude.go`

New `--resume` flag (bool) on the `claude` command.

Flow when `--resume` is passed:

1. Resolve sandbox identity (same as today)
2. Ensure sandbox is running (auto-up if needed)
3. Determine the host `~/.claude` path (already known from mount assembly)
4. Call `session.List(claudeDir)` to discover sessions
5. If no sessions found → print "No sessions found for this project" to stderr and exit
6. If sessions found → call `picker.PickSession(sessions)`
7. If user selects a session → exec `claude --dangerously-skip-permissions --resume <session-id>`
8. If user cancels (esc) → exit without launching

When `--resume` is not passed, behavior is unchanged — a fresh session starts.

No changes to `container.Exec` or any other existing package.

## New Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — list component
- `github.com/charmbracelet/lipgloss` — styling

## Testing

### `internal/session`

Unit tests with fixture JSONL files:
- Session with a custom-title → title is extracted
- Session without custom-title → title is "(untitled)"
- Multiple sessions → sorted by UpdatedAt descending
- Empty directory → returns empty slice, no error
- Malformed first line → session is skipped with a warning log

### `internal/picker`

Light tests using bubbletea's test program:
- Send `enter` on first item → returns that session
- Send `esc` → returns nil
- Send `↓` then `enter` → returns second session

### CLI integration

No new unit tests for `claude.go` — the flag wiring is thin orchestration. The two packages carry the testable logic.

## Out of Scope

- Standalone `claustro sessions` command
- Session pruning / deletion
- Session naming / tagging
- Cross-project session listing
- Automatic resume prompt on `claustro claude` (without `--resume`)
