// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func TestRenderDockerfile_AllEnabled(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, out)

	// Base always-present items
	assert.Contains(t, out, "FROM ubuntu:24.04")
	assert.Contains(t, out, "nodesource")
	assert.Contains(t, out, "claude.ai/install.sh")
	assert.Contains(t, out, "claustro-init")
	assert.Contains(t, out, "ccstatusline")
	assert.Contains(t, out, "useradd")

	// Go
	assert.Contains(t, out, "go.dev/dl/go1.24.2")
	assert.Contains(t, out, "gopls")

	// Rust
	assert.Contains(t, out, "rustup.rs")
	assert.Contains(t, out, "cp -r /root/.cargo")

	// Python
	assert.Contains(t, out, "python3-venv")

	// Dev tools
	assert.Contains(t, out, "ripgrep")

	// Build tools
	assert.Contains(t, out, "build-essential")

	// MCP servers
	assert.Contains(t, out, "@modelcontextprotocol/server-filesystem")
	assert.Contains(t, out, "@modelcontextprotocol/server-memory")
	assert.Contains(t, out, "mcp-server-fetch")
}

func TestRenderDockerfile_MinimalConfig(t *testing.T) {
	f := false
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     &f,
			Rust:   &f,
			Python: &f,
		},
		Tools: config.ToolsConfig{
			Dev:   &f,
			Build: &f,
		},
		MCPServers: config.MCPServersConfig{
			Filesystem: &f,
			Memory:     &f,
			Fetch:      &f,
		},
	}
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, out)

	// Base items still present
	assert.Contains(t, out, "FROM ubuntu:24.04")
	assert.Contains(t, out, "nodesource")
	assert.Contains(t, out, "claude.ai/install.sh")
	assert.Contains(t, out, "ccstatusline")
	assert.Contains(t, out, "claustro-init")

	// Conditional items absent
	assert.NotContains(t, out, "go.dev/dl")
	assert.NotContains(t, out, "rustup.rs")
	assert.NotContains(t, out, "python3-venv")
	assert.NotContains(t, out, "ripgrep")
	assert.NotContains(t, out, "build-essential")
	assert.NotContains(t, out, "@modelcontextprotocol/server-filesystem")
	assert.NotContains(t, out, "@modelcontextprotocol/server-memory")
	assert.NotContains(t, out, "mcp-server-fetch")
}

func TestRenderDockerfile_SelectiveLanguages(t *testing.T) {
	f := false
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     nil, // enabled
			Rust:   &f,  // disabled
			Python: nil, // enabled
		},
	}
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	assert.Contains(t, out, "go.dev/dl/go1.24.2")
	assert.Contains(t, out, "gopls")
	assert.Contains(t, out, "python3-venv")

	assert.NotContains(t, out, "rustup.rs")
	assert.NotContains(t, out, "cp -r /root/.cargo")
}

func TestRenderDockerfile_NodeAlwaysPresent(t *testing.T) {
	f := false
	cfg := config.ImageBuildConfig{
		Languages: config.LanguagesConfig{
			Go:     &f,
			Rust:   &f,
			Python: &f,
		},
	}
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)

	// Node is always installed regardless of config
	assert.Contains(t, out, "nodesource")
	assert.Contains(t, out, "nodejs")
}

func TestRenderDockerfile_VoiceMode(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	tr := true
	cfg.Tools.Voice = &tr

	content, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.Contains(t, content, "sox")
	assert.Contains(t, content, "Install SoX for Claude Code voice mode")
}

func TestRenderDockerfile_NoVoiceMode(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	// voice defaults to false (nil = disabled for voice)

	content, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.NotContains(t, content, "sox")
}

func TestRenderDockerfile_IsValidDockerfile(t *testing.T) {
	cfg := config.DefaultImageBuildConfig()
	out, err := RenderDockerfile(&cfg)
	require.NoError(t, err)
	assert.True(t, len(out) > 0)
	// Valid Dockerfiles start with FROM
	assert.Equal(t, "FROM ", out[:5])
}
