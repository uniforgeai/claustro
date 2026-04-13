// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package config

// LanguagesConfig controls which language runtimes are installed in the sandbox image.
// A nil pointer means the language is enabled (opt-out model).
type LanguagesConfig struct {
	Node   *bool `yaml:"node"`
	Go     *bool `yaml:"go"`
	Rust   *bool `yaml:"rust"`
	Python *bool `yaml:"python"`
}

// ToolsConfig controls which tool groups are installed in the sandbox image.
// A nil pointer means the tool group is enabled (opt-out model), except Voice
// which defaults to disabled (nil = false) since it adds significant image size.
type ToolsConfig struct {
	Dev   *bool `yaml:"dev"`
	Build *bool `yaml:"build"`
	Voice *bool `yaml:"voice"`
}

// MCPServersConfig controls which built-in MCP servers are installed in the sandbox image.
// A nil pointer means the server is enabled (opt-out model).
type MCPServersConfig struct {
	Filesystem *bool `yaml:"filesystem"`
	Memory     *bool `yaml:"memory"`
	Fetch      *bool `yaml:"fetch"`
}

// ImageBuildConfig controls what gets installed in the sandbox image during build.
// All fields use an opt-out model: nil means enabled, false means disabled.
type ImageBuildConfig struct {
	Languages  LanguagesConfig  `yaml:"languages"`
	Tools      ToolsConfig      `yaml:"tools"`
	MCPServers MCPServersConfig `yaml:"mcp_servers"`
}

// DefaultImageBuildConfig returns an ImageBuildConfig with all features enabled (nil pointers).
func DefaultImageBuildConfig() ImageBuildConfig {
	return ImageBuildConfig{} // nil pointers = all defaults to true
}

// IsLanguageEnabled reports whether the given language runtime should be installed.
// node is always enabled. For go/rust/python, nil means true, false means false.
// Unknown language names return false.
func (c *ImageBuildConfig) IsLanguageEnabled(lang string) bool {
	switch lang {
	case "node":
		return true // node is always enabled regardless of config
	case "go":
		return c.Languages.Go == nil || *c.Languages.Go
	case "rust":
		return c.Languages.Rust == nil || *c.Languages.Rust
	case "python":
		return c.Languages.Python == nil || *c.Languages.Python
	default:
		return false
	}
}

// IsToolGroupEnabled reports whether the given tool group should be installed.
// nil means true (enabled), false means disabled. Unknown groups return false.
func (c *ImageBuildConfig) IsToolGroupEnabled(group string) bool {
	switch group {
	case "dev":
		return c.Tools.Dev == nil || *c.Tools.Dev
	case "build":
		return c.Tools.Build == nil || *c.Tools.Build
	case "voice":
		// Voice defaults to disabled (opt-in) — nil means false.
		return c.Tools.Voice != nil && *c.Tools.Voice
	default:
		return false
	}
}

// IsMCPServerEnabled reports whether the given built-in MCP server should be installed.
// nil means true (enabled), false means disabled. Unknown servers return false.
func (c *ImageBuildConfig) IsMCPServerEnabled(server string) bool {
	switch server {
	case "filesystem":
		return c.MCPServers.Filesystem == nil || *c.MCPServers.Filesystem
	case "memory":
		return c.MCPServers.Memory == nil || *c.MCPServers.Memory
	case "fetch":
		return c.MCPServers.Fetch == nil || *c.MCPServers.Fetch
	default:
		return false
	}
}
