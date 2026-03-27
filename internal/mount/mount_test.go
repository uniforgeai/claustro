package mount

import (
	"os"
	"path/filepath"
	"testing"

	dockermount "github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func boolPtr(b bool) *bool { return &b }

func TestAssemble_basicMounts(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)

	// Must always have at least workspace + .claude
	assert.GreaterOrEqual(t, len(mounts), 2)

	assertMount(t, mounts, "/some/project", "/workspace", dockermount.TypeBind)

	home, _ := os.UserHomeDir()
	assertMount(t, mounts, filepath.Join(home, ".claude"), "/home/sandbox/.claude", dockermount.TypeBind)
}

func TestAssemble_claudeJSONIncludedWhenPresent(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	claudeJSON := filepath.Join(home, ".claude.json")
	exists := fileExists(claudeJSON)

	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)

	found := false
	for _, m := range mounts {
		if m.Target == "/home/sandbox/.claude.json" {
			found = true
			assert.Equal(t, claudeJSON, m.Source)
		}
	}
	assert.Equal(t, exists, found, ".claude.json mount presence should match file existence")
}

func TestAssemble_allMountsAreBind(t *testing.T) {
	mounts, err := Assemble("/any/path", nil, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.Equal(t, dockermount.TypeBind, m.Type)
	}
}

func assertMount(t *testing.T, mounts []dockermount.Mount, src, tgt string, typ dockermount.Type) {
	t.Helper()
	for _, m := range mounts {
		if m.Target == tgt {
			assert.Equal(t, src, m.Source)
			assert.Equal(t, typ, m.Type)
			return
		}
	}
	t.Errorf("mount with target %q not found", tgt)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestAssemble_gitconfigDisabled(t *testing.T) {
	git := &config.GitConfig{MountGitconfig: boolPtr(false)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.NotEqual(t, "/home/sandbox/.gitconfig", m.Target, "gitconfig mount should be absent when disabled")
	}
}

func TestAssemble_sshDirNotMountedByDefault(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.NotEqual(t, "/home/sandbox/.ssh", m.Target, "~/.ssh should not be mounted by default")
	}
}

func TestAssemble_sshDirMountedWhenEnabled(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	sshDir := filepath.Join(home, ".ssh")
	if !fileExists(sshDir) {
		t.Skip("~/.ssh does not exist on this machine")
	}

	git := &config.GitConfig{MountSSHDir: boolPtr(true)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)

	found := false
	for _, m := range mounts {
		if m.Target == "/home/sandbox/.ssh" {
			found = true
			assert.Equal(t, sshDir, m.Source)
			assert.True(t, m.ReadOnly)
		}
	}
	assert.True(t, found, "~/.ssh mount should be present when explicitly enabled")
}

func TestAssemble_agentForwardingDisabled(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/fake.sock")
	git := &config.GitConfig{ForwardAgent: boolPtr(false)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.NotEqual(t, "/tmp/fake.sock", m.Target, "SSH agent socket should not be mounted when disabled")
	}
}

func TestAssemble_clipboardSockDir(t *testing.T) {
	sockDir := t.TempDir()
	mounts, err := Assemble("/some/project", nil, sockDir)
	require.NoError(t, err)

	assertMount(t, mounts, sockDir, "/run/claustro", dockermount.TypeBind)
}

func TestAssemble_clipboardSockDir_empty_noMount(t *testing.T) {
	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.NotEqual(t, "/run/claustro", m.Target, "/run/claustro should not be mounted when clipboardSockDir is empty")
	}
}

func TestAssemble_clipboardSockDir_createsDir(t *testing.T) {
	parent := t.TempDir()
	sockDir := filepath.Join(parent, "new-socket-dir")

	_, err := Assemble("/some/project", nil, sockDir)
	require.NoError(t, err)

	info, err := os.Stat(sockDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "clipboard socket directory should be created")
}
