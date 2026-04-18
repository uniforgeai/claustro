// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"github.com/spf13/cobra"
)

func newCodexCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Launch Codex CLI inside a sandbox",
		Long:  "Runs 'codex --dangerously-bypass-approvals-and-sandbox' inside the sandbox. Automatically starts a sandbox if none is running. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent(cmd.Context(), name, codexSpec, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}
