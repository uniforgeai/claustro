## 1. Update `internal/config`

- [x] 1.1 In `internal/config/config.go`, change `"sandbox.yaml"` → `"claustro.yaml"` in `Load()` (filepath.Join line and both error message strings)
- [x] 1.2 In `internal/config/config_test.go`, replace all `"sandbox.yaml"` filename strings with `"claustro.yaml"`

## 2. Update spec and openspec change artifacts

- [x] 2.1 In `openspec/specs/spec.md`, replace all occurrences of `sandbox.yaml` with `claustro.yaml` (config structure section, scenarios, milestone descriptions)
- [x] 2.2 In `openspec/changes/image-customization/proposal.md`, replace `sandbox.yaml` references with `claustro.yaml`
- [x] 2.3 In `openspec/changes/image-customization/tasks.md`, replace `sandbox.yaml` references with `claustro.yaml`
- [x] 2.4 In `openspec/changes/git-github-integration/proposal.md`, replace `sandbox.yaml` references with `claustro.yaml`

## 3. Verify

- [x] 3.1 `go build ./...` passes with no errors
- [x] 3.2 `go test ./...` passes (config tests use `claustro.yaml`)
- [x] 3.3 `grep -r "sandbox.yaml" /workspace --include="*.go"` returns no results
- [x] 3.4 `grep -r "sandbox.yaml" /workspace/openspec` returns no results
