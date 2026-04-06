// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"encoding/binary"
	"io"
)

// WriteWAVHeader writes a 44-byte RIFF WAV header for PCM audio.
func WriteWAVHeader(w io.Writer, dataSize uint32) error {
	byteRate := uint32(SampleRate * Channels * BitDepth / 8)
	blockAlign := uint16(Channels * BitDepth / 8)

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], 36+dataSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
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
