## Approach

### 1. Forward terminal environment variables into exec

When `interactive=true`, read the host values of `TERM`, `COLORTERM`, `LANG`, and `LC_ALL` and inject them into the exec `Env` slice. Fall back to sensible defaults (`TERM=xterm-256color`) when the host value is empty.

```go
// in Exec(), when interactive:
execCfg := containertypes.ExecOptions{
    ...
    Env: termEnv(), // ["TERM=xterm-256color", "COLORTERM=truecolor", ...]
}
```

Add a `termEnv() []string` helper in `internal/container/terminal.go` that reads env vars from the host and returns a slice suitable for injection.

### 2. I/O piping audit

Claude Code's image paste uses OSC sequences that arrive as multi-byte payloads over stdin. The current `io.Copy(resp.Conn, os.Stdin)` and `io.Copy(os.Stdout, resp.Reader)` are direct byte copies — they should be transparent for a `Tty=true` exec (where Docker does not multiplex). Verify this is the case and document the finding; only patch if a real corruption path is found.

## Key Decisions

- **Scope**: Only interactive execs get env injection. Non-interactive `exec` subcommand is unaffected.
- **Defaults**: If host `TERM` is empty, default to `xterm-256color` rather than passing nothing.
- **No OSC proxy**: A dedicated OSC 52 proxy layer is out of scope; fixing the missing `TERM` var is the minimal correct fix.

## Files Modified

- `internal/container/terminal.go` — add `termEnv() []string` helper
- `internal/container/container.go` — wire `termEnv()` into `Exec` when interactive
