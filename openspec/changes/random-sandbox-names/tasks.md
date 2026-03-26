## 1. Name generator

- [x] 1.1 Create `internal/identity/names.go` with `var adjectives []string` (~50 words) and `var nouns []string` (~50 words)
- [x] 1.2 Implement `RandomName() string` — picks one adjective and one noun using `math/rand`, joins with `_`
- [x] 1.3 Write unit tests: verify output matches `^[a-z]+_[a-z]+$` pattern, verify repeated calls produce valid names (`internal/identity/names_test.go`)

## 2. `identity.FromCWD` — replace "default" fallback

- [x] 2.1 In `internal/identity/identity.go`, change the `name == ""` branch in `fromPath` to call `RandomName()` instead of `"default"`
- [x] 2.2 Update `internal/identity/identity_test.go` — tests that expected `"default"` for empty name now verify the `adjective_noun` pattern instead

## 3. `up` command — collision check + print generated name

- [x] 3.1 In `cmd/claustro/up.go`, after `identity.FromCWD`, if the original `name` arg was `""` (user gave no `--name`), check for collision: call `container.FindByIdentity` and if found, retry `RandomName()` up to 5 times before returning an error
- [x] 3.2 Print the generated name prominently in `runUp` output:
  ```
  Sandbox started: claustro-myapp-happy-panda
    Name: happy_panda  (use --name happy_panda to target it)
    Run: claustro shell --name happy_panda
    Run: claustro claude --name happy_panda
  ```
- [x] 3.3 Update `--name` flag description to `Sandbox name (default: auto-generated)`

## 4. Auto-select helper

- [x] 4.1 Create `cmd/claustro/resolve.go` with `resolveName(ctx context.Context, cli *client.Client, project, name string) (string, error)`:
  - If `name != ""` → return name unchanged
  - Call `container.ListByProject(ctx, cli, project, false)`
  - 0 results → error: `"no sandboxes running for project %q — run: claustro up"`
  - 1 result → return `c.Labels["claustro.name"]`
  - 2+ results → error listing names: `"multiple sandboxes running, specify --name:\n  <name1>\n  <name2>"`
- [x] 4.2 Add unit tests for `resolveName` covering all three cases

## 5. Wire auto-select into targeting commands

- [x] 5.1 `cmd/claustro/shell.go`: call `resolveName` to resolve name before `identity.FromCWD`; update `--name` flag description
- [x] 5.2 `cmd/claustro/claude.go`: same
- [x] 5.3 `cmd/claustro/exec.go`: same
- [x] 5.4 `cmd/claustro/burn.go` (single-target path): same
- [x] 5.5 `cmd/claustro/status.go`: same
- [x] 5.6 `cmd/claustro/logs.go`: same

## 6. Verify

- [x] 6.1 `go build ./...` passes with no errors
- [x] 6.2 `go test ./...` passes
- [ ] 6.3 `golangci-lint run` passes with no new warnings
- [ ] 6.4 Manual smoke: `claustro up` prints a generated `adjective_noun` name; `claustro ls` shows it; `claustro shell` (no `--name`, single sandbox) auto-selects; `claustro burn` removes it
- [ ] 6.5 Manual smoke: two sandboxes running → `claustro shell` (no `--name`) prints helpful error listing both names
