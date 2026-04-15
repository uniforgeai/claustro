// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package wizard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions("myproject")
	assert.Equal(t, "myproject", opts.Project)
	assert.Equal(t, []string{"go", "rust", "python"}, opts.Languages)
	assert.Equal(t, []string{"dev", "build"}, opts.Tools)
	assert.Equal(t, []string{"filesystem", "memory", "fetch"}, opts.MCPServers)
	assert.Equal(t, []string{"codex"}, opts.Agents)
	assert.Equal(t, "4", opts.CPUs)
	assert.Equal(t, "8G", opts.Memory)
	assert.False(t, opts.Firewall)
	assert.False(t, opts.ReadOnly)
	assert.True(t, opts.ForwardAgent)
	assert.True(t, opts.MountGitconfig)
	assert.True(t, opts.MountGhConfig)
}

func TestBuildConfig_Defaults(t *testing.T) {
	opts := DefaultOptions("myproject")
	cfg := BuildConfig(opts)

	assert.Equal(t, "myproject", cfg.Project)
	assert.Equal(t, "4", cfg.Defaults.Resources.CPUs)
	assert.Equal(t, "8G", cfg.Defaults.Resources.Memory)

	// All languages enabled (nil = enabled).
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"), "go should be enabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("rust"), "rust should be enabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"), "python should be enabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"), "node is always enabled")

	// All tools enabled.
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"), "dev should be enabled")
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("build"), "build should be enabled")

	// All MCP servers enabled.
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"), "filesystem should be enabled")
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("memory"), "memory should be enabled")
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("fetch"), "fetch should be enabled")

	// All agents enabled.
	assert.True(t, cfg.ImageBuild.IsAgentEnabled("codex"), "codex should be enabled")

	// Firewall off by default.
	assert.Nil(t, cfg.Defaults.Firewall)
	assert.Nil(t, cfg.Firewall.Enabled)

	// Git settings.
	require.NotNil(t, cfg.Git.ForwardAgent)
	assert.True(t, *cfg.Git.ForwardAgent)
	require.NotNil(t, cfg.Git.MountGitconfig)
	assert.True(t, *cfg.Git.MountGitconfig)
	require.NotNil(t, cfg.Git.MountGhConfig)
	assert.True(t, *cfg.Git.MountGhConfig)
}

func TestBuildConfig_SelectiveLanguages(t *testing.T) {
	opts := DefaultOptions("selective")
	opts.Languages = []string{"go", "python"} // rust NOT included

	cfg := BuildConfig(opts)

	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"), "go should be enabled")
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("rust"), "rust should be disabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"), "python should be enabled")
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"), "node is always enabled")

	// Rust should be explicitly false.
	require.NotNil(t, cfg.ImageBuild.Languages.Rust)
	assert.False(t, *cfg.ImageBuild.Languages.Rust)
}

func TestBuildConfig_FirewallEnabled(t *testing.T) {
	opts := DefaultOptions("firewall-project")
	opts.Firewall = true

	cfg := BuildConfig(opts)

	require.NotNil(t, cfg.Defaults.Firewall)
	assert.True(t, *cfg.Defaults.Firewall)
	require.NotNil(t, cfg.Firewall.Enabled)
	assert.True(t, *cfg.Firewall.Enabled)
}

func TestBuildConfig_SelectiveTools(t *testing.T) {
	opts := DefaultOptions("tools-project")
	opts.Tools = []string{"dev"} // build NOT included

	cfg := BuildConfig(opts)

	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.False(t, cfg.ImageBuild.IsToolGroupEnabled("build"))

	require.NotNil(t, cfg.ImageBuild.Tools.Build)
	assert.False(t, *cfg.ImageBuild.Tools.Build)
}

func TestBuildConfig_SelectiveMCPServers(t *testing.T) {
	opts := DefaultOptions("mcp-project")
	opts.MCPServers = []string{"filesystem"} // memory and fetch NOT included

	cfg := BuildConfig(opts)

	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("memory"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("fetch"))
}

func TestBuildConfig_SelectiveAgents(t *testing.T) {
	opts := DefaultOptions("agents-project")
	opts.Agents = []string{} // codex NOT included

	cfg := BuildConfig(opts)

	assert.False(t, cfg.ImageBuild.IsAgentEnabled("codex"))

	// Codex should be explicitly false.
	require.NotNil(t, cfg.ImageBuild.Agents.Codex)
	assert.False(t, *cfg.ImageBuild.Agents.Codex)
}

func TestMarshalConfig(t *testing.T) {
	opts := DefaultOptions("myproject")
	cfg := BuildConfig(opts)

	data, err := MarshalConfig(cfg)
	require.NoError(t, err)

	yaml := string(data)
	t.Log("YAML output:\n", yaml)

	assert.Contains(t, yaml, "project: myproject")
	assert.Contains(t, yaml, "node: true")
	assert.Contains(t, yaml, "cpus: \"4\"")
	assert.Contains(t, yaml, "memory: 8G")
	assert.Contains(t, yaml, "forward_agent:")
	assert.Contains(t, yaml, "mount_gitconfig:")
	assert.Contains(t, yaml, "mount_gh_config:")
}

func TestMarshalConfig_FirewallSection(t *testing.T) {
	opts := DefaultOptions("fw-project")
	opts.Firewall = true
	cfg := BuildConfig(opts)

	data, err := MarshalConfig(cfg)
	require.NoError(t, err)

	yaml := string(data)
	assert.Contains(t, yaml, "firewall:")
	assert.Contains(t, yaml, "enabled: true")
}

func TestMarshalConfig_NoFirewallWhenDisabled(t *testing.T) {
	opts := DefaultOptions("no-fw-project")
	opts.Firewall = false
	cfg := BuildConfig(opts)

	data, err := MarshalConfig(cfg)
	require.NoError(t, err)

	yaml := string(data)
	assert.False(t, strings.Contains(yaml, "firewall:"), "firewall section should be omitted when disabled")
}

func TestMarshalConfig_DisabledLanguages(t *testing.T) {
	opts := DefaultOptions("lang-project")
	opts.Languages = []string{"go"} // only go

	cfg := BuildConfig(opts)
	data, err := MarshalConfig(cfg)
	require.NoError(t, err)

	yaml := string(data)
	assert.Contains(t, yaml, "rust: false")
	assert.Contains(t, yaml, "python: false")
	assert.Contains(t, yaml, "node: true")
}

func TestContainsHelper(t *testing.T) {
	assert.True(t, contains([]string{"a", "b", "c"}, "b"))
	assert.False(t, contains([]string{"a", "b", "c"}, "d"))
	assert.False(t, contains([]string{}, "a"))
}

func TestBuildConfig_ReadOnly(t *testing.T) {
	opts := DefaultOptions("ro-project")
	opts.ReadOnly = true

	cfg := BuildConfig(opts)

	require.NotNil(t, cfg.Defaults.ReadOnly)
	assert.True(t, *cfg.Defaults.ReadOnly)
}

func TestBuildConfig_GitDisabled(t *testing.T) {
	opts := DefaultOptions("git-project")
	opts.ForwardAgent = false
	opts.MountGitconfig = false
	opts.MountGhConfig = false

	cfg := BuildConfig(opts)

	require.NotNil(t, cfg.Git.ForwardAgent)
	assert.False(t, *cfg.Git.ForwardAgent)
	require.NotNil(t, cfg.Git.MountGitconfig)
	assert.False(t, *cfg.Git.MountGitconfig)
	require.NotNil(t, cfg.Git.MountGhConfig)
	assert.False(t, *cfg.Git.MountGhConfig)
}

func TestBuildConfig_EmptyLanguages(t *testing.T) {
	opts := DefaultOptions("empty-project")
	opts.Languages = []string{} // no languages selected

	cfg := BuildConfig(opts)

	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("python"))
	// node is always on
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"))
}
