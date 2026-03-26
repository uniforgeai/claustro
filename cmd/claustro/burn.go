package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var burnCmd = &cobra.Command{
	Use:   "burn",
	Short: "Stop and remove a sandbox container",
	Long:  "Stops and removes the sandbox container. Image and ~/.claude are preserved.",
	RunE:  runBurn,
}

var burnName string

func init() {
	burnCmd.Flags().StringVar(&burnName, "name", "", "Sandbox name (default: \"default\")")
	rootCmd.AddCommand(burnCmd)
}

func runBurn(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(burnName)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return err
	}
	if c == nil {
		fmt.Printf("No sandbox %q found — nothing to burn.\n", id.ContainerName())
		return nil
	}

	fmt.Printf("Burning sandbox %s...\n", id.ContainerName())
	if err := container.Stop(ctx, cli, c.ID); err != nil {
		// Ignore already-stopped errors
		fmt.Printf("(stop: %v — continuing)\n", err)
	}
	if err := container.Remove(ctx, cli, c.ID); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}

	fmt.Printf("Burned: %s\n", id.ContainerName())
	return nil
}
