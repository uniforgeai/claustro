// Package wizard converts user choices from the init wizard into a config.Config
// and serializes it to YAML.
package wizard

import (
	"github.com/uniforgeai/claustro/internal/config"
	"gopkg.in/yaml.v3"
)

// Options holds all user choices from the init wizard or CLI flags.
type Options struct {
	Project        string
	Languages      []string // subset of: go, rust, python
	Tools          []string // subset of: dev, build
	MCPServers     []string // subset of: filesystem, memory, fetch
	CPUs           string
	Memory         string
	Firewall       bool
	ReadOnly       bool
	ForwardAgent   bool
	MountGitconfig bool
	MountGhConfig  bool
}

// DefaultOptions returns sensible defaults for the wizard.
func DefaultOptions(project string) Options {
	return Options{
		Project:        project,
		Languages:      []string{"go", "rust", "python"},
		Tools:          []string{"dev", "build"},
		MCPServers:     []string{"filesystem", "memory", "fetch"},
		CPUs:           "4",
		Memory:         "8G",
		Firewall:       false,
		ReadOnly:       false,
		ForwardAgent:   true,
		MountGitconfig: true,
		MountGhConfig:  true,
	}
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// contains reports whether item is in the slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// BuildConfig converts wizard options into a config.Config.
func BuildConfig(opts Options) config.Config {
	cfg := config.Config{}
	cfg.Project = opts.Project

	// Resources defaults.
	cfg.Defaults = config.DefaultsConfig{
		Resources: config.ResourcesConfig{
			CPUs:   opts.CPUs,
			Memory: opts.Memory,
		},
	}

	if opts.Firewall {
		cfg.Defaults.Firewall = boolPtr(true)
		cfg.Firewall = config.FirewallConfig{
			Enabled: boolPtr(true),
		}
	}

	if opts.ReadOnly {
		cfg.Defaults.ReadOnly = boolPtr(true)
	}

	// Git settings.
	cfg.Git = config.GitConfig{
		ForwardAgent:   boolPtr(opts.ForwardAgent),
		MountGitconfig: boolPtr(opts.MountGitconfig),
		MountGhConfig:  boolPtr(opts.MountGhConfig),
	}

	// ImageBuildConfig: items in list = enabled (nil), items NOT in list = disabled (false).
	var imageBuild config.ImageBuildConfig

	// Languages: go, rust, python (node is always enabled).
	if !contains(opts.Languages, "go") {
		imageBuild.Languages.Go = boolPtr(false)
	}
	if !contains(opts.Languages, "rust") {
		imageBuild.Languages.Rust = boolPtr(false)
	}
	if !contains(opts.Languages, "python") {
		imageBuild.Languages.Python = boolPtr(false)
	}

	// Tools: dev, build.
	if !contains(opts.Tools, "dev") {
		imageBuild.Tools.Dev = boolPtr(false)
	}
	if !contains(opts.Tools, "build") {
		imageBuild.Tools.Build = boolPtr(false)
	}

	// MCP servers: filesystem, memory, fetch.
	if !contains(opts.MCPServers, "filesystem") {
		imageBuild.MCPServers.Filesystem = boolPtr(false)
	}
	if !contains(opts.MCPServers, "memory") {
		imageBuild.MCPServers.Memory = boolPtr(false)
	}
	if !contains(opts.MCPServers, "fetch") {
		imageBuild.MCPServers.Fetch = boolPtr(false)
	}

	cfg.ImageBuild = imageBuild
	return cfg
}

// marshalableConfig is a clean struct for YAML serialization that avoids yaml.Node issues.
type marshalableConfig struct {
	Project  string                  `yaml:"project"`
	Image    marshalableImage        `yaml:"image"`
	Defaults marshalableDefaults     `yaml:"defaults"`
	Firewall *marshalableFirewall    `yaml:"firewall,omitempty"`
	Git      marshalableGit          `yaml:"git"`
}

type marshalableImage struct {
	Languages  marshalableLanguages   `yaml:"languages"`
	Tools      marshalableTools       `yaml:"tools"`
	MCPServers marshalableMCPServers  `yaml:"mcp_servers"`
}

type marshalableLanguages struct {
	Node   bool  `yaml:"node"`
	Go     *bool `yaml:"go,omitempty"`
	Rust   *bool `yaml:"rust,omitempty"`
	Python *bool `yaml:"python,omitempty"`
}

type marshalableTools struct {
	Dev   *bool `yaml:"dev,omitempty"`
	Build *bool `yaml:"build,omitempty"`
}

type marshalableMCPServers struct {
	Filesystem *bool `yaml:"filesystem,omitempty"`
	Memory     *bool `yaml:"memory,omitempty"`
	Fetch      *bool `yaml:"fetch,omitempty"`
}

type marshalableDefaults struct {
	Resources marshalableResources `yaml:"resources"`
}

type marshalableResources struct {
	CPUs   string `yaml:"cpus"`
	Memory string `yaml:"memory"`
}

type marshalableFirewall struct {
	Enabled bool `yaml:"enabled"`
}

type marshalableGit struct {
	ForwardAgent   *bool `yaml:"forward_agent,omitempty"`
	MountGitconfig *bool `yaml:"mount_gitconfig,omitempty"`
	MountGhConfig  *bool `yaml:"mount_gh_config,omitempty"`
}

// MarshalConfig serializes a Config to YAML bytes suitable for claustro.yaml.
// Node is always shown as true in output.
func MarshalConfig(cfg config.Config) ([]byte, error) {
	mc := marshalableConfig{
		Project: cfg.Project,
		Image: marshalableImage{
			Languages: marshalableLanguages{
				Node:   true, // node is always enabled
				Go:     cfg.ImageBuild.Languages.Go,
				Rust:   cfg.ImageBuild.Languages.Rust,
				Python: cfg.ImageBuild.Languages.Python,
			},
			Tools: marshalableTools{
				Dev:   cfg.ImageBuild.Tools.Dev,
				Build: cfg.ImageBuild.Tools.Build,
			},
			MCPServers: marshalableMCPServers{
				Filesystem: cfg.ImageBuild.MCPServers.Filesystem,
				Memory:     cfg.ImageBuild.MCPServers.Memory,
				Fetch:      cfg.ImageBuild.MCPServers.Fetch,
			},
		},
		Defaults: marshalableDefaults{
			Resources: marshalableResources{
				CPUs:   cfg.Defaults.Resources.CPUs,
				Memory: cfg.Defaults.Resources.Memory,
			},
		},
		Git: marshalableGit{
			ForwardAgent:   cfg.Git.ForwardAgent,
			MountGitconfig: cfg.Git.MountGitconfig,
			MountGhConfig:  cfg.Git.MountGhConfig,
		},
	}

	if cfg.Firewall.Enabled != nil && *cfg.Firewall.Enabled {
		mc.Firewall = &marshalableFirewall{Enabled: true}
	} else if cfg.Defaults.Firewall != nil && *cfg.Defaults.Firewall {
		mc.Firewall = &marshalableFirewall{Enabled: true}
	}

	return yaml.Marshal(mc)
}
