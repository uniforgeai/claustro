# Tasks: Doctor Command

## 1. Internal doctor package
- [ ] 1.1 Create `internal/doctor/` package with `CheckResult` and `CheckStatus` types
- [ ] 1.2 Implement `CheckDocker` — ping Docker daemon via SDK, report version
- [ ] 1.3 Implement `CheckDockerSocket` — verify socket path exists and is accessible
- [ ] 1.4 Implement `CheckBaseImage` — check if `claustro-base:latest` exists locally
- [ ] 1.5 Implement `CheckGitConfig` — verify `~/.gitconfig` exists
- [ ] 1.6 Implement `CheckSSHAgent` — verify `SSH_AUTH_SOCK` and list keys
- [ ] 1.7 Implement `CheckGitHubCLI` — run `gh auth status` and parse result
- [ ] 1.8 Implement `CheckClipboard` — platform-specific clipboard tool detection
- [ ] 1.9 Implement `CheckConfigFile` — look for `claustro.yaml` in given directory
- [ ] 1.10 Unit tests for each check function (mock Docker client where needed)

## 2. Doctor command
- [ ] 2.1 Create `cmd/claustro/doctor.go` with Cobra command
- [ ] 2.2 Wire check functions in correct order with skip logic
- [ ] 2.3 Format output with status indicators (✓/✗/⚠/-)
- [ ] 2.4 Print summary line and set exit code
- [ ] 2.5 Respect `NO_COLOR` env var

## 3. Verification
- [ ] 3.1 `go build ./...` passes
- [ ] 3.2 `go test ./...` passes
- [ ] 3.3 `golangci-lint run` passes
