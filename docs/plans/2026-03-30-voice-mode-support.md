# Voice Mode Support in Sandbox Image

**Status:** TODO
**Date:** 2026-03-30

## Problem

Claude Code's `/voice` command requires SoX (`sox`) for audio recording. The current sandbox image does not include SoX, so `/voice` fails inside claustro containers.

## Requirements

- Install SoX in the sandbox Docker image
- Investigate audio device passthrough from host to container
- Ensure `/voice` works end-to-end inside a claustro sandbox

## Open Questions

- Does the container need access to the host's audio device (e.g. PulseAudio socket, ALSA device)?
- What are the minimal SoX dependencies needed (just `sox`, or also `libsox-fmt-*` packages)?
- Is there a cross-platform approach that works on both macOS and Linux hosts?
