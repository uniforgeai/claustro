## 1. Terminal size helpers

- [x] 1.1 Add `getTerminalSize(fd int) (width, height uint)` to `internal/container/terminal.go` — wraps `term.GetSize`, returns defaults (80, 24) on error
- [x] 1.2 Add `monitorResizeEvents(ctx context.Context, cli client.APIClient, execID string, fd int)` to `internal/container/terminal.go` — listens for `SIGWINCH`, calls `cli.ContainerExecResize` with updated dimensions, stops when ctx is cancelled

## 2. Wire into Exec

- [x] 2.1 In `internal/container/container.go` `Exec()`: after `ContainerExecAttach`, query host terminal size and call `cli.ContainerExecResize` with initial dimensions
- [x] 2.2 Start `monitorResizeEvents` goroutine with a cancellable context, cancel it when the exec session ends
- [x] 2.3 Unit tests for `getTerminalSize` fallback behavior

## 3. Verify

- [ ] 3.1 Manual test: `claustro claude` and `claustro shell` should use full terminal width; resizing the terminal window should update the container's PTY dimensions
