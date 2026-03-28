## 1. Terminal env helper

- [x] 1.1 Add `termEnv() []string` to `internal/container/terminal.go` — reads `TERM`, `COLORTERM`, `LANG`, `LC_ALL` from host environment, defaults `TERM` to `xterm-256color` if unset, returns populated slice

## 2. Wire into Exec

- [x] 2.1 In `internal/container/container.go` `Exec()`: set `execCfg.Env = termEnv()` when `interactive=true`
- [x] 2.2 Unit tests for `termEnv()` covering: host vars set, host vars unset (defaults applied)

## 3. I/O piping audit

- [x] 3.1 Verify that `Tty=true` exec connections are unmultiplexed (raw bytes, not Docker stream framing) — documented in comment in `container.go`; `io.Copy` is correct, no patch needed

## 4. Verify

- [ ] 4.1 Manual test: copy a screenshot on the host, run `claustro claude`, paste the image — confirm Claude Code receives and displays it
