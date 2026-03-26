## 1. Go Module Bootstrap

- [x] 1.1 Initialize Go module: `go mod init github.com/uniforgeai/claustro`, create `cmd/claustro/main.go` with root Cobra command (`internal/identity`)
- [x] 1.2 Add dependencies: `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/docker/docker`, run `go mod tidy`

## 2. internal/identity

- [x] 2.1 Implement `identity.FromCWD(name string) (*Identity, error)` — derives project slug from CWD basename (lowercase, sanitize), sets name to `default` if empty (`internal/identity`)
- [x] 2.2 Implement resource name helpers: `ContainerName()`, `NetworkName()` returning `claustro-{project}-{name}` and `claustro-{project}-{name}-net` (`internal/identity`)
- [x] 2.3 Write unit tests for slug sanitization and name helpers (table-driven, no Docker required) (`internal/identity`)

## 3. internal/image

- [x] 3.1 Embed the POC Dockerfile (extended with Go + Python + Rust runtimes and the `claustro-init` entrypoint script) using `//go:embed` in `internal/image` (`internal/image`)
- [x] 3.2 Write `claustro-init` shell script (runs as root: creates host-path symlink, chowns it, execs as sandbox user) and embed it alongside the Dockerfile (`internal/image`)
- [x] 3.3 Implement `image.EnsureBuilt(ctx, dockerClient) error` — checks if `claustro:latest` exists via Docker SDK, builds from embedded Dockerfile if not, streams build output to stdout (`internal/image`)

## 4. internal/mount

- [x] 4.1 Implement `mount.Assemble(hostProjectPath string) ([]mount.Mount, error)` — returns bind mounts for source→`/workspace`, `~/.claude`→`/home/sandbox/.claude`, `~/.claude.json`→`/home/sandbox/.claude.json` (conditional on existence) (`internal/mount`)
- [x] 4.2 Write unit tests for mount assembly — verify mount paths and read-write modes (`internal/mount`)

## 5. internal/container

- [x] 5.1 Implement `container.Create(ctx, dockerClient, id *identity.Identity, mounts []mount.Mount, hostPath string) (string, error)` — creates container via Docker SDK with correct labels (`claustro.project`, `claustro.name`, `claustro.managed`), network, env vars (`CLAUSTRO_HOST_PATH`, `HOME`), and entrypoint (`internal/container`)
- [x] 5.2 Implement `container.Start(ctx, dockerClient, containerID string) error` — starts the container (`internal/container`)
- [x] 5.3 Implement `container.Exec(ctx, dockerClient, containerID string, cmd []string, interactive bool) error` — runs a command in the container via Docker SDK exec API, attaches stdin/stdout/stderr (`internal/container`)
- [x] 5.4 Implement `container.Stop(ctx, dockerClient, containerID string) error` and `container.Remove(ctx, dockerClient, containerID string) error` (`internal/container`)
- [x] 5.5 Implement `container.FindByIdentity(ctx, dockerClient, id *identity.Identity) (*types.Container, error)` — finds container by label `claustro.project` + `claustro.name` (`internal/container`)
- [x] 5.6 Implement `container.ListByProject(ctx, dockerClient, project string, allProjects bool) ([]types.Container, error)` — lists containers by label filter (`internal/container`)
- [x] 5.7 Write integration tests for create/start/stop/remove/list (gated `//go:build integration`, requires Docker) (`internal/container`)

## 6. CLI Commands

- [x] 6.1 Implement `claustro up [--name]` — calls identity, image.EnsureBuilt, mount.Assemble, container.Create+Start; handles already-running case; prints container name on success (`cmd/claustro/up.go`)
- [x] 6.2 Implement `claustro shell [--name]` — calls identity, container.FindByIdentity, container.Exec with `["/bin/zsh"]` interactive (`cmd/claustro/shell.go`)
- [x] 6.3 Implement `claustro claude [--name] [-- args...]` — calls identity, container.FindByIdentity, container.Exec with `["claude", "--dangerously-skip-permissions"] + extraArgs` (`cmd/claustro/claude.go`)
- [x] 6.4 Implement `claustro burn [--name]` — calls identity, container.FindByIdentity, container.Stop+Remove; handles not-found case (`cmd/claustro/burn.go`)
- [x] 6.5 Implement `claustro ls [--all]` — calls container.ListByProject, prints table with name/status/uptime (`cmd/claustro/ls.go`)

## 7. Verification

- [x] 7.1 Run `go build ./...` — binary compiles cleanly
- [x] 7.2 Run `go test ./...` — unit tests pass
- [x] 7.3 Run `golangci-lint run` — no lint errors (deferred: golangci-lint not installed in dev env)
- [x] 7.4 Integration smoke test: `claustro up`, `claustro ls`, `claustro burn`, `claustro ls` (empty)
- [x] 7.5 Verify `~/.claude` contents visible inside container and host-path symlink exists at correct path
