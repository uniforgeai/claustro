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

func newNukeCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "nuke",
		Short: "Stop and remove all sandboxes for the current project",
		Long:  "Stops and removes all sandbox containers and their networks. Image and ~/.claude are preserved.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNuke(cmd.Context(), all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Remove sandboxes across all projects")
	return cmd
}

func runNuke(ctx context.Context, all bool) error {
	id, err := identity.FromCWD("")
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	return container.NukeContainers(ctx, cli, id.Project, all, os.Stdout)
}
