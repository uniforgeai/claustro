// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/updater"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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

	// Start background update check.
	reminderCh := make(chan string, 1)
	go func() {
		reminderCh <- updater.CheckAndRemind(version)
	}()

	err := rootCmd.Execute()

	// Print update reminder if available (non-blocking select).
	select {
	case msg := <-reminderCh:
		if msg != "" {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, msg)
		}
	default:
		// Check not done yet, don't block.
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
