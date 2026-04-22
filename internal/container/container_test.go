// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uniforgeai/claustro/internal/sysinfo"
)

func TestSmartCPUs_Table(t *testing.T) {
	tests := []struct {
		cores int
		want  int64
	}{
		{4, 2 * 1_000_000_000},
		{8, 2 * 1_000_000_000},
		{10, 2 * 1_000_000_000},
		{12, 3 * 1_000_000_000},
		{16, 4 * 1_000_000_000},
	}
	for _, tt := range tests {
		got := smartCPUs(&sysinfo.Host{CPUs: tt.cores})
		assert.Equal(t, tt.want, got, "cores=%d", tt.cores)
	}
}

func TestSmartCPUs_FloorAtTwo(t *testing.T) {
	got := smartCPUs(&sysinfo.Host{CPUs: 1})
	assert.Equal(t, int64(2)*1_000_000_000, got)
}

func TestSmartMemory_Table(t *testing.T) {
	gib := func(n int64) int64 { return n * 1024 * 1024 * 1024 }
	tests := []struct {
		hostBytes int64
		want      int64
	}{
		{gib(8), gib(2)},
		{gib(16), gib(4)},
		{gib(32), gib(8)},
		{gib(64), gib(8)}, // capped at 8 GiB
	}
	for _, tt := range tests {
		got := smartMemory(&sysinfo.Host{MemoryBytes: tt.hostBytes})
		assert.Equal(t, tt.want, got, "host=%d", tt.hostBytes)
	}
}

func TestParseNanoCPUs_EmptyUsesSmartCPUs(t *testing.T) {
	host := &sysinfo.Host{CPUs: 16, MemoryBytes: 32 * 1024 * 1024 * 1024}
	got, err := parseNanoCPUsForHost("", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(4)*1_000_000_000, got)
}

func TestParseNanoCPUs_ExplicitOverridesSmart(t *testing.T) {
	host := &sysinfo.Host{CPUs: 16, MemoryBytes: 32 * 1024 * 1024 * 1024}
	got, err := parseNanoCPUsForHost("8", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(8)*1_000_000_000, got)
}

func TestParseMemory_EmptyUsesSmartMemory(t *testing.T) {
	host := &sysinfo.Host{CPUs: 8, MemoryBytes: int64(32) * 1024 * 1024 * 1024}
	got, err := parseMemoryForHost("", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(8)*1024*1024*1024, got)
}

func TestParseMemory_ExplicitOverridesSmart(t *testing.T) {
	host := &sysinfo.Host{CPUs: 8, MemoryBytes: int64(32) * 1024 * 1024 * 1024}
	got, err := parseMemoryForHost("4G", host)
	assert.NoError(t, err)
	assert.Equal(t, int64(4)*1024*1024*1024, got)
}
