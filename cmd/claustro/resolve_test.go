// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveName_NonEmptyName verifies that a non-empty name is passed through unchanged,
// without any Docker calls.
func TestResolveName_NonEmptyName(t *testing.T) {
	ctx := context.Background()
	// When name is provided, resolveName should return it without calling Docker.
	// We pass nil as the client intentionally — it must not be used.
	result, err := resolveName(ctx, nil, "myproject", "happy_panda")
	require.NoError(t, err)
	assert.Equal(t, "happy_panda", result)
}

// TestResolveName_NonEmptyName_AnyProject verifies the pass-through for different project values.
func TestResolveName_NonEmptyName_AnyProject(t *testing.T) {
	tests := []struct {
		project string
		name    string
	}{
		{"myapp", "swift_falcon"},
		{"other-project", "calm_river"},
		{"proj", "bold_eagle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveName(context.Background(), nil, tt.project, tt.name)
			require.NoError(t, err)
			assert.Equal(t, tt.name, result)
		})
	}
}
