package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, cfg.Image.Extra)
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "sandbox.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.Image.Extra)
}

func TestLoad_WithImageExtra(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  extra:
    - run: apt-get install -y ffmpeg
    - run: pip install black
`
	err := os.WriteFile(filepath.Join(dir, "sandbox.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cfg.Image.Extra, 2)
	assert.Equal(t, "apt-get install -y ffmpeg", cfg.Image.Extra[0].Run)
	assert.Equal(t, "pip install black", cfg.Image.Extra[1].Run)
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "sandbox.yaml"), []byte("image:\n  extra:\n    - {bad yaml"), 0644)
	require.NoError(t, err)

	_, err = Load(dir)
	assert.Error(t, err)
}

func boolPtr(b bool) *bool { return &b }

func TestGitConfig_defaults(t *testing.T) {
	var g GitConfig
	assert.True(t, g.IsForwardAgent(), "ForwardAgent defaults to true")
	assert.True(t, g.IsMountGitconfig(), "MountGitconfig defaults to true")
	assert.True(t, g.IsMountGhConfig(), "MountGhConfig defaults to true")
	assert.False(t, g.IsMountSSHDir(), "MountSSHDir defaults to false")
}

func TestGitConfig_explicitOverrides(t *testing.T) {
	g := GitConfig{
		ForwardAgent:   boolPtr(false),
		MountGitconfig: boolPtr(false),
		MountGhConfig:  boolPtr(false),
		MountSSHDir:    boolPtr(true),
	}
	assert.False(t, g.IsForwardAgent())
	assert.False(t, g.IsMountGitconfig())
	assert.False(t, g.IsMountGhConfig())
	assert.True(t, g.IsMountSSHDir())
}

func TestLoad_ExtraStepFields(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  extra:
    - run: npm install -g prettier
`
	err := os.WriteFile(filepath.Join(dir, "sandbox.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cfg.Image.Extra, 1)
	assert.Equal(t, "npm install -g prettier", cfg.Image.Extra[0].Run)
}
