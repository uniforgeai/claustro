package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
)

func newRebuildCmd() *cobra.Command {
	var restart bool
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the claustro Docker image",
		Long:  "Forces a full rebuild of the claustro:latest image from the embedded Dockerfile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRebuild(cmd.Context(), restart)
		},
	}
	cmd.Flags().BoolVar(&restart, "restart", false, "Stop project sandboxes before rebuild and restart after")
	return cmd
}

func runRebuild(ctx context.Context, restart bool) error {
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	if restart {
		id, err := identity.FromCWD("")
		if err != nil {
			return fmt.Errorf("resolving identity: %w", err)
		}
		return container.RebuildRestart(ctx, cli, id.Project, os.Stdout)
	}

	return image.Build(ctx, cli, os.Stdout)
}
