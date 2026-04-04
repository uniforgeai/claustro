// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNanoCPUs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr string
	}{
		{name: "empty returns default", input: "", want: defaultNanoCPUs},
		{name: "integer cpus", input: "4", want: 4_000_000_000},
		{name: "fractional cpus", input: "0.5", want: 500_000_000},
		{name: "two cpus", input: "2", want: 2_000_000_000},
		{name: "invalid string", input: "abc", wantErr: "invalid cpu value"},
		{name: "zero", input: "0", wantErr: "cpus must be positive"},
		{name: "negative", input: "-1", wantErr: "cpus must be positive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNanoCPUs(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr string
	}{
		{name: "empty returns default", input: "", want: defaultMemory},
		{name: "gigabytes", input: "8G", want: 8 * 1024 * 1024 * 1024},
		{name: "lowercase g", input: "8g", want: 8 * 1024 * 1024 * 1024},
		{name: "megabytes", input: "512M", want: 512 * 1024 * 1024},
		{name: "lowercase m", input: "512m", want: 512 * 1024 * 1024},
		{name: "kilobytes", input: "1024K", want: 1024 * 1024},
		{name: "lowercase k", input: "1024k", want: 1024 * 1024},
		{name: "invalid suffix", input: "8X", wantErr: "unknown memory suffix"},
		{name: "no suffix", input: "8", wantErr: "invalid memory value"},
		{name: "too short", input: "G", wantErr: "invalid memory value"},
		{name: "zero", input: "0G", wantErr: "memory must be positive"},
		{name: "negative", input: "-1G", wantErr: "memory must be positive"},
		{name: "non-numeric", input: "abcG", wantErr: "invalid memory number"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMemory(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
