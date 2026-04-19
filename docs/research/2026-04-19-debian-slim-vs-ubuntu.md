# Research: `debian:12-slim` vs `ubuntu:24.04` Base Image

> **Status:** Not yet conducted — template only.
> **Owner:** Peter
> **Date queued:** 2026-04-19
> **Related spec:** `docs/specs/2026-04-19-sandbox-resource-overhead-design.md`

## Goal

Decide whether to swap the claustro sandbox base image from `ubuntu:24.04` to `debian:12-slim`. Decision criterion: a positive recommendation requires **>30% size reduction with no functional regression** in the existing sandbox feature set.

## Context

The claustro sandbox image (`internal/image/Dockerfile.tmpl`) currently bases on `ubuntu:24.04`. Anecdotally, `debian:12-slim` has a smaller footprint and fewer pre-installed packages. Smaller images mean faster `docker pull` (one-time per host), less disk used on the host, and potentially smaller idle RSS for the container's PID 1.

This research is a sibling to the resource-overhead reduction work (smart resource defaults + idle auto-pause). The image swap is **not** expected to materially help the user's primary pain (active-iteration CPU); it's an opportunistic disk/cold-start improvement evaluated independently.

## Methodology

### Step 1 — Build both images

In two clean clones (or two worktrees):
- Apply the existing `Dockerfile.tmpl` against base `ubuntu:24.04` (current state).
- Apply a copy of `Dockerfile.tmpl` against base `debian:12-slim`. Adjust apt package names where they differ (e.g. some packages are split differently). Document each adjustment.

Build with the same enabled features on both: default config (Node + dev tools + MCP filesystem/memory/fetch + Claude Code + Codex). Tag `claustro:ubuntu` and `claustro:debian`.

### Step 2 — Capture metrics

For each tag:

| Metric | Command | Captured value |
|--------|---------|----------------|
| On-disk size | `docker image inspect <tag> --format '{{.Size}}'` | bytes |
| Layer count | `docker history <tag> --no-trunc | wc -l` | int |
| Build time (cold, no cache) | `docker build --no-cache --pull` wall time | seconds |
| Build time (warm, second run) | `docker build` wall time | seconds |
| Container start time | `time docker run --rm <tag> /bin/true` (median of 5) | ms |
| Idle RSS at PID 1 | After `docker run -d`, `docker stats --no-stream` after 30 s settle | MiB |
| Cold-start memory peak | `docker stats` sampled at 1 Hz during the first 30 s | MiB |

### Step 3 — Functional verification

Against `claustro:debian`:
1. `claustro up` succeeds (with the test image substituted for the default).
2. `claustro shell` opens a working zsh.
3. `claustro claude` runs (no missing deps).
4. `claustro codex` runs (once PR #36 is merged).
5. Each opt-in language (Go, Rust, Python) installs correctly when enabled.
6. Each MCP server (filesystem, memory, fetch) starts.
7. Voice mode (sox) installs when enabled.
8. Egress firewall via iptables works.
9. Native Claude Code installer succeeds.

Document any package name differences encountered, any features broken, and whether each break is fixable with a small `Dockerfile.tmpl` change.

### Step 4 — Record results

Fill in the **Results** section below. If results meet the criterion, write a separate spec at `docs/specs/<date>-base-image-debian-slim-design.md` proposing the swap. If not, close out this research with a "no-go, reasons" entry and revisit only if circumstances change.

## Decision Criterion

**Swap recommended iff:**
- On-disk size reduction ≥ 30% (compared to `ubuntu:24.04` build).
- All functional verification steps pass (or have trivially fixable breaks documented).
- Build time and start time do not regress beyond +20%.
- No known package licensing or support concerns introduced.

## Results

*(To be filled in after experiment.)*

| Metric | ubuntu:24.04 | debian:12-slim | Δ |
|--------|--------------|----------------|---|
| On-disk size | | | |
| Layer count | | | |
| Cold build time | | | |
| Warm build time | | | |
| Container start time (p50) | | | |
| Idle RSS (30 s) | | | |
| Cold-start memory peak | | | |

### Functional verification

| Feature | Status | Notes |
|---------|--------|-------|
| `claustro up` | | |
| `claustro shell` | | |
| `claustro claude` | | |
| `claustro codex` | | |
| Go runtime | | |
| Rust runtime | | |
| Python runtime | | |
| MCP filesystem | | |
| MCP memory | | |
| MCP fetch | | |
| Voice mode (sox) | | |
| Egress firewall | | |
| Claude native installer | | |

### Package name differences

*(List any apt package name changes between ubuntu and debian-slim that required `Dockerfile.tmpl` edits.)*

### Recommendation

*(go / no-go, with one-paragraph justification.)*

## References

- `internal/image/Dockerfile.tmpl` — current Dockerfile template
- `docs/specs/2026-04-19-sandbox-resource-overhead-design.md` — parent brainstorm that surfaced this question
