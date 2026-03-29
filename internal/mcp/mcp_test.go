package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.Len(t, cfg.MCPServers, 3)

	fs, ok := cfg.MCPServers["filesystem"]
	require.True(t, ok, "filesystem server must exist")
	assert.Equal(t, "npx", fs.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"}, fs.Args)

	mem, ok := cfg.MCPServers["memory"]
	require.True(t, ok, "memory server must exist")
	assert.Equal(t, "npx", mem.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-memory"}, mem.Args)

	fetch, ok := cfg.MCPServers["fetch"]
	require.True(t, ok, "fetch server must exist")
	assert.Equal(t, "mcp-server-fetch", fetch.Command)
	assert.Nil(t, fetch.Args)
}

func TestMerge_SingleConfig(t *testing.T) {
	cfg := DefaultConfig()
	merged := Merge(cfg)
	assert.Equal(t, cfg, merged)
}

func TestMerge_LaterOverridesEarlier(t *testing.T) {
	base := Config{
		MCPServers: map[string]ServerEntry{
			"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"}},
		},
	}
	override := Config{
		MCPServers: map[string]ServerEntry{
			"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace", "/data"}},
		},
	}
	merged := Merge(base, override)
	require.Len(t, merged.MCPServers, 1)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace", "/data"}, merged.MCPServers["filesystem"].Args)
}

func TestMerge_AddsNewServers(t *testing.T) {
	base := DefaultConfig()
	extra := Config{
		MCPServers: map[string]ServerEntry{
			"custom": {Command: "node", Args: []string{"/workspace/server.js"}},
		},
	}
	merged := Merge(base, extra)
	assert.Len(t, merged.MCPServers, 4)
	assert.Contains(t, merged.MCPServers, "custom")
	assert.Contains(t, merged.MCPServers, "filesystem")
}

func TestMerge_ThreeLayers(t *testing.T) {
	defaults := DefaultConfig()
	host := Config{
		MCPServers: map[string]ServerEntry{
			"host-tool":  {Command: "host-cmd", Args: []string{"a"}},
			"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/host-path"}},
		},
	}
	project := Config{
		MCPServers: map[string]ServerEntry{
			"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/project-path"}},
		},
	}
	merged := Merge(defaults, host, project)

	// Project wins for filesystem.
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/project-path"}, merged.MCPServers["filesystem"].Args)
	// Host tool preserved.
	assert.Contains(t, merged.MCPServers, "host-tool")
	// Defaults preserved.
	assert.Contains(t, merged.MCPServers, "memory")
	assert.Contains(t, merged.MCPServers, "fetch")
}

func TestMerge_EmptyConfigs(t *testing.T) {
	merged := Merge(Config{}, Config{})
	assert.Empty(t, merged.MCPServers)
}

func TestMerge_NoArgs(t *testing.T) {
	merged := Merge()
	assert.Empty(t, merged.MCPServers)
}

func TestFromProjectConfig(t *testing.T) {
	stdio := map[string]config.MCPStdio{
		"custom":     {Command: "node", Args: []string{"/workspace/server.js"}},
		"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace", "/extra"}},
	}
	cfg := FromProjectConfig(stdio)

	require.Len(t, cfg.MCPServers, 2)
	assert.Equal(t, "node", cfg.MCPServers["custom"].Command)
	assert.Equal(t, []string{"/workspace/server.js"}, cfg.MCPServers["custom"].Args)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace", "/extra"}, cfg.MCPServers["filesystem"].Args)
}

func TestFromProjectConfig_Empty(t *testing.T) {
	cfg := FromProjectConfig(nil)
	assert.Empty(t, cfg.MCPServers)
}

func TestJSON(t *testing.T) {
	cfg := Config{
		MCPServers: map[string]ServerEntry{
			"test": {Command: "echo", Args: []string{"hello"}},
		},
	}
	data, err := cfg.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"mcpServers"`)
	assert.Contains(t, string(data), `"test"`)
	assert.Contains(t, string(data), `"command"`)
}

func TestParseJSON(t *testing.T) {
	original := DefaultConfig()
	data, err := original.JSON()
	require.NoError(t, err)

	parsed, err := ParseJSON(data)
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestParseJSON_Invalid(t *testing.T) {
	_, err := ParseJSON([]byte(`not json`))
	assert.Error(t, err)
}

func TestParseJSON_Empty(t *testing.T) {
	cfg, err := ParseJSON([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, cfg.MCPServers)
}

func TestWriteCommand(t *testing.T) {
	cfg := Config{
		MCPServers: map[string]ServerEntry{
			"test": {Command: "echo", Args: []string{"hello"}},
		},
	}
	cmd, err := WriteCommand(cfg, "/home/sandbox/.claude/mcp.json")
	require.NoError(t, err)
	require.Len(t, cmd, 3)
	assert.Equal(t, "sh", cmd[0])
	assert.Equal(t, "-c", cmd[1])
	assert.Contains(t, cmd[2], `"mcpServers"`)
	assert.Contains(t, cmd[2], "/home/sandbox/.claude/mcp.json")
}
