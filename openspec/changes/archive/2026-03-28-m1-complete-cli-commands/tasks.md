## 1. Makefile

- [x] 1.1 Add `Makefile` at project root with `build`, `run`, `test`, `lint`, `clean` targets

## 2. Internal package extensions

- [x] 2.1 Add `container.Inspect` — wraps `cli.ContainerInspect`, returns `types.ContainerJSON`
- [x] 2.2 Add `container.RemoveNetwork` — removes the per-sandbox network by name
- [x] 2.3 Add `image.Build` — forces a rebuild of `claustro:latest` without the existence check
- [x] 2.4 Add unit tests for `container.Inspect` and `container.RemoveNetwork`

## 3. `exec` command

- [x] 3.1 Create `cmd/claustro/exec.go` — resolves identity, finds container, calls `container.Exec(interactive=false)` with user args
- [x] 3.2 Propagate the command exit code via `os.Exit`

## 4. `status` command

- [x] 4.1 Create `cmd/claustro/status.go` — resolves identity, calls `container.Inspect`, formats output with `text/tabwriter`
- [x] 4.2 Show: name, state, image, uptime, mounts, network, host project path

## 5. `logs` command

- [x] 5.1 Create `cmd/claustro/logs.go` — resolves identity, calls `cli.ContainerLogs` with `--follow`/`--tail` flags
- [x] 5.2 Use `stdcopy.StdCopy` to demultiplex stdout/stderr from the container stream

## 6. `nuke` command

- [x] 6.1 Create `cmd/claustro/nuke.go` — resolves identity, calls `ListByProject`, loops Stop+Remove+RemoveNetwork per container
- [x] 6.2 Support `--all` flag to target all claustro-managed sandboxes across projects

## 7. `rebuild` command

- [x] 7.1 Create `cmd/claustro/rebuild.go` — calls `image.Build` unconditionally
- [x] 7.2 Support `--restart` flag: stop project sandboxes before rebuild, restart after

## 8. Tests

- [x] 8.1 Add unit tests for `exec.go` command flag parsing and identity resolution
- [x] 8.2 Add unit tests for `status.go` output formatting
- [x] 8.3 Add unit tests for `logs.go` flag defaults
- [x] 8.4 Add integration test for `nuke` (gated with `//go:build integration`)

## 9. Verify

- [x] 9.1 `go build ./...` passes with no errors
- [x] 9.2 `go test ./...` passes
- [x] 9.3 `golangci-lint run` passes with no new warnings
- [x] 9.4 Manual smoke test: `make build && ./bin/claustro --help` shows all 10 commands
