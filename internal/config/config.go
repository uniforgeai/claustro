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
}

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
