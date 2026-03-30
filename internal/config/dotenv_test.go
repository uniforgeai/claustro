// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDotenv_MissingFile(t *testing.T) {
	env, err := LoadDotenv(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, env)
}

func TestLoadDotenv_BasicKeyValue(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\nBAZ=qux\n"), 0644)
	require.NoError(t, err)

	env, err := LoadDotenv(dir)
	require.NoError(t, err)
	assert.Equal(t, "bar", env["FOO"])
	assert.Equal(t, "qux", env["BAZ"])
}

func TestLoadDotenv_CommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	content := `
# This is a comment
FOO=bar

# Another comment
BAZ=qux
`
	err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644)
	require.NoError(t, err)

	env, err := LoadDotenv(dir)
	require.NoError(t, err)
	assert.Len(t, env, 2)
	assert.Equal(t, "bar", env["FOO"])
	assert.Equal(t, "qux", env["BAZ"])
}

func TestLoadDotenv_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	content := `
DOUBLE="hello world"
SINGLE='hello world'
UNQUOTED=hello world
`
	err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0644)
	require.NoError(t, err)

	env, err := LoadDotenv(dir)
	require.NoError(t, err)
	assert.Equal(t, "hello world", env["DOUBLE"])
	assert.Equal(t, "hello world", env["SINGLE"])
	assert.Equal(t, "hello world", env["UNQUOTED"])
}

func TestLoadDotenv_KeyWithNoValue(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".env"), []byte("EMPTY=\nNOEQUALS\n"), 0644)
	require.NoError(t, err)

	env, err := LoadDotenv(dir)
	require.NoError(t, err)
	assert.Equal(t, "", env["EMPTY"])
	assert.NotContains(t, env, "NOEQUALS")
}

func TestLoadDotenv_DuplicateKeyLastWins(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".env"), []byte("KEY=first\nKEY=second\n"), 0644)
	require.NoError(t, err)

	env, err := LoadDotenv(dir)
	require.NoError(t, err)
	assert.Equal(t, "second", env["KEY"])
}
