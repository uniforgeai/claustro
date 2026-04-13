// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import "github.com/spf13/cobra"

func setupCommands(root *cobra.Command) {
	root.AddCommand(newInitCmd())
	root.AddCommand(newUpCmd())
	root.AddCommand(newBurnCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newClaudeCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newNukeCmd())
	root.AddCommand(newRebuildCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newLogsCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newUpdateCmd())
}
