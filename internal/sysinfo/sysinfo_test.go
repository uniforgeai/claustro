// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package sysinfo

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetect_ReturnsUsableHost(t *testing.T) {
	h, _ := Detect()
	assert.NotNil(t, h)
	assert.Greater(t, h.CPUs, 0, "CPUs should be > 0")
	assert.Greater(t, h.MemoryBytes, int64(0), "MemoryBytes should be > 0")
}

func TestDetect_CPUsMatchesRuntime(t *testing.T) {
	h, _ := Detect()
	assert.Equal(t, runtime.NumCPU(), h.CPUs)
}

func TestSafeFallback_IsUsable(t *testing.T) {
	h := safeFallback()
	assert.Equal(t, 4, h.CPUs)
	assert.Equal(t, int64(8)*1024*1024*1024, h.MemoryBytes)
}
