## 1. `identity.VolumeName`

- [ ] 1.1 Add `VolumeName(purpose string) string` to `internal/identity/identity.go` — returns `claustro-{project}-{name}-{purpose}`
- [ ] 1.2 Add table-driven unit tests for `VolumeName` in `internal/identity/identity_test.go`

## 2. Volume operations in `internal/container`

- [ ] 2.1 Add `EnsureVolume(ctx, cli, name string, labels map[string]string) error` — checks if volume exists by name, creates it if not (idempotent)
- [ ] 2.2 Add `RemoveVolume(ctx, cli, name string) error` — removes volume by name, ignores not-found errors
- [ ] 2.3 Add unit tests for `EnsureVolume` and `RemoveVolume` in `internal/container/`

## 3. `up`: create and mount cache volumes

- [ ] 3.1 In `cmd/claustro/up.go`, before `container.Create`, call `container.EnsureVolume` for `id.VolumeName("npm")` and `id.VolumeName("pip")` with the sandbox labels
- [ ] 3.2 Append two `mount.Mount` entries (type `volume`) to the mounts slice: `id.VolumeName("npm")` → `/home/sandbox/.npm`, `id.VolumeName("pip")` → `/home/sandbox/.cache/pip`

## 4. `nuke`: remove cache volumes

- [ ] 4.1 In `cmd/claustro/nuke.go`, after removing the container, call `container.RemoveVolume` for `id.VolumeName("npm")` and `id.VolumeName("pip")`
- [ ] 4.2 Verify the existing network removal still occurs (it should — just adding volume removal after it)

## 5. `burn --all`

- [ ] 5.1 Add `--all` flag to `newBurnCmd()` in `cmd/claustro/burn.go`
- [ ] 5.2 When `--all` is set, call `container.ListByProject(ctx, cli, id.Project, false)` and loop Stop+Remove per container; print each burned name
- [ ] 5.3 Return an error if both `--name` and `--all` are provided
- [ ] 5.4 Add unit test for the `--name` + `--all` mutual exclusion check

## 6. Verify

- [ ] 6.1 `go build ./...` passes with no errors
- [ ] 6.2 `go test ./...` passes
- [ ] 6.3 `golangci-lint run` passes with no new warnings
- [ ] 6.4 Manual smoke: `claustro up` → `docker volume ls | grep claustro` shows two volumes; `claustro burn` → volumes still present; `claustro nuke` → volumes removed
- [ ] 6.5 Manual smoke: `claustro up --name a && claustro up --name b && claustro burn --all` → both containers removed, volumes for both preserved
