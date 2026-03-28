## Tasks

### 1. Expand Config struct to full schema

**Package:** `internal/config`

- [x] 1.1 Add top-level fields to `Config`: `Project string`, `ImageName string` (the `image:` shorthand), `Defaults DefaultsConfig`, `Sandboxes map[string]SandboxDef`, `Firewall FirewallConfig`, `MCP MCPConfig`.
- [x] 1.2 Define `DefaultsConfig` struct: `Firewall *bool`, `ReadOnly *bool`, `Resources ResourcesConfig`.
- [x] 1.3 Define `ResourcesConfig` struct: `CPUs string`, `Memory string`.
- [x] 1.4 Define `SandboxDef` struct: `Workdir string`, `Mounts []string`, `Env map[string]string`.
- [x] 1.5 Define `FirewallConfig` struct: `Enabled *bool`, `Allow []string`.
- [x] 1.6 Define `MCPConfig` struct: `Stdio map[string]MCPStdio`, `SSE map[string]MCPSSE`.
- [x] 1.7 Define `MCPStdio` struct: `Command string`, `Args []string`. Define `MCPSSE` struct: `Image string`, `Env map[string]string`.
- [x] 1.8 Handle the dual `image:` syntax: top-level string sets `ImageName`, nested `image.extra` populates `ImageConfig.Extra`. Use a custom `UnmarshalYAML` on the image field.
- [x] 1.9 Write tests: parse the full example config from spec.md lines 431-473 and assert every field.
- [x] 1.10 Write tests: existing minimal configs (empty, image-extra-only, git-only) still parse correctly.

### 2. Add Mount parsing

**Package:** `internal/config`

- [x] 2.1 Define `Mount` struct: `HostPath string`, `ContainerPath string`, `ReadOnly bool`.
- [x] 2.2 Add `ParseMount(raw string, projectRoot string) (Mount, error)` function. Splits on `:`, resolves relative host paths against `projectRoot`, handles optional `:ro`/`:rw` mode suffix.
- [x] 2.3 Write table-driven tests for `ParseMount`: absolute path, relative path, ro mode, rw mode, missing container path error, too many colons error.

### 3. Add .env file loader

**Package:** `internal/config`

- [x] 3.1 Add `LoadDotenv(projectPath string) (map[string]string, error)` function. Reads `<projectPath>/.env`, returns empty map if file missing.
- [x] 3.2 Parser rules: skip blank lines and `#` comments, split on first `=`, trim whitespace from keys, strip optional surrounding quotes from values.
- [x] 3.3 Write tests: missing file returns empty map, basic key=value, comments and blank lines skipped, quoted values, key with no value, duplicate key (last wins).

### 4. Add Resolve method

**Package:** `internal/config`

- [x] 4.1 Define `SandboxConfig` struct (flat, no pointers): `Name string`, `Workdir string`, `Mounts []Mount`, `Env map[string]string`, `Firewall bool`, `ReadOnly bool`, `CPUs string`, `Memory string`, `ImageName string`.
- [x] 4.2 Define `CLIOverrides` struct: `Name string`, `Workdir string`, `Mounts []string`, `Env map[string]string`.
- [x] 4.3 Add `(c *Config) Resolve(projectRoot string, cli CLIOverrides, dotenv map[string]string) (*SandboxConfig, error)` method. Resolution order: spec defaults -> `defaults:` -> named sandbox -> dotenv -> per-sandbox env -> CLI overrides.
- [x] 4.4 In `Resolve`: look up `cli.Name` in `c.Sandboxes`; if not found and `cli.Name != ""`, that's fine (unnamed sandbox with custom name). Parse all mount strings via `ParseMount`.
- [x] 4.5 Write tests: defaults only, defaults + sandbox override, CLI overrides win, dotenv merging, mount resolution with relative paths.

### 5. Wire CLI flags on `up` command

**Package:** `cmd/claustro`

- [x] 5.1 Add `--workdir` (string), `--mount` (string slice), `--env` (string slice, `KEY=VALUE` format) flags to `upCmd`.
- [x] 5.2 In `up.go` RunE: after `config.Load`, call `config.LoadDotenv`, build `CLIOverrides` from flags, call `cfg.Resolve(...)`.
- [x] 5.3 Pass `SandboxConfig` fields to downstream calls (container create, mount assembly). For now, log resolved config at debug level; actual consumption is a follow-up change.
- [x] 5.4 Write a test in `commands_test.go` verifying flag parsing produces correct `CLIOverrides` values.
