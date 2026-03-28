# Tasks: Validate Mount Security

## 1. Internal validate package — types and rules

**Package:** `internal/validate`

- [ ] 1.1 Create `internal/validate/` package with `Severity`, `Finding`, `Report` types
- [ ] 1.2 Implement blocked mount rules: `RuleNoDockerSocket`, `RuleNoEtc`, `RuleNoProcSys`, `RuleNoRootFS`
- [ ] 1.3 Implement error mount rules: `RuleNoHomeDir`, `RuleNoVar`, `RuleNoCredentialPaths`, `RuleNoTmp`
- [ ] 1.4 Implement warning mount rules: `RuleSSHReadOnly`, `RuleBroadMount`, `RulePreferReadOnly`
- [ ] 1.5 Implement container security rules: `RuleNoNewPrivileges`, `RuleNotPrivileged`, `RuleNoCapAdd`, `RuleResourceLimits`, `RuleNonRootUser`
- [ ] 1.6 Implement `ValidateMounts`, `ValidateContainer`, `ValidateAll` orchestrators
- [ ] 1.7 Unit tests for all mount rules (table-driven: safe mounts pass, dangerous mounts caught)
- [ ] 1.8 Unit tests for all container security rules

## 2. Live inspection support

**Package:** `internal/validate`

- [ ] 2.1 Implement `FromContainerInspect` to extract mounts and security config from `types.ContainerJSON`
- [ ] 2.2 Unit tests for `FromContainerInspect` with mock container JSON

## 3. Validate command

**Package:** `cmd/claustro`

- [ ] 3.1 Create `cmd/claustro/validate.go` with Cobra command, `--live` and `--name` flags
- [ ] 3.2 Pre-flight mode: resolve config and mount list, run `ValidateAll`, print report
- [ ] 3.3 Live mode: inspect running container via Docker SDK, run `ValidateAll`, print report
- [ ] 3.4 Output formatting with status indicators (✓/✗/⚠), summary line, `NO_COLOR` support

## 4. Verification

- [ ] 4.1 `go build ./...` passes
- [ ] 4.2 `go test ./...` passes
- [ ] 4.3 `golangci-lint run` passes
