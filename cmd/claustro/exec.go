// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
)

func newExecCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "exec -- <command> [args...]",
		Short: "Run a one-off command in a running sandbox",
		Long:  "Runs a command inside the sandbox and streams its output. Pass the command after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExec(cmd.Context(), name, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runExec(ctx context.Context, name string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command required after '--'")
	}

	cli, _, c, err := resolveTargetContainer(ctx, name)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	return container.Exec(ctx, cli, c.ID, args, container.ExecOptions{})
}
