## Why

When Claude Code runs inside a claustro container (Linux), pasting a clipboard image with Ctrl+V fails with "no image on clipboard". The root cause is that Claude Code's image-paste path on Linux shells out to `xclip` or `wl-paste` — neither of which has access to the host clipboard from inside a container.

Investigation of `cli.js` confirms the exact call chain:

- **check**: `xclip -selection clipboard -t TARGETS -o | grep image/...`
- **save**: `xclip -selection clipboard -t image/png -o > /tmp/claude_cli_latest_screenshot.png`

Both fail silently inside the container because there is no X11 or Wayland socket.

## What Changes

A lightweight **clipboard bridge** built into claustro:

1. When `claustro claude` or `claustro shell` starts an interactive session, a goroutine starts a Unix domain socket server on the host.
2. The socket file is bind-mounted into the container at a known path (`/run/claustro/clipboard.sock`).
3. The container image ships shim scripts for `xclip` and `wl-paste` that proxy requests to the socket via `curl`.
4. The host-side server reads the clipboard using platform-native tools: `osascript` on macOS, `xclip`/`wl-paste` with forwarded `DISPLAY`/`WAYLAND_DISPLAY` on Linux, `powershell` on Windows.
5. The server shuts down when the interactive session ends.

## Capabilities

### Modified Capabilities

- `sandbox-lifecycle`: `claude` and `shell` commands now start/stop the clipboard bridge server alongside the exec session.

### New Capabilities

- `clipboard-bridge`: Unix socket server that exposes host clipboard to the container over a simple HTTP API.

## Impact

- `internal/clipboard/` — new package: HTTP-over-Unix-socket server, platform handlers
- `internal/mount/mount.go` — add socket file mount when bridge is active
- `internal/container/container.go` — start/stop bridge goroutine in `Exec()` when interactive
- `internal/image/Dockerfile` — add `xclip` and `wl-paste` shim scripts
- No new external Go dependencies (stdlib `net/http`, `os/exec`)
