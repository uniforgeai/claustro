# POC — Docker + Claude Code Spike

Validates the core assumptions of claustro before writing Go code.

## What This Tests

| # | Question | How |
|---|----------|-----|
| 1 | Does Claude Code run as a non-root user (uid 1000) inside a container? | `whoami`, `claude --version` |
| 2 | Is `~/.claude` correctly visible when bind-mounted from the host? | `ls ~/.claude` inside container |
| 3 | Does `HOME=/home/sandbox` resolve `~/.claude` to the bind-mounted path? | `echo $HOME` |
| 4 | Does source code bind-mount to `/workspace` work? | `ls /workspace` |
| 5 | Do plans/sessions written inside the container persist after it's destroyed? | Manual: create plan, kill, respawn, check |
| 6 | Does Claude Code recognize the project despite the path difference (`/workspace` vs host path)? | Manual: check `~/.claude/projects/` |
| 7 | Does auth persist across container restarts? | Manual: login once, kill, respawn, check |

## How to Run

```bash
bash poc/test-poc.sh
```

This will:
1. Build the Docker image `claustro-poc`
2. Start a container with source + `~/.claude` bind-mounted
3. Run automated checks and print results
4. Leave the container running for manual testing

## Manual Tests

After the automated checks, follow the instructions printed by the script to test persistence and project path mapping.

Record your findings in the **Findings** section below.

## Requirements

- Docker Engine or Docker Desktop running
- Active Claude Code subscription (for manual `claude` tests)

## Findings

### Automated checks
- [x] user is `sandbox` (uid 9999 — uid 1000 conflicts with Ubuntu base image, use 9999+)
- [x] HOME is `/home/sandbox`
- [x] Node.js v24.14.0 installed
- [x] Claude Code 2.1.83 installed
- [x] `/workspace` mounted
- [x] `~/.claude` mounted (full contents visible inside container)

### Issues Found

#### 1. `.claude.json` must also be bind-mounted

Claude Code looks for `~/.claude.json` (at the home directory level) separately from `~/.claude/`.
The file exists at `~/.claude.json` on the host and must be explicitly mounted:

```
~/.claude.json → /home/sandbox/.claude.json
```

Without it: `Claude configuration file not found at: /home/sandbox/.claude.json`

#### 2. Project path mapping mismatch (critical)

Claude Code indexes projects by absolute path. When running inside the container from `/workspace`,
it creates a **new** project entry `-workspace` instead of reusing the host's
`-Users-pepusz-code-claustro` entry. Plans, sessions, and todos created on the host
are NOT visible inside the container and vice versa.

**Root cause:** host path `/Users/pepusz/code/claustro` ≠ container path `/workspace`

**Workaround (validated): Host-path symlink**

At container start, `claustro` should:
1. Extract the host project path (e.g. `/Users/pepusz/code/claustro`)
2. Create the full directory structure inside the container as root
3. Symlink that path to `/workspace`

```bash
mkdir -p /Users/pepusz/code
ln -sf /workspace /Users/pepusz/code/claustro
```

Then Claude Code running from `/Users/pepusz/code/claustro` inside the container
will resolve to the same `~/.claude/projects/` entry as the host session.

The symlink directory tree must be owned by the sandbox user.

#### 3. Non-root UID conflict

Ubuntu 24.04 base image already has a user with uid 1000 (`ubuntu`).
Use uid 9999 (or any high uid) for the `sandbox` user.

### Implementation Notes for Walking Skeleton

When `claustro up` creates a container, it must:
1. Bind mount source → `/workspace`
2. Bind mount `~/.claude` → `/home/sandbox/.claude`
3. Bind mount `~/.claude.json` → `/home/sandbox/.claude.json` (if it exists)
4. As a startup entrypoint step (root), create `mkdir -p <host-path-parent> && ln -sf /workspace <host-path>` inside the container
5. Hand off to the sandbox user

The host path is available from the `claustro` binary (it knows where it was invoked from).
