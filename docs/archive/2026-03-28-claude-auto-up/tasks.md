# Tasks: Auto-Up on Claude Command

## 1. Extract shared up logic
- [ ] 1.1 Create `ensureRunning(ctx, cli, id, nameWasEmpty, quiet bool)` that encapsulates sandbox creation from `runUp()`
- [ ] 1.2 Refactor `runUp()` to call `ensureRunning()` with `quiet=false`
- [ ] 1.3 Verify `go build` and `go test` still pass after refactor

## 2. Wire auto-up into claude command
- [ ] 2.1 In `runClaude()`, replace `errNotRunning` with a call to `ensureRunning(quiet=true)`
- [ ] 2.2 Print a brief "Starting sandbox..." message before auto-up
- [ ] 2.3 After auto-up completes, proceed to exec Claude Code in the new container

## 3. Tests
- [ ] 3.1 Unit test: `ensureRunning` creates container when none exists
- [ ] 3.2 Unit test: `ensureRunning` is a no-op when container is already running
- [ ] 3.3 Verify existing up/claude tests still pass

## 4. Verification
- [ ] 4.1 `go build ./...` passes
- [ ] 4.2 `go test ./...` passes
- [ ] 4.3 `golangci-lint run` passes
