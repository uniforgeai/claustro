// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build linux && cgo

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
