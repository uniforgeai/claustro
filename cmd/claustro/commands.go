package main

import "github.com/spf13/cobra"

func setupCommands(root *cobra.Command) {
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
}
