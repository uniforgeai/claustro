// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build !(darwin || (linux && cgo))

package audio

import "errors"

// NewCapturer returns a stub capturer on platforms without audio support.
// On macOS, capture_darwin.go provides CoreAudio. On Linux with CGO,
// capture_linux.go provides ALSA. This stub covers everything else.
func NewCapturer() Capturer {
	return &stubCapturer{}
}

type stubCapturer struct{}

func (s *stubCapturer) Available() error           { return errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Start() error               { return errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Read(_ []byte) (int, error) { return 0, errors.New("audio capture not supported on this platform") }
func (s *stubCapturer) Stop() error                { return nil }
