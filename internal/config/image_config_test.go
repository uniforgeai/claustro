package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultImageBuildConfig_AllEnabled(t *testing.T) {
	cfg := DefaultImageBuildConfig()

	// All languages should be enabled by default (nil = true)
	assert.True(t, cfg.IsLanguageEnabled("node"), "node should be enabled by default")
	assert.True(t, cfg.IsLanguageEnabled("go"), "go should be enabled by default")
	assert.True(t, cfg.IsLanguageEnabled("rust"), "rust should be enabled by default")
	assert.True(t, cfg.IsLanguageEnabled("python"), "python should be enabled by default")

	// All tool groups should be enabled by default
	assert.True(t, cfg.IsToolGroupEnabled("dev"), "dev tools should be enabled by default")
	assert.True(t, cfg.IsToolGroupEnabled("build"), "build tools should be enabled by default")

	// All MCP servers should be enabled by default
	assert.True(t, cfg.IsMCPServerEnabled("filesystem"), "filesystem MCP should be enabled by default")
	assert.True(t, cfg.IsMCPServerEnabled("memory"), "memory MCP should be enabled by default")
	assert.True(t, cfg.IsMCPServerEnabled("fetch"), "fetch MCP should be enabled by default")
}

func TestImageBuildConfig_DisableLanguages(t *testing.T) {
	f := false

	cfg := ImageBuildConfig{
		Languages: LanguagesConfig{
			Go:     &f,
			Rust:   &f,
			Python: &f,
		},
	}

	// node is always true, even without explicit setting
	assert.True(t, cfg.IsLanguageEnabled("node"))
	assert.False(t, cfg.IsLanguageEnabled("go"))
	assert.False(t, cfg.IsLanguageEnabled("rust"))
	assert.False(t, cfg.IsLanguageEnabled("python"))
}

func TestImageBuildConfig_NodeAlwaysTrue(t *testing.T) {
	f := false
	cfg := ImageBuildConfig{
		Languages: LanguagesConfig{
			Node: &f,
		},
	}
	// Node is always enabled regardless of the flag value
	assert.True(t, cfg.IsLanguageEnabled("node"), "node should always be enabled")
}

func TestImageBuildConfig_DisableTools(t *testing.T) {
	f := false

	cfg := ImageBuildConfig{
		Tools: ToolsConfig{
			Dev:   &f,
			Build: &f,
		},
	}

	assert.False(t, cfg.IsToolGroupEnabled("dev"))
	assert.False(t, cfg.IsToolGroupEnabled("build"))
}

func TestImageBuildConfig_DisableMCPServers(t *testing.T) {
	f := false

	cfg := ImageBuildConfig{
		MCPServers: MCPServersConfig{
			Filesystem: &f,
			Memory:     &f,
			Fetch:      &f,
		},
	}

	assert.False(t, cfg.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.IsMCPServerEnabled("memory"))
	assert.False(t, cfg.IsMCPServerEnabled("fetch"))
}

func TestImageBuildConfig_UnknownKeysReturnFalse(t *testing.T) {
	cfg := DefaultImageBuildConfig()

	assert.False(t, cfg.IsLanguageEnabled("java"), "unknown language should return false")
	assert.False(t, cfg.IsLanguageEnabled("ruby"), "unknown language should return false")
	assert.False(t, cfg.IsLanguageEnabled(""), "empty language should return false")

	assert.False(t, cfg.IsToolGroupEnabled("test"), "unknown tool group should return false")
	assert.False(t, cfg.IsToolGroupEnabled(""), "empty tool group should return false")

	assert.False(t, cfg.IsMCPServerEnabled("github"), "unknown MCP server should return false")
	assert.False(t, cfg.IsMCPServerEnabled(""), "empty MCP server should return false")
}

func TestLoad_ImageBuildConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  languages:
    node: true
    go: false
    rust: false
    python: true
  tools:
    dev: true
    build: false
  mcp_servers:
    filesystem: true
    memory: false
    fetch: true
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("rust"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("python"))

	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.False(t, cfg.ImageBuild.IsToolGroupEnabled("build"))

	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("memory"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("fetch"))
}

func TestLoad_MissingImageBlock_AllEnabled(t *testing.T) {
	dir := t.TempDir()
	content := `project: my-project`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	// When image block is absent, all ImageBuild fields should be enabled by default
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"))
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.True(t, cfg.ImageBuild.IsToolGroupEnabled("dev"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
}

func TestLoad_ImageBuildConfigWithExtra(t *testing.T) {
	dir := t.TempDir()
	content := `
image:
  extra:
    - run: apt-get install -y curl
  languages:
    go: false
  mcp_servers:
    memory: false
`
	err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)

	// ImageConfig.Extra should still be decoded
	require.Len(t, cfg.ImageConfig.Extra, 1)
	assert.Equal(t, "apt-get install -y curl", cfg.ImageConfig.Extra[0].Run)

	// ImageBuild should also be decoded
	assert.True(t, cfg.ImageBuild.IsLanguageEnabled("node"))
	assert.False(t, cfg.ImageBuild.IsLanguageEnabled("go"))
	assert.True(t, cfg.ImageBuild.IsMCPServerEnabled("filesystem"))
	assert.False(t, cfg.ImageBuild.IsMCPServerEnabled("memory"))
}
