// Package config loads per-project claustro configuration from sandbox.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the full claustro project configuration from sandbox.yaml.
type Config struct {
	Image ImageConfig `yaml:"image"`
	Git   GitConfig   `yaml:"git"`
}

// GitConfig controls which host git/GitHub credentials are forwarded into the sandbox.
// All forwarding is enabled by default (opt-out model). Set a field to false in
// sandbox.yaml to disable it.
type GitConfig struct {
	// ForwardAgent forwards the host SSH agent socket (SSH_AUTH_SOCK) when present.
	// Default: true.
	ForwardAgent *bool `yaml:"forward_agent"`
	// MountGitconfig mounts ~/.gitconfig read-only into the sandbox.
	// Default: true.
	MountGitconfig *bool `yaml:"mount_gitconfig"`
	// MountGhConfig mounts ~/.config/gh/ read-write into the sandbox when the directory exists.
	// Default: true.
	MountGhConfig *bool `yaml:"mount_gh_config"`
	// MountSSHDir mounts ~/.ssh/ read-only into the sandbox.
	// Disabled by default — SSH agent forwarding is preferred.
	// Default: false.
	MountSSHDir *bool `yaml:"mount_ssh_dir"`
}

// IsForwardAgent returns true unless explicitly disabled in config.
func (g *GitConfig) IsForwardAgent() bool { return g.ForwardAgent == nil || *g.ForwardAgent }

// IsMountGitconfig returns true unless explicitly disabled in config.
func (g *GitConfig) IsMountGitconfig() bool { return g.MountGitconfig == nil || *g.MountGitconfig }

// IsMountGhConfig returns true unless explicitly disabled in config.
func (g *GitConfig) IsMountGhConfig() bool { return g.MountGhConfig == nil || *g.MountGhConfig }

// IsMountSSHDir returns true only when explicitly enabled in config (default: false).
func (g *GitConfig) IsMountSSHDir() bool { return g.MountSSHDir != nil && *g.MountSSHDir }

// ImageConfig configures how the sandbox image is built for this project.
type ImageConfig struct {
	Extra []ExtraStep `yaml:"extra"`
}

// ExtraStep is a single additional Dockerfile RUN step for the project's image extension.
type ExtraStep struct {
	Run string `yaml:"run"`
}

// Load reads sandbox.yaml from projectPath and returns the parsed Config.
// If sandbox.yaml is not present, an empty Config is returned with no error.
func Load(projectPath string) (*Config, error) {
	path := filepath.Join(projectPath, "sandbox.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading sandbox.yaml: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing sandbox.yaml: %w", err)
	}
	return &cfg, nil
}
