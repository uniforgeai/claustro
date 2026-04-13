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
			w := cmd.OutOrStdout()

			switch method {
			case updater.MethodHomebrew:
				_, _ = fmt.Fprintln(w, "Detected install method: Homebrew")
			case updater.MethodGoInstall:
				_, _ = fmt.Fprintln(w, "Detected install method: go install")
			default:
				_, _ = fmt.Fprintln(w, "Detected install method: unknown")
			}

			_, _ = fmt.Fprintf(w, "Current version: %s\n", version)
			_, _ = fmt.Fprintln(w, "Updating...")

			msg, err := updater.Update(method, version)
			if err != nil {
				return err
			}
			if msg != "" {
				_, _ = fmt.Fprintln(w, msg)
			}
			_, _ = fmt.Fprintln(w, "Update complete.")
			return nil
		},
	}
}
