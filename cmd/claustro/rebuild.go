package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild the claustro Docker image",
	Long:  "Forces a full rebuild of the claustro:latest image from the embedded Dockerfile.",
	RunE:  runRebuild,
}

var rebuildRestart bool

func init() {
	rebuildCmd.Flags().BoolVar(&rebuildRestart, "restart", false, "Stop project sandboxes before rebuild and restart after")
	rootCmd.AddCommand(rebuildCmd)
}

func runRebuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	if rebuildRestart {
		id, err := identity.FromCWD("")
		if err != nil {
			return fmt.Errorf("resolving identity: %w", err)
		}

		containers, err := container.ListByProject(ctx, cli, id.Project, false)
		if err != nil {
			return err
		}

		// Stop all sandboxes first
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			fmt.Printf("Stopping %s...\n", name)
			if err := container.Stop(ctx, cli, c.ID); err != nil {
				fmt.Printf("  (stop: %v — continuing)\n", err)
			}
		}

		// Rebuild
		if err := image.Build(ctx, cli); err != nil {
			return fmt.Errorf("rebuilding image: %w", err)
		}

		// Restart all sandboxes
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			fmt.Printf("Restarting %s...\n", name)
			if err := container.Start(ctx, cli, c.ID); err != nil {
				fmt.Printf("  error restarting %s: %v\n", name, err)
			}
		}
		return nil
	}

	return image.Build(ctx, cli)
}
