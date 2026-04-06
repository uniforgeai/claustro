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
			assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(hdr[20:22]))
			assert.Equal(t, uint16(Channels), binary.LittleEndian.Uint16(hdr[22:24]))
			assert.Equal(t, uint32(SampleRate), binary.LittleEndian.Uint32(hdr[24:28]))
			assert.Equal(t, uint16(BitDepth), binary.LittleEndian.Uint16(hdr[34:36]))
			assert.Equal(t, "data", string(hdr[36:40]))
			assert.Equal(t, tt.dataSize, binary.LittleEndian.Uint32(hdr[40:44]))
		})
	}
}
