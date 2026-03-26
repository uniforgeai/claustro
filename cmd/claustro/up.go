package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	"github.com/uniforgeai/claustro/internal/mount"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Create and start a sandbox",
	Long:  "Build the claustro image if needed, then create and start a sandbox container.",
	RunE:  runUp,
}

var upName string

func init() {
	upCmd.Flags().StringVar(&upName, "name", "", "Sandbox name (default: \"default\")")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(upName)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	// Check if already running
	existing, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return err
	}
	if existing != nil && strings.Contains(existing.Status, "Up") {
		fmt.Printf("Sandbox %q is already running (%s)\n", id.ContainerName(), existing.Status)
		return nil
	}

	// Ensure image exists
	if err := image.EnsureBuilt(ctx, cli); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	// Assemble mounts
	mounts, err := mount.Assemble(id.HostPath)
	if err != nil {
		return fmt.Errorf("assembling mounts: %w", err)
	}

	// Create and start
	slog.Info("creating sandbox", "container", id.ContainerName())
	containerID, err := container.Create(ctx, cli, id, mounts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}
	if err := container.Start(ctx, cli, containerID); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	fmt.Printf("Sandbox started: %s\n", id.ContainerName())
	fmt.Printf("  Run: claustro shell  —  open a shell\n")
	fmt.Printf("  Run: claustro claude —  start Claude Code\n")
	return nil
}

func newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("connecting to Docker: %w", err)
	}
	return cli, nil
}

func exitIfNotRunning(name string) {
	fmt.Fprintf(os.Stderr, "No running sandbox %q found. Run: claustro up%s\n",
		name, nameFlag(name))
	os.Exit(1)
}

func nameFlag(name string) string {
	if name != "" && name != "default" {
		return " --name " + name
	}
	return ""
}
