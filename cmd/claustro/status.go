// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
)

func newStatusCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show detailed status of a sandbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd.Context(), name)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	return cmd
}

func runStatus(ctx context.Context, name string) error {
	cli, _, c, err := resolveTargetContainer(ctx, name)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	info, err := container.Inspect(ctx, cli, c.ID)
	if err != nil {
		return fmt.Errorf("inspecting container: %w", err)
	}

	return container.FormatStatus(os.Stdout, info)
}
