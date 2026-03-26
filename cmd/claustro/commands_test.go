package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecCmd_Defaults(t *testing.T) {
	nameFlag := execCmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "", nameFlag.DefValue)
}

func TestStatusCmd_Defaults(t *testing.T) {
	nameFlag := statusCmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "", nameFlag.DefValue)
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
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := logsCmd.Flags().Lookup(tt.flag)
			assert.NotNil(t, f)
			assert.Equal(t, tt.defValue, f.DefValue)
		})
	}
}

func TestNukeCmd_Defaults(t *testing.T) {
	f := nukeCmd.Flags().Lookup("all")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

func TestRebuildCmd_Defaults(t *testing.T) {
	f := rebuildCmd.Flags().Lookup("restart")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}
