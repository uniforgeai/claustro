package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uniforgeai/claustro/internal/config"
)

func TestEffectivePort(t *testing.T) {
	assert.Equal(t, 8000, effectivePort(0))
	assert.Equal(t, 3000, effectivePort(3000))
	assert.Equal(t, 8000, effectivePort(8000))
}

func TestEndpointURL(t *testing.T) {
	assert.Equal(t, "http://postgres:8000/sse", EndpointURL("postgres", 0))
	assert.Equal(t, "http://browser:3000/sse", EndpointURL("browser", 3000))
	assert.Equal(t, "http://redis:8000/sse", EndpointURL("redis", 8000))
}

func TestSSEEntries(t *testing.T) {
	servers := map[string]config.MCPSSE{
		"postgres": {Image: "pg:latest", Port: 8000},
		"browser":  {Image: "br:latest", Port: 3000},
	}
	cfg := SSEEntries(servers)

	assert.Len(t, cfg.MCPServers, 2)
	assert.Equal(t, "http://postgres:8000/sse", cfg.MCPServers["postgres"].URL)
	assert.Equal(t, "http://browser:3000/sse", cfg.MCPServers["browser"].URL)
	// SSE entries should not have command/args
	assert.Empty(t, cfg.MCPServers["postgres"].Command)
	assert.Empty(t, cfg.MCPServers["postgres"].Args)
}

func TestSSEEntries_DefaultPort(t *testing.T) {
	servers := map[string]config.MCPSSE{
		"noport": {Image: "img:latest", Port: 0},
	}
	cfg := SSEEntries(servers)

	assert.Equal(t, "http://noport:8000/sse", cfg.MCPServers["noport"].URL)
}

func TestSSEEntries_Empty(t *testing.T) {
	cfg := SSEEntries(nil)
	assert.Empty(t, cfg.MCPServers)
}

func TestSSEEntries_MergedWithStdio(t *testing.T) {
	// Verify SSE entries can be merged with stdio entries
	stdio := DefaultConfig()
	sse := SSEEntries(map[string]config.MCPSSE{
		"postgres": {Image: "pg:latest", Port: 8000},
	})

	merged := Merge(stdio, sse)
	// Should have 3 stdio + 1 SSE = 4 entries
	assert.Len(t, merged.MCPServers, 4)
	assert.Equal(t, "http://postgres:8000/sse", merged.MCPServers["postgres"].URL)
	assert.Equal(t, "npx", merged.MCPServers["filesystem"].Command)
}
