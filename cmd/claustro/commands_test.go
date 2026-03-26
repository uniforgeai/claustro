package main

import (
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
	expected := []string{"burn", "claude", "exec", "logs", "ls", "nuke", "rebuild", "shell", "status", "up"}
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

func TestNukeCmd_Defaults(t *testing.T) {
	cmd := newNukeCmd()
	f := cmd.Flags().Lookup("all")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

func TestRebuildCmd_Defaults(t *testing.T) {
	cmd := newRebuildCmd()
	f := cmd.Flags().Lookup("restart")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}
