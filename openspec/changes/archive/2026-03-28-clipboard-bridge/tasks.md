## 1. Clipboard server package

- [ ] 1.1 Create `internal/clipboard/server.go` — `Server` struct with `Start(hostSockPath string) error` and `Close() error`; HTTP mux with `/types`, `/image/png`, `/text` routes that delegate to a `PlatformHandler` interface
- [ ] 1.2 Create `internal/clipboard/platform_darwin.go` — implements `PlatformHandler` using `osascript` subprocess for check and read
- [ ] 1.3 Create `internal/clipboard/platform_linux.go` — implements `PlatformHandler` using `xclip` (if `DISPLAY` set) or `wl-paste` (if `WAYLAND_DISPLAY` set) from host environment
- [ ] 1.4 Create `internal/clipboard/platform_windows.go` — implements `PlatformHandler` using `powershell`
- [ ] 1.5 Unit tests for each platform handler (table-driven, mock exec)
- [ ] 1.6 Unit test for `Server` HTTP routes (using `httptest` over a real Unix socket)

## 2. Mount integration

- [ ] 2.1 Update `internal/mount/mount.go` `Assemble()` to accept an optional `extraMounts []mount.Mount` parameter and include them in the returned slice
- [ ] 2.2 Unit tests for extra mount inclusion

## 3. Exec integration

- [ ] 3.1 In `internal/container/container.go` `Exec()`: when `interactive=true`, compute host socket path, call `clipboard.NewServer().Start(hostSockPath)`, defer `server.Close()`
- [ ] 3.2 Pass socket file as extra mount to `mount.Assemble()` (or construct the bind mount directly and add to `HostConfig.Binds`)
- [ ] 3.3 Unit/integration tests for clipboard server lifecycle within Exec

## 4. Container image shims

- [ ] 4.1 Add `xclip` shim script to `internal/image/` — handles `-t TARGETS -o`, `-t image/png -o`, `-t text/plain -o` argument patterns
- [ ] 4.2 Add `wl-paste` shim script to `internal/image/` — handles `-l`, `--type image/png`, no-args patterns
- [ ] 4.3 Update `Dockerfile` to COPY and chmod both shims to `/usr/local/bin/`
- [ ] 4.4 Embed shim files in `image.go` alongside Dockerfile and claustro-init (or COPY in Dockerfile)

## 5. Verify

- [ ] 5.1 Build image: `claustro build` (or `go run ./cmd/claustro build`)
- [ ] 5.2 Manual test on macOS: copy screenshot, run `claustro claude`, press Ctrl+V — confirm image received
- [ ] 5.3 Confirm graceful failure: run shim outside claustro (no socket) — exits 1 without error output
- [ ] 5.4 `go build ./...`, `go test ./...`, `golangci-lint run` all pass
