## Why

When Claude Code runs inside a claustro sandbox, pasting a screenshot or image into the Claude Code chat does not work. The `claude` (and `shell`) commands start an interactive exec session but pass no terminal environment variables to the container. As a result, `TERM` is unset, terminal capability detection fails, and Claude Code's image/clipboard input never activates.

A secondary issue: the exec session's I/O is piped with plain `io.Copy` rather than Docker's `stdcopy`, which may corrupt or drop multi-byte escape sequences (including the OSC 52 clipboard protocol that terminal paste relies on).

## What Changes

- **Forward terminal env vars**: Pass `TERM`, `COLORTERM`, and `LANG`/`LC_ALL` from the host into the Docker exec environment so Claude Code can correctly detect terminal capabilities.
- **Review I/O piping**: Evaluate whether the raw `io.Copy` on the exec connection drops OSC 52 sequences and fix if so.

## Capabilities

### Modified Capabilities

- `sandbox-lifecycle`: The `claude` and `shell` interactive exec sessions will correctly propagate terminal environment, enabling Claude Code's image/clipboard paste feature.

## Impact

- `internal/container/container.go`: Pass terminal env vars in `ExecOptions.Env` for interactive sessions.
- No new external dependencies required.
