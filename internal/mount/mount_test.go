package mount

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestSSHAgentContainerSock_linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific behaviour")
	}
	sock := "/run/user/1000/ssh-agent.sock"
	assert.Equal(t, sock, SSHAgentContainerSock(sock), "on Linux, host socket path passes through unchanged")
}

func TestSSHAgentContainerSock_darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific behaviour")
	}
	macSock := "/private/tmp/com.apple.launchd.ABC/Listeners"
	got := SSHAgentContainerSock(macSock)
	assert.Equal(t, dockerDesktopRelayPath, got, "on macOS, Docker Desktop relay path must be used")
}

func TestAssemble_agentForwardingEnabled_mountTarget(t *testing.T) {
	// Regardless of platform, when agent forwarding is on the container must
	// have exactly one mount whose Target matches SSHAgentContainerSock.
	hostSock := "/tmp/test-agent.sock"
	t.Setenv("SSH_AUTH_SOCK", hostSock)
	git := &config.GitConfig{ForwardAgent: boolPtr(true)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)

	want := SSHAgentContainerSock(hostSock)
	found := false
	for _, m := range mounts {
		if m.Target == want {
			found = true
			assert.Equal(t, want, m.Source, "Source and Target must match for SSH agent socket")
		}
	}
	assert.True(t, found, "SSH agent socket mount with target %q not found", want)
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
	assert.Equal(t, os.FileMode(0o777), info.Mode().Perm(), "clipboard socket directory must be world-accessible so the sandbox user can traverse it")
}

func TestAssemble_knownHostsMountedWhenPresent(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	realKnownHosts := filepath.Join(home, ".ssh", "known_hosts")

	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)

	found := false
	for _, m := range mounts {
		if m.Target == "/home/sandbox/.ssh/known_hosts" {
			found = true
			assert.Equal(t, realKnownHosts, m.Source)
			assert.True(t, m.ReadOnly, "known_hosts must be read-only")
		}
	}
	if fileExists(realKnownHosts) {
		assert.True(t, found, "known_hosts mount should be present when file exists")
	} else {
		assert.False(t, found, "known_hosts mount should be absent when file does not exist")
	}
}

func TestAssemble_pubKeysMountedWithAgentForwarding(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	sshDir := filepath.Join(home, ".ssh")
	entries, readErr := os.ReadDir(sshDir)
	if readErr != nil {
		t.Skip("~/.ssh does not exist on this machine")
	}

	var expectedPubs []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".pub" {
			expectedPubs = append(expectedPubs, e.Name())
		}
	}
	if len(expectedPubs) == 0 {
		t.Skip("no .pub files in ~/.ssh on this machine")
	}

	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	git := &config.GitConfig{ForwardAgent: boolPtr(true)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)

	for _, name := range expectedPubs {
		target := "/home/sandbox/.ssh/" + name
		found := false
		for _, m := range mounts {
			if m.Target == target {
				found = true
				assert.Equal(t, filepath.Join(sshDir, name), m.Source)
				assert.True(t, m.ReadOnly, ".pub mounts must be read-only")
				break
			}
		}
		assert.True(t, found, "expected pub key mount for %s", name)
	}
}

func TestAssemble_pluginCacheRemappedWhenHomeDiffers(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	pluginCache := filepath.Join(home, ".claude", "plugins", "cache")
	containerCache := "/home/sandbox/.claude/plugins/cache"

	mounts, err := Assemble("/some/project", nil, "")
	require.NoError(t, err)

	if home == "/home/sandbox" {
		// Inside a real container or if home happens to match, no extra mount needed.
		for _, m := range mounts {
			if m.Target == containerCache && m.Source == containerCache {
				t.Error("should not add redundant plugin cache mount when home is /home/sandbox")
			}
		}
	} else if fileExists(pluginCache) {
		// Host home differs from container home — expect the remapping mount.
		found := false
		for _, m := range mounts {
			if m.Target == pluginCache && m.Source == pluginCache {
				found = true
				assert.True(t, m.ReadOnly, "plugin cache remapping must be read-only")
			}
		}
		assert.True(t, found, "plugin cache should be mounted at host path %s", pluginCache)
	}
}

func TestAssemble_pubKeysNotMountedWhenAgentDisabled(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	git := &config.GitConfig{ForwardAgent: boolPtr(false)}
	mounts, err := Assemble("/some/project", git, "")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.False(t, len(m.Target) > 16 &&
			m.Target[:17] == "/home/sandbox/.ss" &&
			len(m.Target) > 4 &&
			m.Target[len(m.Target)-4:] == ".pub",
			"no .pub mounts should appear when agent forwarding is disabled: %s", m.Target)
	}
}
