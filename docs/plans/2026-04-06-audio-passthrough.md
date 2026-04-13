# Audio Passthrough Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Claude Code's `/voice` command inside claustro containers by bridging the host microphone into the sandbox via a socket-based audio server.

**Architecture:** Host-side audio capture server (CoreAudio on macOS, ALSA on Linux via CGO) streams raw 16kHz/mono/16-bit PCM to container-side `rec`/`arecord` shim binaries via TCP (macOS) or Unix socket (Linux). Follows the existing clipboard bridge pattern.

**Tech Stack:** Go, CGO, CoreAudio (macOS), ALSA/libasound2 (Linux), Docker SDK for Go

---

### Task 1: Audio Protocol Constants and WAV Writer

The foundation shared by both host server and container shims. Pure Go, no CGO, no platform-specific code.

**Files:**
- Create: `internal/audio/protocol.go`
- Create: `internal/audio/wav.go`
- Create: `internal/audio/protocol_test.go`
- Create: `internal/audio/wav_test.go`

- [ ] **Step 1: Write protocol constants**

Create `internal/audio/protocol.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package audio provides the audio bridge for streaming host microphone
// audio into claustro sandbox containers.
package audio

const (
	// SampleRate is the fixed audio sample rate in Hz.
	SampleRate = 16000
	// Channels is the fixed channel count (mono).
	Channels = 1
	// BitDepth is the fixed bit depth (16-bit signed LE).
	BitDepth = 16
	// FrameSize is the size of one audio frame in bytes (Channels * BitDepth/8).
	FrameSize = Channels * BitDepth / 8
	// ChunkFrames is the number of frames per chunk sent over the wire.
	ChunkFrames = 1024
	// ChunkSize is the byte size of one chunk (ChunkFrames * FrameSize).
	ChunkSize = ChunkFrames * FrameSize

	// CmdStart is sent by the shim to begin recording.
	CmdStart = "START"
	// CmdStop is sent by the shim to stop recording.
	CmdStop = "STOP"
	// ErrByte is the first byte of an error response from the server.
	ErrByte = 0xFF

	// PortFileName is the name of the file written to the socket directory
	// containing the TCP port number for macOS.
	PortFileName = "audio.port"
	// SockFileName is the name of the Unix socket for Linux.
	SockFileName = "audio.sock"
)
```

- [ ] **Step 2: Write WAV header writer**

Create `internal/audio/wav.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"encoding/binary"
	"io"
)

// WriteWAVHeader writes a 44-byte RIFF WAV header for PCM audio.
// dataSize is the number of raw PCM bytes that follow the header.
func WriteWAVHeader(w io.Writer, dataSize uint32) error {
	byteRate := uint32(SampleRate * Channels * BitDepth / 8)
	blockAlign := uint16(Channels * BitDepth / 8)

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], 36+dataSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16) // PCM chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(hdr[22:24], uint16(Channels))
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(SampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], byteRate)
	binary.LittleEndian.PutUint16(hdr[32:34], blockAlign)
	binary.LittleEndian.PutUint16(hdr[34:36], uint16(BitDepth))
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)

	_, err := w.Write(hdr[:])
	return err
}
```

- [ ] **Step 3: Write tests for WAV header**

Create `internal/audio/wav_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteWAVHeader(t *testing.T) {
	tests := []struct {
		name     string
		dataSize uint32
	}{
		{"empty", 0},
		{"one_chunk", ChunkSize},
		{"one_second", uint32(SampleRate * FrameSize)},
		{"ten_seconds", uint32(10 * SampleRate * FrameSize)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteWAVHeader(&buf, tt.dataSize)
			require.NoError(t, err)
			assert.Equal(t, 44, buf.Len())

			hdr := buf.Bytes()
			assert.Equal(t, "RIFF", string(hdr[0:4]))
			assert.Equal(t, 36+tt.dataSize, binary.LittleEndian.Uint32(hdr[4:8]))
			assert.Equal(t, "WAVE", string(hdr[8:12]))
			assert.Equal(t, "fmt ", string(hdr[12:16]))
			assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(hdr[20:22]))   // PCM
			assert.Equal(t, uint16(Channels), binary.LittleEndian.Uint16(hdr[22:24]))
			assert.Equal(t, uint32(SampleRate), binary.LittleEndian.Uint32(hdr[24:28]))
			assert.Equal(t, uint16(BitDepth), binary.LittleEndian.Uint16(hdr[34:36]))
			assert.Equal(t, "data", string(hdr[36:40]))
			assert.Equal(t, tt.dataSize, binary.LittleEndian.Uint32(hdr[40:44]))
		})
	}
}
```

- [ ] **Step 4: Write protocol constant tests**

Create `internal/audio/protocol_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolConstants(t *testing.T) {
	assert.Equal(t, 2, FrameSize, "16-bit mono = 2 bytes per frame")
	assert.Equal(t, 2048, ChunkSize, "1024 frames * 2 bytes")
	assert.Equal(t, 5, len(CmdStart))
	assert.Equal(t, 4, len(CmdStop))
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/audio/ -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/audio/
git commit -m "feat(audio): add protocol constants and WAV header writer"
```

---

### Task 2: Capturer Interface and Stub Implementation

Define the platform-agnostic capturer interface and the stub fallback for unsupported platforms.

**Files:**
- Create: `internal/audio/capture.go`
- Create: `internal/audio/capture_stub.go`

- [ ] **Step 1: Write capturer interface**

Create `internal/audio/capture.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

// Capturer abstracts platform-specific audio capture.
// Implementations exist for macOS (CoreAudio) and Linux (ALSA).
type Capturer interface {
	// Available checks if audio capture is possible on this host.
	// Returns nil if a microphone is accessible, or an error describing why not.
	Available() error
	// Start opens the microphone and begins capturing at 16kHz/mono/16-bit.
	Start() error
	// Read fills buf with raw PCM data. Blocks until data is available.
	// Returns the number of bytes read.
	Read(buf []byte) (int, error)
	// Stop closes the microphone and releases resources.
	Stop() error
}
```

- [ ] **Step 2: Write stub capturer**

Create `internal/audio/capture_stub.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build !darwin && !linux

package audio

import "errors"

// NewCapturer returns a stub capturer on unsupported platforms.
func NewCapturer() Capturer {
	return &stubCapturer{}
}

type stubCapturer struct{}

func (s *stubCapturer) Available() error            { return errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Start() error                { return errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Read(_ []byte) (int, error)  { return 0, errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Stop() error                 { return nil }
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/audio/`
Expected: Compiles without errors

- [ ] **Step 4: Commit**

```bash
git add internal/audio/capture.go internal/audio/capture_stub.go
git commit -m "feat(audio): add Capturer interface and stub fallback"
```

---

### Task 3: Audio Bridge Server

The host-side server that accepts connections from container shims, captures audio from the mic, and streams PCM chunks. Uses a mock capturer in tests.

**Files:**
- Create: `internal/audio/server.go`
- Create: `internal/audio/server_test.go`

- [ ] **Step 1: Write failing test for server START/STOP protocol**

Create `internal/audio/server_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"bytes"
	"io"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCapturer produces a fixed pattern of PCM data for testing.
type mockCapturer struct {
	mu      sync.Mutex
	running bool
	frame   byte
}

func (m *mockCapturer) Available() error { return nil }

func (m *mockCapturer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	m.frame = 0
	return nil
}

func (m *mockCapturer) Read(buf []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return 0, io.EOF
	}
	n := len(buf)
	if n > ChunkSize {
		n = ChunkSize
	}
	for i := 0; i < n; i++ {
		buf[i] = m.frame
	}
	m.frame++
	return n, nil
}

func (m *mockCapturer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

func TestServer_UnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, SockFileName)

	srv := NewServer(&mockCapturer{})
	err := srv.Start(sockPath)
	require.NoError(t, err)
	defer srv.Close()

	// Verify socket file exists
	_, err = os.Stat(sockPath)
	require.NoError(t, err)

	// Connect and run the protocol
	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send START
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	// Read a few chunks of PCM data
	var received bytes.Buffer
	buf := make([]byte, ChunkSize)
	for i := 0; i < 3; i++ {
		n, err := io.ReadFull(conn, buf)
		require.NoError(t, err)
		assert.Equal(t, ChunkSize, n)
		received.Write(buf[:n])
	}
	assert.Equal(t, 3*ChunkSize, received.Len())

	// Send STOP — server should close the connection
	_, err = conn.Write([]byte(CmdStop))
	require.NoError(t, err)

	// Read until EOF
	remaining, err := io.ReadAll(conn)
	assert.NoError(t, err)
	// May have some trailing data before EOF, that's fine
	_ = remaining
}

func TestServer_TCP(t *testing.T) {
	dir := t.TempDir()

	srv := NewServer(&mockCapturer{})
	port, err := srv.StartTCP(dir)
	require.NoError(t, err)
	defer srv.Close()

	assert.Greater(t, port, 0)

	// Verify port file exists
	portData, err := os.ReadFile(filepath.Join(dir, PortFileName))
	require.NoError(t, err)
	assert.NotEmpty(t, portData)

	// Connect via TCP
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer conn.Close()

	// Send START, read one chunk, send STOP
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	buf := make([]byte, ChunkSize)
	n, err := io.ReadFull(conn, buf)
	require.NoError(t, err)
	assert.Equal(t, ChunkSize, n)

	_, err = conn.Write([]byte(CmdStop))
	require.NoError(t, err)
}

func TestServer_ErrorWhenCapturerUnavailable(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, SockFileName)

	srv := NewServer(&stubCapturer{})
	err := srv.Start(sockPath)
	require.NoError(t, err)
	defer srv.Close()

	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send START — should get error byte + message
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	assert.Greater(t, n, 1)
	assert.Equal(t, byte(ErrByte), buf[0])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audio/ -run TestServer -v`
Expected: FAIL — `NewServer` not defined

- [ ] **Step 3: Implement the server**

Create `internal/audio/server.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Server is the host-side audio bridge server. It listens for connections
// from container-side shims and streams microphone audio as raw PCM.
type Server struct {
	capturer Capturer
	listener net.Listener
	portFile string
	sockPath string
	mu       sync.Mutex
	closed   bool
}

// NewServer creates a new audio bridge server backed by the given capturer.
func NewServer(capturer Capturer) *Server {
	return &Server{capturer: capturer}
}

// Start begins listening on a Unix socket at sockPath.
func (s *Server) Start(sockPath string) error {
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listening on audio socket: %w", err)
	}
	if err := os.Chmod(sockPath, 0o666); err != nil {
		ln.Close() //nolint:errcheck
		return fmt.Errorf("setting audio socket permissions: %w", err)
	}
	s.sockPath = sockPath
	s.listener = ln
	go s.acceptLoop()
	return nil
}

// StartTCP begins listening on TCP 127.0.0.1:0 and writes the port to a file.
func (s *Server) StartTCP(dir string) (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listening on audio TCP: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s.listener = ln

	portFile := filepath.Join(dir, PortFileName)
	if err := os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o644); err != nil {
		ln.Close() //nolint:errcheck
		return 0, fmt.Errorf("writing audio port file: %w", err)
	}
	s.portFile = portFile

	go s.acceptLoop()
	return port, nil
}

// Close shuts down the server and cleans up files.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true

	var closeErr error
	if s.listener != nil {
		closeErr = s.listener.Close()
	}
	if s.sockPath != "" {
		os.Remove(s.sockPath) //nolint:errcheck
	}
	if s.portFile != "" {
		os.Remove(s.portFile) //nolint:errcheck
	}
	return closeErr
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			slog.Debug("audio server accept error", "error", err)
			continue
		}
		// Handle one connection at a time (one recording at a time).
		s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// Wait for START command.
	cmdBuf := make([]byte, len(CmdStart))
	n, err := io.ReadFull(conn, cmdBuf)
	if err != nil || string(cmdBuf[:n]) != CmdStart {
		slog.Debug("audio server: invalid start command", "got", string(cmdBuf[:n]))
		return
	}

	// Try to start capture.
	if err := s.capturer.Start(); err != nil {
		slog.Debug("audio server: capturer start failed", "error", err)
		errMsg := append([]byte{ErrByte}, []byte(err.Error())...)
		conn.Write(errMsg) //nolint:errcheck
		return
	}

	// Stream PCM chunks until STOP is received.
	stopCh := make(chan struct{}, 1)
	go func() {
		// Listen for STOP command on the same connection.
		buf := make([]byte, len(CmdStop))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				close(stopCh)
				return
			}
			if string(buf[:n]) == CmdStop {
				close(stopCh)
				return
			}
		}
	}()

	chunk := make([]byte, ChunkSize)
	for {
		select {
		case <-stopCh:
			s.capturer.Stop() //nolint:errcheck
			return
		default:
		}

		n, err := s.capturer.Read(chunk)
		if err != nil {
			s.capturer.Stop() //nolint:errcheck
			return
		}
		if _, err := conn.Write(chunk[:n]); err != nil {
			s.capturer.Stop() //nolint:errcheck
			return
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audio/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audio/server.go internal/audio/server_test.go
git commit -m "feat(audio): add bridge server with TCP and Unix socket support"
```

---

### Task 4: macOS CoreAudio Capturer

CGO-based audio capture using macOS AudioQueue API. This will only compile on darwin.

**Files:**
- Create: `internal/audio/capture_darwin.go`

- [ ] **Step 1: Write CoreAudio capturer**

Create `internal/audio/capture_darwin.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build darwin

package audio

/*
#cgo LDFLAGS: -framework AudioToolbox -framework CoreFoundation
#include <AudioToolbox/AudioToolbox.h>
#include <string.h>

// Ring buffer shared between the callback and Go.
#define RING_SIZE (16000 * 2 * 2) // 2 seconds of 16kHz mono 16-bit
static unsigned char ringBuf[RING_SIZE];
static volatile int ringHead = 0;
static volatile int ringTail = 0;
static volatile int ringOverflow = 0;

static void audioCallback(
	void *inUserData,
	AudioQueueRef inAQ,
	AudioQueueBufferRef inBuffer,
	const AudioTimeStamp *inStartTime,
	UInt32 inNumberPackets,
	const AudioStreamPacketDescription *inPacketDescs
) {
	unsigned char *src = (unsigned char *)inBuffer->mAudioData;
	int len = (int)inBuffer->mAudioDataByteSize;
	for (int i = 0; i < len; i++) {
		int nextHead = (ringHead + 1) % RING_SIZE;
		if (nextHead == ringTail) {
			ringOverflow = 1;
			break;
		}
		ringBuf[ringHead] = src[i];
		ringHead = nextHead;
	}
	AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL);
}

static AudioQueueRef gQueue = NULL;
#define NUM_BUFFERS 3
#define BUFFER_SIZE 2048
static AudioQueueBufferRef gBuffers[NUM_BUFFERS];

static int startCapture() {
	AudioStreamBasicDescription fmt = {0};
	fmt.mSampleRate = 16000.0;
	fmt.mFormatID = kAudioFormatLinearPCM;
	fmt.mFormatFlags = kAudioFormatFlagIsSignedInteger | kAudioFormatFlagIsPacked;
	fmt.mBitsPerChannel = 16;
	fmt.mChannelsPerFrame = 1;
	fmt.mBytesPerFrame = 2;
	fmt.mFramesPerPacket = 1;
	fmt.mBytesPerPacket = 2;

	OSStatus status = AudioQueueNewInput(&fmt, audioCallback, NULL, NULL, NULL, 0, &gQueue);
	if (status != 0) return (int)status;

	for (int i = 0; i < NUM_BUFFERS; i++) {
		AudioQueueAllocateBuffer(gQueue, BUFFER_SIZE, &gBuffers[i]);
		AudioQueueEnqueueBuffer(gQueue, gBuffers[i], 0, NULL);
	}

	ringHead = 0;
	ringTail = 0;
	ringOverflow = 0;
	return (int)AudioQueueStart(gQueue, NULL);
}

static void stopCapture() {
	if (gQueue != NULL) {
		AudioQueueStop(gQueue, true);
		AudioQueueDispose(gQueue, true);
		gQueue = NULL;
	}
}

static int readRing(unsigned char *dst, int maxLen) {
	int count = 0;
	while (count < maxLen && ringTail != ringHead) {
		dst[count] = ringBuf[ringTail];
		ringTail = (ringTail + 1) % RING_SIZE;
		count++;
	}
	return count;
}
*/
import "C"

import (
	"errors"
	"time"
	"unsafe"
)

type darwinCapturer struct {
	running bool
}

// NewCapturer returns a CoreAudio-based capturer on macOS.
func NewCapturer() Capturer {
	return &darwinCapturer{}
}

func (d *darwinCapturer) Available() error {
	// CoreAudio is always available on macOS.
	// Mic permission is checked at Start() time by the OS.
	return nil
}

func (d *darwinCapturer) Start() error {
	if d.running {
		return errors.New("capture already running")
	}
	status := C.startCapture()
	if status != 0 {
		return errors.New("CoreAudio: failed to start capture (check microphone permission in System Settings > Privacy > Microphone)")
	}
	d.running = true
	return nil
}

func (d *darwinCapturer) Read(buf []byte) (int, error) {
	if !d.running {
		return 0, errors.New("capture not running")
	}
	// Poll the ring buffer. In practice the AudioQueue callback fills it
	// continuously, so we only spin briefly.
	for attempts := 0; attempts < 100; attempts++ {
		n := C.readRing((*C.uchar)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
		if n > 0 {
			return int(n), nil
		}
		time.Sleep(time.Millisecond)
	}
	return 0, nil // silence — no data available yet
}

func (d *darwinCapturer) Stop() error {
	if !d.running {
		return nil
	}
	C.stopCapture()
	d.running = false
	return nil
}
```

- [ ] **Step 2: Verify build on macOS**

Run: `go build ./internal/audio/`

Note: This file only compiles on macOS (`//go:build darwin`). On Linux CI, the stub or linux capturer is used instead. Verify locally on a Mac or accept that CI tests use the mock capturer.

- [ ] **Step 3: Commit**

```bash
git add internal/audio/capture_darwin.go
git commit -m "feat(audio): add macOS CoreAudio capturer via CGO"
```

---

### Task 5: Linux ALSA Capturer

CGO-based audio capture using ALSA (libasound2). Only compiles on linux.

**Files:**
- Create: `internal/audio/capture_linux.go`

- [ ] **Step 1: Write ALSA capturer**

Create `internal/audio/capture_linux.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build linux

package audio

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type alsaCapturer struct {
	handle *C.snd_pcm_t
}

// NewCapturer returns an ALSA-based capturer on Linux.
func NewCapturer() Capturer {
	return &alsaCapturer{}
}

func (a *alsaCapturer) Available() error {
	device := C.CString("default")
	defer C.free(unsafe.Pointer(device))

	var handle *C.snd_pcm_t
	rc := C.snd_pcm_open(&handle, device, C.SND_PCM_STREAM_CAPTURE, 0)
	if rc < 0 {
		return fmt.Errorf("ALSA: no capture device available (%s)", C.GoString(C.snd_strerror(rc)))
	}
	C.snd_pcm_close(handle)
	return nil
}

func (a *alsaCapturer) Start() error {
	if a.handle != nil {
		return errors.New("capture already running")
	}

	device := C.CString("default")
	defer C.free(unsafe.Pointer(device))

	rc := C.snd_pcm_open(&a.handle, device, C.SND_PCM_STREAM_CAPTURE, 0)
	if rc < 0 {
		return fmt.Errorf("ALSA open: %s", C.GoString(C.snd_strerror(rc)))
	}

	// Configure hardware parameters.
	var hwParams *C.snd_pcm_hw_params_t
	C.snd_pcm_hw_params_malloc(&hwParams)
	defer C.snd_pcm_hw_params_free(hwParams)

	C.snd_pcm_hw_params_any(a.handle, hwParams)
	C.snd_pcm_hw_params_set_access(a.handle, hwParams, C.SND_PCM_ACCESS_RW_INTERLEAVED)
	C.snd_pcm_hw_params_set_format(a.handle, hwParams, C.SND_PCM_FORMAT_S16_LE)
	C.snd_pcm_hw_params_set_channels(a.handle, hwParams, C.uint(Channels))

	rate := C.uint(SampleRate)
	C.snd_pcm_hw_params_set_rate_near(a.handle, hwParams, &rate, nil)

	rc = C.snd_pcm_hw_params(a.handle, hwParams)
	if rc < 0 {
		C.snd_pcm_close(a.handle)
		a.handle = nil
		return fmt.Errorf("ALSA hw_params: %s", C.GoString(C.snd_strerror(rc)))
	}

	return nil
}

func (a *alsaCapturer) Read(buf []byte) (int, error) {
	if a.handle == nil {
		return 0, errors.New("capture not running")
	}

	frames := C.snd_pcm_uframes_t(len(buf) / FrameSize)
	rc := C.snd_pcm_readi(a.handle, unsafe.Pointer(&buf[0]), frames)
	if rc == -C.EPIPE {
		// Overrun — recover and retry.
		C.snd_pcm_prepare(a.handle)
		rc = C.snd_pcm_readi(a.handle, unsafe.Pointer(&buf[0]), frames)
	}
	if rc < 0 {
		return 0, fmt.Errorf("ALSA read: %s", C.GoString(C.snd_strerror(C.int(rc))))
	}
	return int(rc) * FrameSize, nil
}

func (a *alsaCapturer) Stop() error {
	if a.handle == nil {
		return nil
	}
	C.snd_pcm_close(a.handle)
	a.handle = nil
	return nil
}
```

- [ ] **Step 2: Verify build on Linux**

Run: `go build ./internal/audio/`

Note: Requires `libasound2-dev` installed (`apt-get install libasound2-dev`). On macOS the darwin build tag is used instead.

- [ ] **Step 3: Commit**

```bash
git add internal/audio/capture_linux.go
git commit -m "feat(audio): add Linux ALSA capturer via CGO"
```

---

### Task 6: Container-Side rec-shim Binary

A pure Go binary that replaces `rec` inside the container. Connects to the audio bridge, receives PCM, writes WAV.

**Files:**
- Create: `cmd/rec-shim/main.go`
- Create: `cmd/rec-shim/main_test.go`

- [ ] **Step 1: Write the shim test**

Create `cmd/rec-shim/main_test.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/audio"
)

func TestParseOutputFile_Rec(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
	}{
		{[]string{"rec", "output.wav"}, "output.wav"},
		{[]string{"rec", "-q", "-r", "16000", "output.wav"}, "output.wav"},
		{[]string{"rec", "-q", "-r", "16000", "-c", "1", "-b", "16", "-t", "wav", "/tmp/recording.wav"}, "/tmp/recording.wav"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := parseOutputFile(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecordToWAV(t *testing.T) {
	// Start a mock server that sends 3 chunks then closes.
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "audio.sock")

	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read START
		buf := make([]byte, len(audio.CmdStart))
		conn.Read(buf) //nolint:errcheck

		// Send 3 chunks of predictable data
		chunk := make([]byte, audio.ChunkSize)
		for i := 0; i < 3; i++ {
			for j := range chunk {
				chunk[j] = byte(i)
			}
			conn.Write(chunk) //nolint:errcheck
		}
		// Close to signal EOF (simulating STOP response)
	}()

	outFile := filepath.Join(dir, "output.wav")
	err = recordToWAV(sockPath, "", 0, outFile)
	require.NoError(t, err)

	// Verify WAV file
	data, err := os.ReadFile(outFile)
	require.NoError(t, err)

	// Check WAV header
	assert.Equal(t, "RIFF", string(data[0:4]))
	assert.Equal(t, "WAVE", string(data[8:12]))

	dataSize := binary.LittleEndian.Uint32(data[40:44])
	assert.Equal(t, uint32(3*audio.ChunkSize), dataSize)
	assert.Equal(t, 44+int(dataSize), len(data))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rec-shim/ -v`
Expected: FAIL — `parseOutputFile` and `recordToWAV` not defined

- [ ] **Step 3: Implement the shim**

Create `cmd/rec-shim/main.go`:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// rec-shim is a drop-in replacement for SoX's `rec` command inside claustro
// containers. Instead of capturing from a hardware audio device, it connects
// to the claustro audio bridge on the host and receives PCM data via socket.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/uniforgeai/claustro/internal/audio"
)

func main() {
	outFile := parseOutputFile(os.Args)
	if outFile == "" {
		fmt.Fprintln(os.Stderr, "rec-shim: no output file specified")
		os.Exit(1)
	}

	sockPath := os.Getenv("CLAUSTRO_AUDIO_SOCK")
	host := os.Getenv("CLAUSTRO_AUDIO_HOST")
	portStr := os.Getenv("CLAUSTRO_AUDIO_PORT")

	var port int
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rec-shim: invalid CLAUSTRO_AUDIO_PORT: %s\n", portStr)
			os.Exit(1)
		}
	}

	if sockPath == "" && host == "" {
		fmt.Fprintln(os.Stderr, "rec-shim: audio bridge not available — is voice mode enabled in claustro.yaml?")
		os.Exit(1)
	}

	if err := recordToWAV(sockPath, host, port, outFile); err != nil {
		fmt.Fprintf(os.Stderr, "rec-shim: %v\n", err)
		os.Exit(1)
	}
}

// parseOutputFile extracts the output filename from rec-style arguments.
// The output file is the last argument that doesn't start with '-'.
func parseOutputFile(args []string) string {
	for i := len(args) - 1; i >= 1; i-- {
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return ""
}

// recordToWAV connects to the audio bridge, streams PCM, and writes a WAV file.
func recordToWAV(sockPath, host string, port int, outFile string) error {
	var conn net.Conn
	var err error

	if sockPath != "" {
		conn, err = net.Dial("unix", sockPath)
	} else {
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	}
	if err != nil {
		return fmt.Errorf("connecting to audio bridge: %w", err)
	}
	defer conn.Close()

	// Send START.
	if _, err := conn.Write([]byte(audio.CmdStart)); err != nil {
		return fmt.Errorf("sending START: %w", err)
	}

	// Check for error response.
	peek := make([]byte, 1)
	n, err := conn.Read(peek)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if n > 0 && peek[0] == audio.ErrByte {
		errMsg, _ := io.ReadAll(conn)
		return fmt.Errorf("audio bridge error: %s", string(errMsg))
	}

	// Set up signal handler for graceful stop.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Collect PCM data. Start with the byte we already peeked.
	var pcmBuf bytes.Buffer
	pcmBuf.Write(peek[:n])

	buf := make([]byte, audio.ChunkSize)
	done := false

	go func() {
		<-sigCh
		// Send STOP to the server.
		conn.Write([]byte(audio.CmdStop)) //nolint:errcheck
		done = true
	}()

	for !done {
		n, err := conn.Read(buf)
		if n > 0 {
			pcmBuf.Write(buf[:n])
		}
		if err != nil {
			break // EOF or error — server closed connection
		}
	}

	// Write WAV file.
	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	pcmData := pcmBuf.Bytes()
	if err := audio.WriteWAVHeader(f, uint32(len(pcmData))); err != nil {
		return fmt.Errorf("writing WAV header: %w", err)
	}
	if _, err := f.Write(pcmData); err != nil {
		return fmt.Errorf("writing PCM data: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/rec-shim/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/rec-shim/
git commit -m "feat(audio): add rec-shim binary for container-side audio bridge"
```

---

### Task 7: Wire Audio Bridge into Container Exec

Connect the audio bridge server to the container exec lifecycle, following the clipboard bridge pattern.

**Files:**
- Modify: `internal/container/container.go` — add `VoiceMode` to `ExecOptions`, add `setupAudioBridge()`
- Modify: `cmd/claustro/claude.go` — pass voice mode flag

- [ ] **Step 1: Add VoiceMode to ExecOptions and setupAudioBridge**

In `internal/container/container.go`, add `VoiceMode bool` to `ExecOptions`:

```go
// ExecOptions configures an Exec call.
type ExecOptions struct {
	// Interactive attaches stdin/stdout/stderr and allocates a TTY.
	Interactive bool
	// ClipboardSockDir is the host directory where the clipboard bridge socket will
	// be created. When non-empty and Interactive is true, a clipboard server is
	// started for the duration of the exec session.
	ClipboardSockDir string
	// VoiceMode enables the audio bridge for microphone passthrough.
	// When true and Interactive, an audio capture server is started.
	VoiceMode bool
}
```

Add `setupAudioBridge()` function after `setupClipboardBridge()`:

```go
// setupAudioBridge starts an audio capture server for interactive sessions
// when voice mode is enabled. Returns a cleanup function.
func setupAudioBridge(opts ExecOptions) (func(), []string) {
	noop := func() {}
	if !opts.Interactive || !opts.VoiceMode || opts.ClipboardSockDir == "" {
		return noop, nil
	}

	capturer := audio.NewCapturer()
	if err := capturer.Available(); err != nil {
		slog.Warn("audio bridge unavailable", "err", err)
		return noop, nil
	}

	srv := audio.NewServer(capturer)
	var env []string

	if runtime.GOOS == "darwin" {
		port, err := srv.StartTCP(opts.ClipboardSockDir)
		if err != nil {
			slog.Warn("audio bridge TCP start failed", "err", err)
			return noop, nil
		}
		env = []string{
			"CLAUSTRO_AUDIO_HOST=host.docker.internal",
			"CLAUSTRO_AUDIO_PORT=" + strconv.Itoa(port),
		}
	} else {
		sockPath := filepath.Join(opts.ClipboardSockDir, audio.SockFileName)
		if err := srv.Start(sockPath); err != nil {
			slog.Warn("audio bridge socket start failed", "err", err)
			return noop, nil
		}
		env = []string{
			"CLAUSTRO_AUDIO_SOCK=/run/claustro/" + audio.SockFileName,
		}
	}

	return func() { srv.Close() }, env //nolint:errcheck
}
```

Update `Exec()` to call `setupAudioBridge` and merge env vars:

In the `Exec` function, after `cleanup := setupClipboardBridge(opts)`, add:

```go
	audioCleanup, audioEnv := setupAudioBridge(opts)
	defer audioCleanup()
```

And in the exec config setup, where `execCfg.Env` is set for interactive mode:

```go
	if opts.Interactive {
		execCfg.Env = append(termEnv(), gitEnv()...)
		execCfg.Env = append(execCfg.Env, audioEnv...)
	}
```

Add the required imports: `"github.com/uniforgeai/claustro/internal/audio"` and `"strconv"`.

- [ ] **Step 2: Update claude.go to pass VoiceMode**

In `cmd/claustro/claude.go`, load the config and check voice mode:

```go
func runClaude(ctx context.Context, name string, extraArgs []string) error {
	// ... existing code up to container.Exec call ...

	// Check if voice mode is enabled in config.
	cfg, _ := config.Load(".")
	voiceEnabled := cfg != nil && cfg.ImageBuild.IsToolGroupEnabled("voice")

	execCmd := append([]string{"claude", "--dangerously-skip-permissions"}, extraArgs...)
	sockDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	return container.Exec(ctx, cli, c.ID, execCmd, container.ExecOptions{
		Interactive:      true,
		ClipboardSockDir: sockDir,
		VoiceMode:        voiceEnabled,
	})
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/claustro/`
Expected: Compiles without errors

- [ ] **Step 4: Commit**

```bash
git add internal/container/container.go cmd/claustro/claude.go
git commit -m "feat(audio): wire audio bridge into container exec lifecycle"
```

---

### Task 8: Dockerfile Template and Shim Embedding

Update the Dockerfile template to COPY shim binaries and embed them in the image build context.

**Files:**
- Modify: `internal/image/Dockerfile.tmpl` — add COPY for rec-shim and arecord-shim
- Modify: `internal/image/image.go` — embed shim binaries and add to build context

- [ ] **Step 1: Build the shim binary for embedding**

The shim needs to be compiled as a static Linux binary. Add a build script or Makefile target. For now, build manually:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -o internal/image/rec-shim ./cmd/rec-shim/
cp internal/image/rec-shim internal/image/arecord-shim
```

- [ ] **Step 2: Update Dockerfile.tmpl**

In `internal/image/Dockerfile.tmpl`, replace the existing voice mode block:

```dockerfile
{{if .VoiceMode}}
# Install SoX for Claude Code voice mode (/voice command)
RUN apt-get update && apt-get install -y \
    sox \
    libsox-fmt-all \
    alsa-utils \
    pulseaudio-utils \
    && rm -rf /var/lib/apt/lists/*
# Audio bridge shims — shadow rec/arecord to capture from host mic
COPY rec-shim /usr/local/bin/rec
COPY arecord-shim /usr/local/bin/arecord
RUN chmod +x /usr/local/bin/rec /usr/local/bin/arecord
{{end}}
```

- [ ] **Step 3: Update image.go to embed shims**

In `internal/image/image.go`, add embeds and include in build context:

```go
//go:embed rec-shim
var recShim []byte

//go:embed arecord-shim
var arecordShim []byte
```

In the `buildContext` function, add to the `files` slice (conditionally for voice mode — this requires passing imgCfg into buildContext, which is already done):

```go
if imgCfg.IsToolGroupEnabled("voice") {
	files = append(files,
		struct{ name string; data []byte; mode int64 }{"rec-shim", recShim, 0755},
		struct{ name string; data []byte; mode int64 }{"arecord-shim", arecordShim, 0755},
	)
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/image/`
Expected: Compiles (requires rec-shim and arecord-shim binaries to exist in internal/image/)

- [ ] **Step 5: Run existing image tests**

Run: `go test ./internal/image/ -v`
Expected: All existing tests PASS, voice mode tests include shim COPY directives

- [ ] **Step 6: Commit**

```bash
git add internal/image/Dockerfile.tmpl internal/image/image.go internal/image/rec-shim internal/image/arecord-shim
git commit -m "feat(audio): embed rec/arecord shims in Docker image build"
```

---

### Task 9: Doctor Diagnostic for Voice Mode

Add a microphone accessibility check to `claustro doctor`.

**Files:**
- Modify: `internal/doctor/doctor.go` — add `CheckVoiceMode` function

- [ ] **Step 1: Read existing doctor checks to understand the pattern**

Check `internal/doctor/doctor.go` — each check returns a `CheckResult` with `Name`, `Status`, `Detail`, and `FixHint`.

- [ ] **Step 2: Add CheckVoiceMode**

Add to `internal/doctor/doctor.go`:

```go
// CheckVoiceMode verifies that audio capture is available for voice mode.
func CheckVoiceMode() CheckResult {
	capturer := audio.NewCapturer()
	if err := capturer.Available(); err != nil {
		fixHint := "Audio capture is not available on this platform."
		if runtime.GOOS == "darwin" {
			fixHint = "Check System Settings > Privacy & Security > Microphone and allow claustro."
		} else if runtime.GOOS == "linux" {
			fixHint = "Ensure libasound2 is installed and a capture device is available (arecord -l)."
		}
		return CheckResult{
			Name:    "Voice mode",
			Status:  Warn,
			Detail:  err.Error(),
			FixHint: fixHint,
		}
	}
	return CheckResult{
		Name:   "Voice mode",
		Status: Pass,
		Detail: "microphone accessible",
	}
}
```

Add import: `"github.com/uniforgeai/claustro/internal/audio"` and `"runtime"`.

- [ ] **Step 3: Wire into doctor command**

In `cmd/claustro/doctor.go`, add `CheckVoiceMode` to the list of checks that are run.

- [ ] **Step 4: Verify build**

Run: `go build ./cmd/claustro/`
Expected: Compiles without errors

- [ ] **Step 5: Commit**

```bash
git add internal/doctor/doctor.go cmd/claustro/doctor.go
git commit -m "feat(audio): add voice mode diagnostic to claustro doctor"
```

---

### Task 10: Update Documentation

Update site docs and specs to reflect audio passthrough.

**Files:**
- Modify: `docs/site/content/commands/claude.md`
- Modify: `docs/site/content/reference/claustro-yaml.md`
- Modify: `docs/site/content/guides/custom-image.md`
- Modify: `docs/specs/spec.md`

- [ ] **Step 1: Update claustro.yaml reference**

In `docs/site/content/reference/claustro-yaml.md`, update the `tools.voice` description:

```markdown
| `voice` | bool | false | SoX + audio bridge for Claude Code `/voice` command (opt-in). Enables host microphone passthrough into containers. |
```

- [ ] **Step 2: Update the main spec**

In `docs/specs/spec.md`, update the Voice Mode Support requirement to mention audio passthrough:

Add to the "Voice mode enabled" scenario:

```markdown
- **AND** the audio bridge streams host microphone audio into the container via socket
- **AND** container-side `rec`/`arecord` shims connect to the bridge for recording
```

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: update voice mode documentation for audio passthrough"
```
