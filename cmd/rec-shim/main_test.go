// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/audio"
)

func TestParseOutputFile(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"simple", []string{"rec", "output.wav"}, "output.wav"},
		{"with_flags", []string{"rec", "-q", "-r", "16000", "output.wav"}, "output.wav"},
		{"full_flags", []string{"rec", "-q", "-r", "16000", "-c", "1", "-b", "16", "-t", "wav", "/tmp/recording.wav"}, "/tmp/recording.wav"},
		{"no_output", []string{"rec", "-q"}, ""},
		{"just_binary", []string{"rec"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOutputFile(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecordToWAV(t *testing.T) {
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
		// Close to signal EOF
	}()

	outFile := filepath.Join(dir, "output.wav")
	err = recordToWAV(sockPath, "", 0, outFile)
	require.NoError(t, err)

	// Verify WAV file
	data, err := os.ReadFile(outFile)
	require.NoError(t, err)

	assert.Equal(t, "RIFF", string(data[0:4]))
	assert.Equal(t, "WAVE", string(data[8:12]))

	dataSize := binary.LittleEndian.Uint32(data[40:44])
	assert.Equal(t, uint32(3*audio.ChunkSize), dataSize)
	assert.Equal(t, 44+int(dataSize), len(data))
}
