package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTerminalSizeFallback(t *testing.T) {
	// fd -1 is not a valid terminal; term.GetSize must fail and we expect defaults.
	w, h := getTerminalSize(-1)
	assert.Equal(t, uint(80), w)
	assert.Equal(t, uint(24), h)
}

func TestTermEnvDefaultTerm(t *testing.T) {
	t.Setenv("TERM", "")
	env := termEnv()
	require.Contains(t, env, "TERM=xterm-256color")
}

func TestGitEnvWithAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	env := gitEnv()
	assert.Contains(t, env, "GIT_CONFIG_COUNT=1")
	assert.Contains(t, env, "GIT_CONFIG_KEY_0=gpg.ssh.program")
	assert.Contains(t, env, "GIT_CONFIG_VALUE_0=ssh-keygen")
}

func TestGitEnvWithoutAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	env := gitEnv()
	assert.Nil(t, env, "no git env vars should be injected when SSH agent is absent")
}

func TestTermEnvHostValues(t *testing.T) {
	t.Setenv("TERM", "screen-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("LANG", "en_US.UTF-8")
	// LC_ALL unset — should not appear in output
	t.Setenv("LC_ALL", "")

	env := termEnv()
	assert.Contains(t, env, "TERM=screen-256color")
	assert.Contains(t, env, "COLORTERM=truecolor")
	assert.Contains(t, env, "LANG=en_US.UTF-8")
	for _, e := range env {
		assert.NotEqual(t, "LC_ALL=", e, "empty LC_ALL should be omitted")
	}
}
