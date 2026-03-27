## Architecture

```
macOS/Linux/Windows host                Linux container
┌────────────────────────────┐         ┌──────────────────────────────────┐
│  claustro (Exec goroutine) │         │                                  │
│  ┌────────────────────┐    │         │  Claude Code calls:              │
│  │ clipboard.Server   │◀───┤──mount──│  xclip -t TARGETS -o             │
│  │ net.Listen("unix", │    │         │  xclip -t image/png -o           │
│  │  hostSockPath)     │    │         │  wl-paste -l                     │
│  │                    │    │         │  wl-paste --type image/png       │
│  │ GET /types   →     │    │         │          ↓                       │
│  │   platform check   │    │         │  /usr/local/bin/xclip (shim)     │
│  │ GET /image/png →   │    │         │  /usr/local/bin/wl-paste (shim)  │
│  │   platform read    │    │         │  curl --unix-socket \            │
│  └────────────────────┘    │         │   /run/claustro/clipboard.sock \ │
│                             │         │   http://x/types                 │
└────────────────────────────┘         └──────────────────────────────────┘
```

## Socket API

```
GET /types        → newline-separated MIME types, exit 0 if image present
                    e.g. "image/png\ntext/plain\n"
GET /image/png    → raw PNG bytes (200) or 404 if no image on clipboard
GET /text         → plain text from clipboard (200) or 404
```

## Host-side Platform Handlers

| Platform | Image check | Image read |
|---|---|---|
| darwin | `osascript -e 'the clipboard as «class PNGf»'` | `osascript` write to temp file, read bytes |
| linux  | `xclip -t TARGETS -o` (if DISPLAY set) or `wl-paste -l` (if WAYLAND_DISPLAY set) | `xclip -t image/png -o` or `wl-paste --type image/png` |
| windows | `powershell Get-Clipboard -Format Image` | `powershell` save to temp file |

The server forwards `DISPLAY` and `WAYLAND_DISPLAY` from the host environment when spawning Linux clipboard tools.

## Shim Scripts

Two shim scripts installed in the container image replace the real `xclip` and `wl-paste`.

**`/usr/local/bin/xclip`** — maps xclip argument patterns to socket API calls:
- `-t TARGETS -o` → `GET /types` (exits 0 if response contains an image MIME)
- `-t image/png -o` → `GET /image/png` (streams bytes to stdout)
- `-t text/plain -o` → `GET /text`
- Falls back to exit 1 if socket is absent (graceful non-claustro environments)

**`/usr/local/bin/wl-paste`** — maps wl-paste argument patterns:
- `-l` → `GET /types`
- `--type image/png` → `GET /image/png`
- (no args) → `GET /text`

## Socket Path

| Side | Path |
|---|---|
| Host | `<os.TempDir()>/claustro-<containerID[:12]>-clipboard.sock` |
| Container | `/run/claustro/clipboard.sock` (constant, known to shims) |

The host path uses a temp dir with a container-ID suffix for uniqueness across parallel sandboxes.

## Lifecycle

- `clipboard.Server` is created and started inside `container.Exec()` when `interactive=true`, before the exec attach.
- It is shut down (socket file removed) when `Exec()` returns, via `defer server.Close()`.
- The socket file is passed to `mount.Assemble()` as an extra bind mount for interactive sessions only.

## Key Decisions

- **curl is already in the container image** — no new packages needed.
- **HTTP over Unix socket** — simpler than a custom binary protocol; curl handles it in one line.
- **No persistent daemon** — server starts and stops with the exec session, keeping the design simple.
- **Shim replaces, not wraps** — the shims are installed at the same path real xclip/wl-paste would occupy; no `PATH` tricks needed.
- **Graceful fallback** — shims check for socket existence before curling; fail silently if absent (non-claustro use).

## Files Modified

- `internal/clipboard/server.go` — `Server` type, HTTP handlers, platform dispatch
- `internal/clipboard/platform_darwin.go` — osascript implementation
- `internal/clipboard/platform_linux.go` — xclip/wl-paste host implementation
- `internal/clipboard/platform_windows.go` — powershell implementation
- `internal/mount/mount.go` — accept optional extra bind mounts, add socket mount for interactive execs
- `internal/container/container.go` — start/stop `clipboard.Server` in `Exec()` when interactive
- `internal/image/Dockerfile` — add xclip and wl-paste shim scripts
