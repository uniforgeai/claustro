// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd_DefaultOutput(t *testing.T) {
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestVersionCmd_Format(t *testing.T) {
	// Save and restore globals
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	version = "v1.2.3"
	commit = "abc1234"
	date = "2026-03-30"

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.Run = func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "claustro %s (commit: %s, built: %s)\n", version, commit, date)
	}
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "claustro v1.2.3 (commit: abc1234, built: 2026-03-30)\n", buf.String())
}
