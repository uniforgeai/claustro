// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckStatus_String(t *testing.T) {
	tests := []struct {
		status CheckStatus
		want   string
	}{
		{Pass, "pass"},
		{Warn, "warn"},
		{Fail, "fail"},
		{Skip, "skip"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}

func TestCheckConfigFile(t *testing.T) {
	tests := []struct {
		name       string
		setupFile  bool
		wantStatus CheckStatus
	}{
		{
			name:       "directory with claustro.yaml returns Pass",
			setupFile:  true,
			wantStatus: Pass,
		},
		{
			name:       "directory without claustro.yaml returns Warn",
			setupFile:  false,
			wantStatus: Warn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.setupFile {
				err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte("# config\n"), 0o644)
				require.NoError(t, err)
			}

			result := CheckConfigFile(dir)
			assert.Equal(t, "Config File", result.Name)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}

func TestCheckGitConfig(t *testing.T) {
	result := CheckGitConfig()
	assert.Equal(t, "Git Config", result.Name)
	// We cannot control ~/.gitconfig in unit tests, but the function
	// must return a valid status without panicking.
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckSSHAgent(t *testing.T) {
	t.Run("SSH_AUTH_SOCK unset yields Fail or Warn", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		result := CheckSSHAgent()
		assert.Equal(t, "SSH Agent", result.Name)
		assert.Contains(t, []CheckStatus{Fail, Warn}, result.Status)
	})

	t.Run("SSH_AUTH_SOCK set to a value", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "/tmp/fake-ssh-agent.sock")
		result := CheckSSHAgent()
		assert.Equal(t, "SSH Agent", result.Name)
		// With a non-existent socket the check may pass or warn/fail,
		// but it must not panic.
		assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
	})
}

func TestCheckDockerSocket(t *testing.T) {
	result := CheckDockerSocket()
	assert.Equal(t, "Docker Socket", result.Name)
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckClipboard(t *testing.T) {
	result := CheckClipboard()
	assert.Equal(t, "Clipboard", result.Name)
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckDocker(t *testing.T) {
	result := CheckDocker(context.Background())
	assert.Equal(t, "Docker Engine", result.Name)
	// Docker may or may not be available in the test environment.
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckBaseImage(t *testing.T) {
	// Without a real Docker client we pass nil; the function must handle
	// this gracefully (return Fail or Skip) rather than panic.
	result := CheckBaseImage(context.Background(), nil)
	assert.Equal(t, "Base Image", result.Name)
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckGitHubCLI(t *testing.T) {
	result := CheckGitHubCLI()
	assert.Equal(t, "GitHub CLI", result.Name)
	assert.Contains(t, []CheckStatus{Pass, Warn, Fail, Skip}, result.Status)
}

func TestCheckConfigFile_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `defaults:
  resources:
    cpus: "not-a-number"
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(yaml), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, "Config File", result.Name)
	assert.Equal(t, Fail, result.Status)
}

func TestCheckConfigFile_WarningsOnly(t *testing.T) {
	dir := t.TempDir()
	yaml := `defaults:
  resources:
    cpus: "0"
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(yaml), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, "Config File", result.Name)
	assert.Equal(t, Warn, result.Status)
}

func TestCheckConfigFile_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `defaults:
  resources:
    cpus: "4"
    memory: "8G"
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(yaml), 0o644)
	require.NoError(t, err)

	result := CheckConfigFile(dir)
	assert.Equal(t, "Config File", result.Name)
	assert.Equal(t, Pass, result.Status)
}
