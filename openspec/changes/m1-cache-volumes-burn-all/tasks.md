## 1. `identity.VolumeName`

- [x] 1.1 Add `VolumeName(purpose string) string` to `internal/identity/identity.go` — returns `claustro-{project}-{name}-{purpose}`
- [x] 1.2 Add table-driven unit tests for `VolumeName` in `internal/identity/identity_test.go`

## 2. Volume operations in `internal/container`

- [x] 2.1 Add `EnsureVolume(ctx, cli, name string, labels map[string]string) error` — checks if volume exists by name, creates it if not (idempotent)
- [x] 2.2 Add `RemoveVolume(ctx, cli, name string) error` — removes volume by name, ignores not-found errors
- [x] 2.3 Add unit tests for `EnsureVolume` and `RemoveVolume` in `internal/container/`

## 3. `up`: create and mount cache volumes

- [x] 3.1 In `cmd/claustro/up.go`, before `container.Create`, call `container.EnsureVolume` for `id.VolumeName("npm")` and `id.VolumeName("pip")` with the sandbox labels
- [x] 3.2 Append two `mount.Mount` entries (type `volume`) to the mounts slice: `id.VolumeName("npm")` → `/home/sandbox/.npm`, `id.VolumeName("pip")` → `/home/sandbox/.cache/pip`

## 4. `nuke`: remove cache volumes

- [x] 4.1 In `cmd/claustro/nuke.go`, after removing the container, call `container.RemoveVolume` for `id.VolumeName("npm")` and `id.VolumeName("pip")`
- [x] 4.2 Verify the existing network removal still occurs (it should — just adding volume removal after it)

## 5. `burn --all`

- [x] 5.1 Add `--all` flag to `newBurnCmd()` in `cmd/claustro/burn.go`
- [x] 5.2 When `--all` is set, call `container.ListByProject(ctx, cli, id.Project, false)` and loop Stop+Remove per container; print each burned name
- [x] 5.3 Return an error if both `--name` and `--all` are provided
- [x] 5.4 Add unit test for the `--name` + `--all` mutual exclusion check

## 6. Verify

- [x] 6.1 `go build ./...` passes with no errors
- [x] 6.2 `go test ./...` passes
- [x] 6.3 `golangci-lint run` passes with no new warnings (go vet clean; golangci-lint incompatible with Go 1.26.1)
- [ ] 6.4 Manual smoke: `claustro up` → `docker volume ls | grep claustro` shows two volumes; `claustro burn` → volumes still present; `claustro nuke` → volumes removed
- [ ] 6.5 Manual smoke: `claustro up --name a && claustro up --name b && claustro burn --all` → both containers removed, volumes for both preserved
