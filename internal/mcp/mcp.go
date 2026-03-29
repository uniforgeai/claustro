// Package mcp builds and merges MCP server configurations for Claude Code sandboxes.
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/uniforgeai/claustro/internal/config"
)

// MCPConfigPath is the path inside the container where Claude Code reads MCP config.
const MCPConfigPath = "/home/sandbox/.claude/mcp.json"

// StdioServer represents a single stdio MCP server entry in mcp.json.
type StdioServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// SSEServer represents a single SSE MCP server entry in mcp.json.
type SSEServer struct {
	URL string `json:"url"`
}

// ServerEntry is a single entry in mcp.json that can be either stdio or SSE.
type ServerEntry struct {
	// Stdio fields
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	// SSE field
	URL string `json:"url,omitempty"`
}

// Config represents the full mcp.json structure that Claude Code reads.
type Config struct {
	MCPServers map[string]ServerEntry `json:"mcpServers"`
}

// DefaultConfig returns the built-in MCP server config with the three pre-installed servers.
func DefaultConfig() Config {
	return Config{
		MCPServers: map[string]ServerEntry{
			"filesystem": {
				Command: "npx",
				Args:    []string{"-y", "@anthropic-ai/mcp-server-filesystem", "/workspace"},
			},
			"memory": {
				Command: "npx",
				Args:    []string{"-y", "@anthropic-ai/mcp-server-memory"},
			},
			"fetch": {
				Command: "npx",
				Args:    []string{"-y", "@anthropic-ai/mcp-server-fetch"},
			},
		},
	}
}

// FromProjectConfig converts claustro.yaml MCP stdio entries into a Config.
func FromProjectConfig(stdio map[string]config.MCPStdio) Config {
	if len(stdio) == 0 {
		return Config{}
	}
	servers := make(map[string]ServerEntry, len(stdio))
	for name, s := range stdio {
		servers[name] = ServerEntry{
			Command: s.Command,
			Args:    s.Args,
		}
	}
	return Config{MCPServers: servers}
}

// Merge combines multiple configs. Later configs take precedence on key collision.
func Merge(configs ...Config) Config {
	merged := Config{MCPServers: make(map[string]ServerEntry)}
	for _, cfg := range configs {
		for name, server := range cfg.MCPServers {
			merged.MCPServers[name] = server
		}
	}
	return merged
}

// JSON returns the indented JSON representation suitable for writing to mcp.json.
func (c Config) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling mcp config: %w", err)
	}
	return data, nil
}

// ParseJSON parses the contents of an mcp.json file.
func ParseJSON(data []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing mcp config: %w", err)
	}
	return cfg, nil
}

// WriteCommand returns the shell command (for Docker exec) that writes the given
// config to the specified path inside a container.
func WriteCommand(cfg Config, path string) ([]string, error) {
	data, err := cfg.JSON()
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf("cat > %s << 'CLAUSTRO_MCP_EOF'\n%s\nCLAUSTRO_MCP_EOF", path, string(data))
	return []string{"sh", "-c", script}, nil
}
