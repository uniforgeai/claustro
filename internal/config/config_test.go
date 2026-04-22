// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, cfg.ImageConfig.Extra)
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.ImageConfig.Extra)
}

func TestLoad_WithImageExtra(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  extra:
    - run: apt-get install -y ffmpeg
    - run: pip install black
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cfg.ImageConfig.Extra, 2)
	assert.Equal(t, "apt-get install -y ffmpeg", cfg.ImageConfig.Extra[0].Run)
	assert.Equal(t, "pip install black", cfg.ImageConfig.Extra[1].Run)
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte("image:\n  extra:\n    - {bad yaml"), 0644)
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
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cfg.ImageConfig.Extra, 1)
	assert.Equal(t, "npm install -g prettier", cfg.ImageConfig.Extra[0].Run)
}

func TestLoad_FullSpecSchema(t *testing.T) {
	dir := t.TempDir()
	content := `
project: my-saas
image: claude-sandbox:latest

defaults:
  firewall: false
  readonly: false
  resources:
    cpus: "4"
    memory: 8G

sandboxes:
  api:
    workdir: ./services/api
    mounts:
      - ./libs:/workspace/libs:ro
      - ./proto:/workspace/proto:ro
    env:
      DATABASE_URL: postgresql://host.docker.internal:5432/dev

  web:
    workdir: ./packages/frontend
    env:
      API_URL: http://localhost:3000

firewall:
  enabled: false
  allow:
    - custom-registry.company.com
    - api.openai.com

mcp:
  stdio:
    filesystem:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      env:
        DATABASE_URI: postgresql://postgres:postgres@db:5432/devdb

git:
  forward_agent: true
  mount_gitconfig: true
  mount_gh_config: false
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, "my-saas", cfg.Project)
	assert.Equal(t, "claude-sandbox:latest", cfg.ImageName)

	require.NotNil(t, cfg.Defaults.Firewall)
	assert.False(t, *cfg.Defaults.Firewall)
	require.NotNil(t, cfg.Defaults.ReadOnly)
	assert.False(t, *cfg.Defaults.ReadOnly)
	assert.Equal(t, "4", cfg.Defaults.Resources.CPUs)
	assert.Equal(t, "8G", cfg.Defaults.Resources.Memory)

	require.Len(t, cfg.Sandboxes, 2)
	api := cfg.Sandboxes["api"]
	assert.Equal(t, "./services/api", api.Workdir)
	require.Len(t, api.Mounts, 2)
	assert.Equal(t, "./libs:/workspace/libs:ro", api.Mounts[0])
	assert.Equal(t, "postgresql://host.docker.internal:5432/dev", api.Env["DATABASE_URL"])

	web := cfg.Sandboxes["web"]
	assert.Equal(t, "./packages/frontend", web.Workdir)
	assert.Equal(t, "http://localhost:3000", web.Env["API_URL"])

	require.NotNil(t, cfg.Firewall.Enabled)
	assert.False(t, *cfg.Firewall.Enabled)
	assert.Equal(t, []string{"custom-registry.company.com", "api.openai.com"}, cfg.Firewall.Allow)

	require.Contains(t, cfg.MCP.Stdio, "filesystem")
	assert.Equal(t, "npx", cfg.MCP.Stdio["filesystem"].Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"}, cfg.MCP.Stdio["filesystem"].Args)

	require.Contains(t, cfg.MCP.SSE, "postgres")
	assert.Equal(t, "crystaldba/postgres-mcp-server:latest", cfg.MCP.SSE["postgres"].Image)
	assert.Equal(t, "postgresql://postgres:postgres@db:5432/devdb", cfg.MCP.SSE["postgres"].Env["DATABASE_URI"])

	assert.True(t, cfg.Git.IsForwardAgent())
	assert.True(t, cfg.Git.IsMountGitconfig())
	assert.False(t, cfg.Git.IsMountGhConfig())
}

func TestLoad_MCPSSEPort(t *testing.T) {
	dir := t.TempDir()
	content := `
mcp:
  sse:
    postgres:
      image: crystaldba/postgres-mcp-server:latest
      port: 8000
      env:
        DATABASE_URI: postgresql://localhost/db
    browser:
      image: example/browser-mcp:latest
      port: 3000
    noport:
      image: example/noport:latest
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644))
	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.Equal(t, 8000, cfg.MCP.SSE["postgres"].Port)
	assert.Equal(t, 3000, cfg.MCP.SSE["browser"].Port)
	assert.Equal(t, 0, cfg.MCP.SSE["noport"].Port) // zero means "use default at runtime"
	assert.Equal(t, "crystaldba/postgres-mcp-server:latest", cfg.MCP.SSE["postgres"].Image)
	assert.Equal(t, "postgresql://localhost/db", cfg.MCP.SSE["postgres"].Env["DATABASE_URI"])
}

func TestLoad_ImageScalarString(t *testing.T) {
	dir := t.TempDir()
	content := `image: my-custom-image:v2`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-custom-image:v2", cfg.ImageName)
	assert.Empty(t, cfg.ImageConfig.Extra)
}

func TestLoad_ImageMapping(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  extra:
    - run: apt-get install -y curl
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.ImageName)
	require.Len(t, cfg.ImageConfig.Extra, 1)
	assert.Equal(t, "apt-get install -y curl", cfg.ImageConfig.Extra[0].Run)
}

func TestLoad_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	content := `
defaults:
  resources:
    cpus: "not-a-number"
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid claustro.yaml")
	assert.Contains(t, err.Error(), "defaults.resources.cpus")
}

func TestLoad_ValidationWarnings_DoNotBlock(t *testing.T) {
	dir := t.TempDir()
	content := `
defaults:
  resources:
    cpus: "0"
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err, "cpus=0 should produce a warning, not block load")
	assert.Equal(t, "0", cfg.Defaults.Resources.CPUs)
}

func TestLoad_GitOnly(t *testing.T) {
	dir := t.TempDir()
	content := `
git:
  forward_agent: false
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Git.IsForwardAgent())
	assert.Empty(t, cfg.ImageConfig.Extra)
	assert.Empty(t, cfg.Sandboxes)
}

func TestPauseConfig_Defaults(t *testing.T) {
	var p PauseConfig
	assert.True(t, p.IsEnabled(), "pause should default to enabled")
	assert.Equal(t, 5*time.Minute, p.Timeout())
}

func TestPauseConfig_OptOut(t *testing.T) {
	f := false
	p := PauseConfig{Enabled: &f}
	assert.False(t, p.IsEnabled())
}

func TestPauseConfig_CustomTimeout(t *testing.T) {
	p := PauseConfig{IdleTimeout: 30 * time.Second}
	assert.Equal(t, 30*time.Second, p.Timeout())
}

func TestLoad_PauseConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
pause:
  enabled: false
  idle_timeout: 10m
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0o644))
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Pause.IsEnabled())
	assert.Equal(t, 10*time.Minute, cfg.Pause.Timeout())
}

func TestLoad_PauseConfigDefaultsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte("project: foo"), 0o644))
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Pause.IsEnabled())
	assert.Equal(t, 5*time.Minute, cfg.Pause.Timeout())
}
