// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCapturer produces predictable PCM data: each chunk is filled with an
// incrementing byte value (0x00, 0x01, 0x02, ...).
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

// errCapturer always returns errors, simulating an unavailable microphone.
type errCapturer struct{}

func (e *errCapturer) Available() error           { return errors.New("no mic") }
func (e *errCapturer) Start() error               { return errors.New("no mic") }
func (e *errCapturer) Read(_ []byte) (int, error) { return 0, errors.New("no mic") }
func (e *errCapturer) Stop() error                { return nil }

func TestServer_UnixSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, SockFileName)

	mock := &mockCapturer{}
	srv := NewServer(mock)

	require.NoError(t, srv.Start(sockPath))
	defer srv.Close() //nolint:errcheck

	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send START command.
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	// Read 3 chunks and verify sizes.
	for i := 0; i < 3; i++ {
		buf := make([]byte, ChunkSize)
		n, err := io.ReadFull(conn, buf)
		require.NoError(t, err)
		assert.Equal(t, ChunkSize, n, "chunk %d should be %d bytes", i, ChunkSize)
	}

	// Send STOP command.
	_, err = conn.Write([]byte(CmdStop))
	require.NoError(t, err)

	// Read until EOF — server should close the connection after STOP.
	remaining, err := io.ReadAll(conn)
	// err may be nil (clean EOF) or io.EOF.
	if err != nil {
		assert.ErrorIs(t, err, io.EOF)
	}
	// There may be some trailing data buffered before STOP was processed.
	_ = remaining
}

func TestServer_TCP(t *testing.T) {
	dir := t.TempDir()

	mock := &mockCapturer{}
	srv := NewServer(mock)

	port, err := srv.StartTCP(dir)
	require.NoError(t, err)
	require.Greater(t, port, 0)
	defer srv.Close() //nolint:errcheck

	// Verify port file was written.
	portFile := filepath.Join(dir, PortFileName)
	data, err := os.ReadFile(portFile)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Connect via TCP.
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", string(data)))
	require.NoError(t, err)
	defer conn.Close()

	// Send START.
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	// Read 1 chunk.
	buf := make([]byte, ChunkSize)
	n, err := io.ReadFull(conn, buf)
	require.NoError(t, err)
	assert.Equal(t, ChunkSize, n)

	// Send STOP.
	_, err = conn.Write([]byte(CmdStop))
	require.NoError(t, err)

	// Drain to EOF.
	_, _ = io.ReadAll(conn)
}

func TestServer_ErrorWhenCapturerUnavailable(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, SockFileName)

	srv := NewServer(&errCapturer{})

	require.NoError(t, srv.Start(sockPath))
	defer srv.Close() //nolint:errcheck

	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send START — capturer.Start() will fail.
	_, err = conn.Write([]byte(CmdStart))
	require.NoError(t, err)

	// Read error response.
	resp, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.NotEmpty(t, resp)

	// First byte must be ErrByte.
	assert.Equal(t, byte(ErrByte), resp[0], "first byte should be ErrByte (0xFF)")
	// Remainder is the error message.
	assert.Contains(t, string(resp[1:]), "no mic")
}
