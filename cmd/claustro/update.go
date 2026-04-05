// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/updater"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update claustro to the latest version",
		Long:  "Detects the installation method and updates claustro accordingly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			method := updater.DetectMethod()

			switch method {
			case updater.MethodHomebrew:
				fmt.Fprintln(cmd.OutOrStdout(), "Detected install method: Homebrew")
			case updater.MethodGoInstall:
				fmt.Fprintln(cmd.OutOrStdout(), "Detected install method: go install")
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "Detected install method: unknown")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", version)
			fmt.Fprintln(cmd.OutOrStdout(), "Updating...")

			msg, err := updater.Update(method, version)
			if err != nil {
				return err
			}
			if msg != "" {
				fmt.Fprintln(cmd.OutOrStdout(), msg)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Update complete.")
			return nil
		},
	}
}
