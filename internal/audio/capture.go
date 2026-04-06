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
