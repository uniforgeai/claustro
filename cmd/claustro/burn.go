package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

func newBurnCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Stop and remove a sandbox container",
		Long:  "Stops and removes the sandbox container. Image and ~/.claude are preserved.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBurn(cmd.Context(), name)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	return cmd
}

func runBurn(ctx context.Context, name string) error {
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
		fmt.Printf("No sandbox %q found — nothing to burn.\n", id.ContainerName())
		return nil
	}

	fmt.Printf("Burning sandbox %s...\n", id.ContainerName())
	if err := container.Stop(ctx, cli, c.ID); err != nil {
		fmt.Printf("(stop: %v — continuing)\n", err)
	}
	if err := container.Remove(ctx, cli, c.ID); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}

	fmt.Printf("Burned: %s\n", id.ContainerName())
	return nil
}
