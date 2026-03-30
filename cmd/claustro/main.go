// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claustro",
		Short: "Disposable Docker sandboxes for Claude Code",
		Long: `claustro manages disposable Docker containers for running Claude Code
safely against local source code. Source stays on the host (bind-mounted),
containers are cheap to burn and respawn.`,
	}
	setupCommands(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
