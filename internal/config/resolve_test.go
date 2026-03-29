package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_DefaultsOnly(t *testing.T) {
	cfg := &Config{}
	sc, err := cfg.Resolve("/project", CLIOverrides{Name: "test"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "test", sc.Name)
	assert.False(t, sc.Firewall)
	assert.False(t, sc.ReadOnly)
	assert.Empty(t, sc.Workdir)
	assert.Empty(t, sc.Mounts)
}

func TestResolve_DefaultsPlusSandboxOverride(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{
			Firewall:  boolPtr(true),
			Resources: ResourcesConfig{CPUs: "2", Memory: "4G"},
		},
		Sandboxes: map[string]SandboxDef{
			"api": {
				Workdir: "./services/api",
				Mounts:  []string{"./libs:/workspace/libs:ro"},
				Env:     map[string]string{"DB": "postgres"},
			},
		},
	}

	sc, err := cfg.Resolve("/project", CLIOverrides{Name: "api"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "api", sc.Name)
	assert.True(t, sc.Firewall)
	assert.Equal(t, "2", sc.CPUs)
	assert.Equal(t, "4G", sc.Memory)
	assert.Equal(t, "./services/api", sc.Workdir)
	require.Len(t, sc.Mounts, 1)
	assert.Equal(t, "/project/libs", sc.Mounts[0].HostPath)
	assert.True(t, sc.Mounts[0].ReadOnly)
	assert.Equal(t, "postgres", sc.Env["DB"])
}

func TestResolve_CLIOverridesWin(t *testing.T) {
	cfg := &Config{
		Sandboxes: map[string]SandboxDef{
			"api": {
				Workdir: "./services/api",
				Env:     map[string]string{"KEY": "from-config"},
			},
		},
	}

	cli := CLIOverrides{
		Name:    "api",
		Workdir: "./override",
		Env:     map[string]string{"KEY": "from-cli"},
	}

	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.Equal(t, "./override", sc.Workdir)
	assert.Equal(t, "from-cli", sc.Env["KEY"])
}

func TestResolve_DotenvMerging(t *testing.T) {
	cfg := &Config{
		Sandboxes: map[string]SandboxDef{
			"api": {
				Env: map[string]string{"OVERRIDE": "from-sandbox"},
			},
		},
	}

	dotenv := map[string]string{
		"DOTENV_ONLY": "yes",
		"OVERRIDE":    "from-dotenv",
	}

	sc, err := cfg.Resolve("/project", CLIOverrides{Name: "api"}, dotenv)
	require.NoError(t, err)
	assert.Equal(t, "yes", sc.Env["DOTENV_ONLY"])
	assert.Equal(t, "from-sandbox", sc.Env["OVERRIDE"]) // per-sandbox wins over dotenv
}

func TestResolve_MountResolutionRelativePaths(t *testing.T) {
	cfg := &Config{}
	cli := CLIOverrides{
		Name:   "test",
		Mounts: []string{"./data:/container/data", "/abs:/container/abs:ro"},
	}

	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	require.Len(t, sc.Mounts, 2)
	assert.Equal(t, "/project/data", sc.Mounts[0].HostPath)
	assert.Equal(t, "/abs", sc.Mounts[1].HostPath)
	assert.True(t, sc.Mounts[1].ReadOnly)
}

func TestResolve_UnknownSandboxName(t *testing.T) {
	cfg := &Config{
		Sandboxes: map[string]SandboxDef{
			"api": {Workdir: "./api"},
		},
	}

	sc, err := cfg.Resolve("/project", CLIOverrides{Name: "custom"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "custom", sc.Name)
	assert.Empty(t, sc.Workdir) // no matching sandbox def, so no workdir
}

func TestResolve_ImageNameFromConfig(t *testing.T) {
	cfg := &Config{ImageName: "my-image:v2"}
	sc, err := cfg.Resolve("/project", CLIOverrides{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "my-image:v2", sc.ImageName)
}

func TestResolve_ReadOnlyCLIOverride(t *testing.T) {
	readOnly := true
	cfg := &Config{
		Defaults: DefaultsConfig{ReadOnly: boolPtr(false)},
	}
	cli := CLIOverrides{
		Name:     "test",
		ReadOnly: &readOnly,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.ReadOnly, "CLI --readonly should override config default")
}

func TestResolve_ReadOnlyCLINil_UsesDefault(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{ReadOnly: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.ReadOnly, "config default readonly should apply when CLI flag is nil")
}

func TestResolve_FirewallCLIOverride(t *testing.T) {
	fw := true
	cfg := &Config{
		Defaults: DefaultsConfig{Firewall: boolPtr(false)},
	}
	cli := CLIOverrides{
		Name:     "test",
		Firewall: &fw,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "CLI --firewall should override config default")
}

func TestResolve_FirewallCLINil_UsesDefault(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{Firewall: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "config default firewall should apply when CLI flag is nil")
}

func TestResolve_FirewallConfigEnabled(t *testing.T) {
	cfg := &Config{
		Firewall: FirewallConfig{Enabled: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "firewall.enabled in config should enable firewall")
}

func TestResolve_FirewallConfigEnabledOverriddenByCLI(t *testing.T) {
	fw := false
	cfg := &Config{
		Firewall: FirewallConfig{Enabled: boolPtr(true)},
	}
	cli := CLIOverrides{
		Name:     "test",
		Firewall: &fw,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.False(t, sc.Firewall, "CLI --firewall=false should override config firewall.enabled")
}

func TestResolve_IsolatedStateCLIOverride(t *testing.T) {
	cfg := &Config{}
	cli := CLIOverrides{
		Name:          "test",
		IsolatedState: true,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.IsolatedState, "CLI --isolated-state should be reflected in SandboxConfig")
}
