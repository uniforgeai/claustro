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
