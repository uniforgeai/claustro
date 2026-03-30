// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
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
	// Derive project slug from CWD for auto-select.
	tmpID, err := identity.FromCWD("")
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	resolvedName, err := resolveName(ctx, cli, tmpID.Project, name)
	if err != nil {
		return err
	}

	id, err := identity.FromCWD(resolvedName)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	info, err := container.Inspect(ctx, cli, c.ID)
	if err != nil {
		return fmt.Errorf("inspecting container: %w", err)
	}

	return container.FormatStatus(os.Stdout, info)
}
