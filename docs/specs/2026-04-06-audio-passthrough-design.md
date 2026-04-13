# Audio Passthrough for Voice Mode — Design Spec

> **Status:** Approved
> **Date:** 2026-04-06
> **Milestone:** Post-M5
> **Depends on:** Voice mode (M5, tools.voice)

## Problem

Claude Code's `/voice` command requires microphone access via SoX (`rec`) or ALSA (`arecord`). The M5 voice mode feature installs these tools in the sandbox image, but Docker containers have no access to the host's audio hardware. On macOS this is especially problematic — Docker runs in a Linux VM that has no microphone at all.

## Goal

Enable Claude Code's `/voice` command to capture audio from the host microphone inside a claustro sandbox container, with zero host-side setup, on both macOS and Linux.

## Approach: Audio Bridge (Clipboard Pattern)

Follow the same architecture claustro uses for clipboard bridging: a host-side server captures audio from the real microphone and streams it to the container via a socket. Container-side shim binaries replace `rec`/`arecord` and read from the bridge instead of a hardware device.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Host Machine                                           │
│                                                         │
│  claustro binary                                        │
│  ├─ Audio Capture Server (internal/audio/)              │
│  │   ├─ macOS: CoreAudio (AudioQueue API via CGO)       │
│  │   └─ Linux: ALSA (libasound2 via CGO)                │
│  │                                                      │
│  │   Streams raw PCM via:                               │
│  │   ├─ TCP @ 127.0.0.1:<ephemeral> (macOS)             │
│  │   └─ Unix socket /run/claustro/audio.sock (Linux)    │
│  │                                                      │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Sandbox Container                                │  │
│  │                                                   │  │
│  │  rec-shim (/usr/local/bin/rec)                    │  │
│  │   ├─ Connects to audio bridge socket              │  │
│  │   ├─ Receives raw PCM stream                      │  │
│  │   └─ Writes WAV file for Claude Code              │  │
│  │                                                   │  │
│  │  Claude Code /voice                               │  │
│  │   └─ Calls `rec` → gets audio from shim           │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Lifecycle

1. `claustro claude` starts an interactive session → if voice mode is enabled, audio bridge server starts
2. Bridge server is idle — mic is **not** opened yet
3. User triggers `/voice` → Claude Code calls `rec output.wav`
4. `rec` shim connects to bridge → sends START → host opens mic, streams PCM
5. User stops recording (SIGINT) → shim sends STOP → host closes mic
6. Shim writes WAV header + PCM data to output file, exits 0
7. Claude Code reads the WAV, sends for transcription
8. Session ends → audio bridge server shuts down

**Key principle:** The host microphone is only active when `rec` is actually called inside the container. Between recordings, no audio data flows.

---

## Audio Protocol

### Wire Format

Fixed format, no negotiation:

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Sample rate | 16,000 Hz | Optimal for speech recognition (Whisper) |
| Channels | 1 (mono) | Speech is mono |
| Bit depth | 16-bit signed LE | Universal PCM format |
| Chunk size | 1,024 frames | ~64ms latency per chunk |

### Message Flow

```
Container (shim)          Host (server)
    │                         │
    │──── START ─────────────►│  shim connects, sends start
    │                         │  host opens mic
    │◄─── PCM chunk ─────────│  1024 frames (2048 bytes)
    │◄─── PCM chunk ─────────│
    │◄─── PCM chunk ─────────│  continuous stream
    │──── STOP ──────────────►│  shim signals end
    │◄─── EOF ───────────────│  host closes mic, closes conn
    │                         │
```

### Commands

- `START` — 5-byte ASCII command. Server opens mic, begins streaming PCM.
- `STOP` — 4-byte ASCII command. Server closes mic, closes connection.
- Error response: 1-byte `0xFF` followed by a UTF-8 error message string, then connection close.

### Shim Output

The `rec` shim collects PCM data and writes a standard WAV file:
- 44-byte RIFF/WAV header (sample rate, channels, bit depth, data size)
- Raw PCM data
- Written to the output filename from the `rec` command arguments

---

## Host-Side Audio Capture

### Package Structure

```
internal/audio/
├── server.go           # Bridge server (TCP/Unix socket, protocol)
├── capture.go          # Platform-agnostic Capturer interface
├── capture_darwin.go   # macOS: CoreAudio via CGO
├── capture_linux.go    # Linux: ALSA via CGO
├── capture_stub.go     # Fallback: returns "unsupported"
└── server_test.go
```

### Capturer Interface

```go
type Capturer interface {
    // Available checks if audio capture is possible (mic accessible, permissions OK).
    Available() error
    // Start opens the microphone and begins capturing.
    Start() error
    // Read fills buf with PCM data. Blocks until data is available.
    Read(buf []byte) (int, error)
    // Stop closes the microphone.
    Stop() error
}
```

### macOS: CoreAudio (capture_darwin.go)

- Build tag: `//go:build darwin`
- CGO links against `AudioToolbox.framework` and `CoreAudio.framework`
- Both frameworks ship with macOS — zero install needed
- Uses AudioQueue API (`AudioQueueNewInput`, `AudioQueueStart`, `AudioQueueStop`)
- Configures capture at 16kHz/mono/16-bit
- `Available()` checks microphone permission status

### Linux: ALSA (capture_linux.go)

- Build tag: `//go:build linux`
- CGO links against `libasound2` (`-lasound`)
- Present on virtually all Linux desktops
- Uses `snd_pcm_open("default", SND_PCM_STREAM_CAPTURE)`
- Configures hardware params: 16kHz, 1 channel, S16_LE
- `Available()` checks if default capture device exists

### Stub (capture_stub.go)

- Build tag: `//go:build !darwin && !linux`
- All methods return `errors.New("audio capture not supported on this platform")`
- Graceful degradation — voice mode tools are installed but bridge won't start

### Server Lifecycle

Mirrors `setupClipboardBridge()` in `container.go`:

```go
func setupAudioBridge(opts ExecOptions) func() {
    if !opts.Interactive || !opts.VoiceMode {
        return noop
    }
    capturer := audio.NewCapturer()
    if err := capturer.Available(); err != nil {
        slog.Warn("audio bridge unavailable", "err", err)
        return noop
    }
    srv := audio.NewServer(capturer)
    if runtime.GOOS == "darwin" {
        port, err := srv.StartTCP("127.0.0.1:0")
        // port injected into exec env as CLAUSTRO_AUDIO_PORT
    } else {
        sockPath := filepath.Join(opts.AudioSockDir, "audio.sock")
        srv.Start(sockPath)
    }
    return func() { srv.Close() }
}
```

---

## Container-Side Shims

### Shim Binaries

Two compiled Go binaries (pure Go, no CGO — they only do socket I/O and WAV writing):

| Binary | Shadows | Purpose |
|--------|---------|---------|
| `rec-shim` | `/usr/local/bin/rec` | Primary — Claude Code's SoX fallback |
| `arecord-shim` | `/usr/local/bin/arecord` | Secondary — Claude Code's ALSA fallback |

Both binaries share the same logic, differing only in argument parsing (SoX-style vs ALSA-style flags).

### Shim Behavior

1. Parse output filename from args (last positional arg for `rec`, `-f` flag for `arecord`)
2. Determine bridge address:
   - Check `CLAUSTRO_AUDIO_SOCK` env var (Unix socket path)
   - Else check `CLAUSTRO_AUDIO_HOST` + `CLAUSTRO_AUDIO_PORT` (TCP)
   - Else exit with error: "audio bridge not available — is voice mode enabled in claustro.yaml?"
3. Connect to bridge socket
4. Send `START`
5. Read PCM chunks into a buffer
6. On SIGINT/SIGTERM → send `STOP`, write WAV file, exit 0
7. On connection error → write whatever is buffered, exit non-zero

### Dockerfile Integration

When voice mode is enabled, the Dockerfile template adds:

```dockerfile
{{if .VoiceMode}}
# Audio bridge shims — shadow rec/arecord to capture from host mic
COPY rec-shim /usr/local/bin/rec
COPY arecord-shim /usr/local/bin/arecord
RUN chmod +x /usr/local/bin/rec /usr/local/bin/arecord
{{end}}
```

`/usr/local/bin` precedes `/usr/bin` in PATH, so the shims shadow the real SoX/ALSA binaries. The real binaries remain available at `/usr/bin/rec` for non-recording use cases.

### Shim Build

The shims are pure Go, cross-compiled as static Linux binaries:
- `GOOS=linux GOARCH=amd64` and `GOOS=linux GOARCH=arm64`
- Built as part of `claustro rebuild` and embedded or placed alongside the Dockerfile
- Source lives in `cmd/rec-shim/` (or `internal/audio/shim/`)

---

## Integration Points

### Configuration

No new `claustro.yaml` fields. Audio passthrough activates automatically when `tools.voice: true` is set. The audio bridge is an implementation detail of voice mode.

### Mount Assembly (internal/mount/)

No new mounts needed. The audio socket reuses the existing `/run/claustro/` directory that the clipboard bridge already mounts:

```
/run/claustro/
├── clipboard.sock    # existing
└── audio.sock        # new (Linux only; macOS uses TCP)
```

### Container Creation (internal/container/)

- Add `VoiceMode bool` to `ExecOptions`
- Add `setupAudioBridge()` alongside `setupClipboardBridge()`
- Inject audio env vars into exec config:
  - Linux: `CLAUSTRO_AUDIO_SOCK=/run/claustro/audio.sock`
  - macOS: `CLAUSTRO_AUDIO_HOST=host.docker.internal`, `CLAUSTRO_AUDIO_PORT=<port>`

### Doctor Diagnostics (internal/doctor/)

Add voice mode check:
- "Voice mode: checking microphone access..."
- "Voice mode: microphone accessible" or "Voice mode: microphone not accessible (reason)"
- On macOS: specific guidance for System Settings → Privacy → Microphone

### Image Build (internal/image/)

- Existing: SoX + audio libraries installed when voice mode enabled
- New: COPY `rec-shim` and `arecord-shim` binaries into image
- Shim binaries cross-compiled during `claustro rebuild`

---

## Build & Cross-Compilation

### Current State

Pure Go, cross-compiled via GoReleaser for 4 targets. No CGO.

### Changes

**claustro binary (CGO enabled):**

| Target | Audio library | Framework/package |
|--------|--------------|-------------------|
| darwin/amd64 | CoreAudio | AudioToolbox.framework (system) |
| darwin/arm64 | CoreAudio | AudioToolbox.framework (system) |
| linux/amd64 | ALSA | libasound2 |
| linux/arm64 | ALSA | libasound2 |

**GoReleaser config:**

```yaml
builds:
  - id: claustro-darwin
    goos: [darwin]
    goarch: [amd64, arm64]
    env:
      - CGO_ENABLED=1

  - id: claustro-linux
    goos: [linux]
    goarch: [amd64, arm64]
    env:
      - CGO_ENABLED=1
```

**GitHub Actions CI:**
- macOS runner for darwin builds (CoreAudio frameworks available by default)
- Linux runner with `libasound2-dev` installed
- Alternative: use `zig cc` as cross-compiler for Linux targets from macOS

**Shim binaries (pure Go, no CGO):**
- Built as static linux binaries: `CGO_ENABLED=0 GOOS=linux`
- Two architectures: amd64 and arm64
- Embedded or built during `claustro rebuild`

### Graceful Fallback

When built without CGO (`CGO_ENABLED=0`):
- `capture_stub.go` activates
- `Available()` returns "audio capture not supported"
- Audio bridge does not start
- `claustro doctor` reports voice mode as unavailable
- SoX is still installed in the image (for non-recording uses)

---

## Security & Privacy

### Mic Access Is On-Demand

- Host microphone opens only when `rec` is called inside the container
- Mic closes immediately when recording stops
- Between recordings, no audio data flows — bridge server is idle

### No Audio Persisted on Host

- Raw PCM streams host → container in memory only
- WAV files exist only inside the container
- When the container is burned, audio files are destroyed

### macOS Microphone Permission

- First use: macOS shows system "Allow microphone access" dialog for claustro
- One-time OS-level prompt — not a claustro setup step
- If denied: shim gets a clear error, `claustro doctor` guides the user
- Permission can be granted later in System Settings → Privacy → Microphone

### Network Exposure (macOS TCP)

- TCP server binds to `127.0.0.1` only — not network-exposed
- Ephemeral port (OS-assigned)
- Single-connection server — rejects concurrent connections

### Container Isolation Preserved

- No `/dev/snd` device access
- No `--privileged` flag or extra capabilities
- No `--device` mappings
- Audio flows only through the bridge — same isolation model as clipboard

### Consent Model (Three Layers)

1. User sets `tools.voice: true` in `claustro.yaml` (opt-in config)
2. User invokes `/voice` in Claude Code (explicit action)
3. macOS enforces OS-level microphone permission (system dialog)

---

## Testing Strategy

### Unit Tests (No Hardware)

- `internal/audio/server_test.go` — mock capturer, verify START/STOP protocol, PCM chunk delivery, error handling on connection drop
- `rec-shim` tests — feed mock PCM data via test socket, verify valid WAV output (header correctness, byte count, sample rate)
- WAV header generation — table-driven tests with known data sizes

### Integration Tests (Hardware Required)

```go
//go:build integration && audio
```

- `capture_darwin_test.go` — CoreAudio capture opens/closes, returns PCM data
- `capture_linux_test.go` — ALSA capture with default device
- End-to-end: start bridge → run shim → verify WAV is valid audio

### CI

- Unit tests run in CI (mock capturer, no hardware)
- Integration tests run manually or on self-hosted runners with audio hardware
- Build verification: CGO compilation succeeds on all 4 targets

### Runtime Diagnostics

`claustro doctor` serves as the user-facing smoke test:
- Microphone accessibility check
- Bridge server start check
- More valuable than automated tests for hardware-dependent features

---

## Future: Optimization Path to Approach C

This design does not preclude adding direct ALSA device passthrough on Linux later:

- Add `audio.passthrough: true` option to `claustro.yaml` (Linux only)
- Mount `/dev/snd/*` devices into the container
- Add sandbox user to `audio` group in Dockerfile
- Bypass the bridge — `rec`/`arecord` use hardware directly
- Near-zero latency on Linux, bridge remains for macOS

This is a backward-compatible addition that can be done independently.

---

## Open Questions (Resolved)

| # | Question | Resolution |
|---|----------|------------|
| 1 | CGO or dlopen? | CGO — accept build complexity |
| 2 | New config fields? | No — activates with existing `tools.voice: true` |
| 3 | Separate mount? | No — reuse `/run/claustro/` from clipboard bridge |
| 4 | Shim as script or binary? | Binary — needs streaming, signal handling, WAV writing |
| 5 | macOS platform? | CoreAudio AudioQueue API via CGO, zero install |
| 6 | Linux platform? | ALSA via CGO (libasound2), zero install |
| 7 | Audio format? | Fixed 16kHz/mono/16-bit PCM, no negotiation |
