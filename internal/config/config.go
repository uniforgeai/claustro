// Package config loads per-project claustro configuration from claustro.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the full claustro project configuration from claustro.yaml.
type Config struct {
	Project  string                `yaml:"project"`
	RawImage yaml.Node             `yaml:"image"`
	Defaults DefaultsConfig        `yaml:"defaults"`
	Sandboxes map[string]SandboxDef `yaml:"sandboxes"`
	Firewall FirewallConfig        `yaml:"firewall"`
	MCP      MCPConfig             `yaml:"mcp"`
	Git      GitConfig             `yaml:"git"`

	// Parsed image fields (populated by postProcess).
	ImageName   string
	ImageConfig ImageConfig
	ImageBuild  ImageBuildConfig `yaml:"-"`
}

// DefaultsConfig holds project-wide sandbox defaults.
type DefaultsConfig struct {
	Firewall  *bool           `yaml:"firewall"`
	ReadOnly  *bool           `yaml:"readonly"`
	Resources ResourcesConfig `yaml:"resources"`
}

// ResourcesConfig specifies container resource limits.
type ResourcesConfig struct {
	CPUs   string `yaml:"cpus"`
	Memory string `yaml:"memory"`
}

// SandboxDef is a named sandbox configuration in claustro.yaml.
type SandboxDef struct {
	Workdir string            `yaml:"workdir"`
	Mounts  []string          `yaml:"mounts"`
	Env     map[string]string `yaml:"env"`
}

// FirewallConfig controls egress filtering.
type FirewallConfig struct {
	Enabled *bool    `yaml:"enabled"`
	Allow   []string `yaml:"allow"`
}

// MCPConfig holds MCP server definitions.
type MCPConfig struct {
	Stdio map[string]MCPStdio `yaml:"stdio"`
	SSE   map[string]MCPSSE   `yaml:"sse"`
}

// MCPStdio is a stdio-based MCP server.
type MCPStdio struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// MCPSSE is an SSE-based MCP server running as a sibling container.
type MCPSSE struct {
	Image string            `yaml:"image"`
	Port  int               `yaml:"port"`
	Env   map[string]string `yaml:"env"`
}

// ImageConfig configures how the sandbox image is built for this project.
type ImageConfig struct {
	Extra []ExtraStep `yaml:"extra"`
}

// ExtraStep is a single additional Dockerfile RUN step for the project's image extension.
type ExtraStep struct {
	Run string `yaml:"run"`
}

// GitConfig controls which host git/GitHub credentials are forwarded into the sandbox.
type GitConfig struct {
	ForwardAgent   *bool `yaml:"forward_agent"`
	MountGitconfig *bool `yaml:"mount_gitconfig"`
	MountGhConfig  *bool `yaml:"mount_gh_config"`
	MountSSHDir    *bool `yaml:"mount_ssh_dir"`
}

// IsForwardAgent returns true unless explicitly disabled in config.
func (g *GitConfig) IsForwardAgent() bool { return g.ForwardAgent == nil || *g.ForwardAgent }

// IsMountGitconfig returns true unless explicitly disabled in config.
func (g *GitConfig) IsMountGitconfig() bool { return g.MountGitconfig == nil || *g.MountGitconfig }

// IsMountGhConfig returns true unless explicitly disabled in config.
func (g *GitConfig) IsMountGhConfig() bool { return g.MountGhConfig == nil || *g.MountGhConfig }

// IsMountSSHDir returns true only when explicitly enabled in config (default: false).
func (g *GitConfig) IsMountSSHDir() bool { return g.MountSSHDir != nil && *g.MountSSHDir }

// Load reads claustro.yaml from projectPath and returns the parsed Config.
// If claustro.yaml is not present, an empty Config is returned with no error.
func Load(projectPath string) (*Config, error) {
	path := filepath.Join(projectPath, "claustro.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading claustro.yaml: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing claustro.yaml: %w", err)
	}
	if err := cfg.postProcess(); err != nil {
		return nil, fmt.Errorf("parsing claustro.yaml image field: %w", err)
	}
	results := cfg.Validate()
	if errs := cfg.Errors(results); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
		}
		return nil, fmt.Errorf("invalid claustro.yaml: %s", strings.Join(msgs, "; "))
	}
	return &cfg, nil
}

// postProcess handles the dual image: syntax.
// "image: name:tag" (scalar) sets ImageName.
// "image:\n  extra: [...]" (mapping) sets ImageConfig.
func (c *Config) postProcess() error {
	if c.RawImage.IsZero() {
		return nil
	}
	switch c.RawImage.Kind {
	case yaml.ScalarNode:
		c.ImageName = c.RawImage.Value
	case yaml.MappingNode:
		if err := c.RawImage.Decode(&c.ImageConfig); err != nil {
			return fmt.Errorf("decoding image config: %w", err)
		}
		if err := c.RawImage.Decode(&c.ImageBuild); err != nil {
			return fmt.Errorf("decoding image build config: %w", err)
		}
	default:
		return fmt.Errorf("image field must be a string or mapping, got %v", c.RawImage.Kind)
	}
	return nil
}
