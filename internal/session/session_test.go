// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package session

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	claudeDir := filepath.Join("testdata")

	sessions, err := List(claudeDir)
	require.NoError(t, err)

	// The malformed file (cccc...) should be skipped; we expect 2 sessions.
	require.Len(t, sessions, 2)

	// Sessions are sorted by UpdatedAt descending — bbbb is newer (Apr 12) than aaaa (Apr 10).
	assert.Equal(t, "bbbb1111-2222-3333-4444-555566667777", sessions[0].ID)
	assert.Equal(t, "(untitled)", sessions[0].Title)

	assert.Equal(t, "aaaa1111-2222-3333-4444-555566667777", sessions[1].ID)
	assert.Equal(t, "refactoring auth middleware", sessions[1].Title)
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := List(dir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestList_MissingDir(t *testing.T) {
	sessions, err := List("/nonexistent/path")
	require.NoError(t, err)
	assert.Empty(t, sessions)
}
