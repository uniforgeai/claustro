package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
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
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: "default")`)
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of lines to show from the end")
	return cmd
}

func runLogs(ctx context.Context, name string, follow bool, tail int) error {
	id, err := identity.FromCWD(name)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	return container.Logs(ctx, cli, c.ID, os.Stdout, os.Stderr, follow, tail)
}
