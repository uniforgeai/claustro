package config

import "fmt"

// SandboxConfig is the fully resolved, flat configuration for a single sandbox.
// All merging (defaults, named sandbox, dotenv, CLI overrides) is already applied.
type SandboxConfig struct {
	Name          string
	Workdir       string
	Mounts        []Mount
	Env           map[string]string
	Firewall      bool
	ReadOnly      bool
	IsolatedState bool
	CPUs          string
	Memory        string
	ImageName     string
}

// CLIOverrides holds values provided via CLI flags that override config file settings.
type CLIOverrides struct {
	Name          string
	Workdir       string
	Mounts        []string
	Env           map[string]string
	ReadOnly      *bool
	IsolatedState bool
}

// Resolve merges defaults, named sandbox config, dotenv, and CLI overrides into a
// flat SandboxConfig. Resolution order (last wins):
// spec defaults -> config defaults -> named sandbox -> dotenv -> per-sandbox env -> CLI overrides.
func (c *Config) Resolve(projectRoot string, cli CLIOverrides, dotenv map[string]string) (*SandboxConfig, error) {
	sc := &SandboxConfig{
		Name: cli.Name,
		Env:  make(map[string]string),
	}

	// Apply config defaults.
	if c.Defaults.Firewall != nil {
		sc.Firewall = *c.Defaults.Firewall
	}
	if c.Defaults.ReadOnly != nil {
		sc.ReadOnly = *c.Defaults.ReadOnly
	}
	sc.CPUs = c.Defaults.Resources.CPUs
	sc.Memory = c.Defaults.Resources.Memory
	sc.ImageName = c.ImageName

	// Apply named sandbox config if it exists.
	if cli.Name != "" {
		if sbox, ok := c.Sandboxes[cli.Name]; ok {
			sc.Workdir = sbox.Workdir
			for _, raw := range sbox.Mounts {
				m, err := ParseMount(raw, projectRoot)
				if err != nil {
					return nil, fmt.Errorf("sandbox %q mount: %w", cli.Name, err)
				}
				sc.Mounts = append(sc.Mounts, m)
			}
			// Dotenv first, then per-sandbox env overwrites.
			for k, v := range dotenv {
				sc.Env[k] = v
			}
			for k, v := range sbox.Env {
				sc.Env[k] = v
			}
		} else {
			// No matching sandbox definition — just apply dotenv.
			for k, v := range dotenv {
				sc.Env[k] = v
			}
		}
	} else {
		for k, v := range dotenv {
			sc.Env[k] = v
		}
	}

	// CLI overrides win.
	if cli.Workdir != "" {
		sc.Workdir = cli.Workdir
	}
	for _, raw := range cli.Mounts {
		m, err := ParseMount(raw, projectRoot)
		if err != nil {
			return nil, fmt.Errorf("CLI mount: %w", err)
		}
		sc.Mounts = append(sc.Mounts, m)
	}
	for k, v := range cli.Env {
		sc.Env[k] = v
	}

	// CLI readonly override.
	if cli.ReadOnly != nil {
		sc.ReadOnly = *cli.ReadOnly
	}
	sc.IsolatedState = cli.IsolatedState

	return sc, nil
}
