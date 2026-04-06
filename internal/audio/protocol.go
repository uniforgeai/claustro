// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package audio provides the audio bridge for streaming host microphone
// audio into claustro sandbox containers.
package audio

const (
	SampleRate   = 16000
	Channels     = 1
	BitDepth     = 16
	FrameSize    = Channels * BitDepth / 8
	ChunkFrames  = 1024
	ChunkSize    = ChunkFrames * FrameSize
	CmdStart     = "START"
	CmdStop      = "STOP"
	ErrByte      = 0xFF
	PortFileName = "audio.port"
	SockFileName = "audio.sock"
)
