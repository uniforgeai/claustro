package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
)

func newRebuildCmd() *cobra.Command {
	var restart bool
	var noExt bool
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the claustro Docker image",
		Long:  "Forces a full rebuild of the claustro:latest image from the embedded Dockerfile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRebuild(cmd.Context(), restart, noExt)
		},
	}
	cmd.Flags().BoolVar(&restart, "restart", false, "Stop project sandboxes before rebuild and restart after")
	cmd.Flags().BoolVar(&noExt, "no-ext", false, "Skip rebuilding the project extension image after the base rebuild")
	return cmd
}

func runRebuild(ctx context.Context, restart bool, noExt bool) error {
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	id, err := identity.FromCWD("")
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cfg, err := config.Load(id.HostPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if restart {
		return container.RebuildRestart(ctx, cli, id.Project, os.Stdout)
	}

	if err := image.Build(ctx, cli, os.Stdout); err != nil {
		return fmt.Errorf("rebuilding image: %w", err)
	}

	if !noExt && len(cfg.ImageConfig.Extra) > 0 {
		steps := extraRunSteps(cfg.ImageConfig.Extra)
		if err := image.BuildExtended(ctx, cli, id.Project, steps, os.Stdout); err != nil {
			return fmt.Errorf("rebuilding extension image: %w", err)
		}
	}
	return nil
}
