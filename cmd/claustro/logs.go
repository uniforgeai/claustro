// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
)

func newLogsCmd() *cobra.Command {
	var name string
	var follow bool
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream or tail container logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd.Context(), name, follow, tail)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of lines to show from the end")
	return cmd
}

func runLogs(ctx context.Context, name string, follow bool, tail int) error {
	cli, _, c, err := resolveTargetContainer(ctx, name)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	return container.Logs(ctx, cli, c.ID, os.Stdout, os.Stderr, follow, tail)
}
