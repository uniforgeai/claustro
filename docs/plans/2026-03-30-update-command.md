# Update Command

**Status:** TODO
**Date:** 2026-03-30

## Problem

Users need a simple way to update claustro to the latest version regardless of how they installed it.

## Requirements

- Add a `claustro update` command that upgrades to the latest release
- Detect install method (Homebrew, binary, go install) and use the appropriate update mechanism
- Show current version → new version in output

## Open Questions

- For binary installs: self-replace the binary, or download and prompt?
- Should it support updating to a specific version (`claustro update v0.2.0`)?
- How to handle permission issues (e.g. binary in `/usr/local/bin` without write access)?
- For Homebrew: just shell out to `brew upgrade claustro`, or handle differently?
