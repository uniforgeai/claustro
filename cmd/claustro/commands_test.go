package main

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/identity"
)

func makeRoot() *cobra.Command {
	root := &cobra.Command{Use: "claustro"}
	setupCommands(root)
	return root
}

func findSubcmd(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestSetupCommands_RegistersAllCommands(t *testing.T) {
	root := makeRoot()
	expected := []string{"burn", "claude", "config", "doctor", "exec", "init", "logs", "ls", "nuke", "rebuild", "shell", "status", "up", "validate"}
	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			cmd := findSubcmd(root, name)
			assert.NotNil(t, cmd, "expected command %q to be registered", name)
		})
	}
}

func TestErrNotRunning_AlwaysShowsName(t *testing.T) {
	// errNotRunning always includes --name so the user can rerun `claustro up` with the same name.
	id := &identity.Identity{Project: "myproject", Name: "calm_river", HostPath: "/tmp"}
	err := errNotRunning(id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "myproject")
	assert.Contains(t, err.Error(), "--name calm_river")
}

func TestErrNotRunning_CustomName(t *testing.T) {
	id := &identity.Identity{Project: "myproject", Name: "staging", HostPath: "/tmp"}
	err := errNotRunning(id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--name staging")
}

func TestClaudeCmd_DescribesAutoUp(t *testing.T) {
	root := makeRoot()
	cmd := findSubcmd(root, "claude")
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Long, "Automatically starts a sandbox if none is running")
}

func TestClaudeCmd_Defaults(t *testing.T) {
	cmd := newClaudeCmd()
	f := cmd.Flags().Lookup("name")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}

func TestExecCmd_Defaults(t *testing.T) {
	cmd := newExecCmd()
	f := cmd.Flags().Lookup("name")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}

func TestStatusCmd_Defaults(t *testing.T) {
	cmd := newStatusCmd()
	f := cmd.Flags().Lookup("name")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}

func TestLogsCmd_Defaults(t *testing.T) {
	tests := []struct {
		flag     string
		defValue string
	}{
		{"name", ""},
		{"follow", "false"},
		{"tail", "100"},
	}
	cmd := newLogsCmd()
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			assert.NotNil(t, f)
			assert.Equal(t, tt.defValue, f.DefValue)
		})
	}
}

func TestBurnCmd_Defaults(t *testing.T) {
	cmd := newBurnCmd()
	fName := cmd.Flags().Lookup("name")
	assert.NotNil(t, fName)
	assert.Equal(t, "", fName.DefValue)

	fAll := cmd.Flags().Lookup("all")
	assert.NotNil(t, fAll)
	assert.Equal(t, "false", fAll.DefValue)
}

func TestRunBurn_MutualExclusion(t *testing.T) {
	err := runBurn(context.Background(), "sandbox1", true)
	require.Error(t, err)
	assert.Equal(t, "--name and --all are mutually exclusive", err.Error())
}

func TestNukeCmd_Defaults(t *testing.T) {
	cmd := newNukeCmd()
	f := cmd.Flags().Lookup("all")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

func TestUpCmd_NewFlags(t *testing.T) {
	cmd := newUpCmd()
	tests := []struct {
		flag     string
		defValue string
	}{
		{"name", ""},
		{"workdir", ""},
		{"mount", "[]"},
		{"env", "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			require.NotNil(t, f, "flag %q should exist", tt.flag)
			assert.Equal(t, tt.defValue, f.DefValue)
		})
	}
}

func TestParseEnvFlags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want map[string]string
	}{
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"single", []string{"FOO=bar"}, map[string]string{"FOO": "bar"}},
		{"multiple", []string{"A=1", "B=2"}, map[string]string{"A": "1", "B": "2"}},
		{"value with equals", []string{"URL=http://host:8080/path?q=1"}, map[string]string{"URL": "http://host:8080/path?q=1"}},
		{"no equals skipped", []string{"NOVALUE"}, map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEnvFlags(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRebuildCmd_Defaults(t *testing.T) {
	cmd := newRebuildCmd()
	f := cmd.Flags().Lookup("restart")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

func TestValidateCmd_Exists(t *testing.T) {
	cmd := newValidateCmd()
	assert.Equal(t, "validate", cmd.Name())
}

func TestInitCmd_Defaults(t *testing.T) {
	cmd := newInitCmd()
	assert.Equal(t, "init", cmd.Name())

	flags := []string{"project", "languages", "tools", "mcp", "cpus", "memory", "firewall", "readonly", "yes"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			f := cmd.Flags().Lookup(name)
			assert.NotNil(t, f, "flag %q should exist", name)
		})
	}
}

func TestConfigCmd_HasSubcommands(t *testing.T) {
	cmd := newConfigCmd()
	assert.Equal(t, "config", cmd.Name())

	subs := []string{"get", "set", "languages", "tools", "mcp", "defaults", "firewall", "git"}
	for _, name := range subs {
		t.Run(name, func(t *testing.T) {
			found := false
			for _, sub := range cmd.Commands() {
				if sub.Name() == name {
					found = true
					break
				}
			}
			assert.True(t, found, "subcommand %q should exist", name)
		})
	}
}
