## Why

When Claude Code runs inside a claustro sandbox, the terminal does not use the full width available on the host. The container's PTY defaults to Docker's 80x24, and claustro never communicates the actual host terminal dimensions or forwards resize events. This makes Claude Code's TUI render in a narrow 80-column area regardless of how wide the user's terminal actually is.

## What Changes

- **Initial terminal size**: Query the host terminal dimensions before attaching to the Docker exec session, then call `ContainerExecResize` to set the correct size.
- **Resize forwarding**: Listen for `SIGWINCH` signals on the host and forward updated dimensions to the container exec via `ContainerExecResize`.

## Capabilities

### Modified Capabilities

- `sandbox-lifecycle`: The interactive exec path (`shell`, `claude`) will now correctly propagate terminal dimensions.

## Impact

- `internal/container/container.go`: `Exec` function updated to set initial size and forward resizes.
- `internal/container/terminal.go`: New helpers to get terminal size and handle SIGWINCH.
- No new external dependencies — uses existing `golang.org/x/term` and Docker SDK APIs.
