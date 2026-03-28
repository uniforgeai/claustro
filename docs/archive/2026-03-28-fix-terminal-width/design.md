## Approach

Use the Docker SDK's `ContainerExecResize` API to set the correct terminal dimensions on interactive exec sessions, and forward host `SIGWINCH` signals to keep dimensions in sync.

## Key Decisions

- **Where**: All changes scoped to `internal/container/` — the `Exec` function and `terminal.go` helpers.
- **When to resize**: (1) Immediately after `ContainerExecAttach` returns, set initial size. (2) On each `SIGWINCH` signal, re-query and forward.
- **Non-interactive execs**: Skip all resize logic when `interactive=false` (no TTY allocated).
- **Signal cleanup**: Use `signal.NotifyContext` or `signal.Stop` to clean up the SIGWINCH listener when the exec session ends.

## Flow

1. `Exec()` is called with `interactive=true`
2. Create exec, attach to exec (existing code)
3. Query host terminal size via `term.GetSize(fd)`
4. Call `cli.ContainerExecResize(ctx, execID, container.ResizeOptions{Height: h, Width: w})`
5. Start a goroutine that listens for `SIGWINCH`, re-queries size, and calls `ContainerExecResize` again
6. Existing I/O copy and raw terminal logic continues as before
7. On session end, stop the SIGWINCH listener

## Files Modified

- `internal/container/terminal.go` — add `getTerminalSize(fd)` and `handleResizeEvents(ctx, cli, execID, fd)` helpers
- `internal/container/container.go` — call initial resize + start resize goroutine in `Exec`
