package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	internalMount "github.com/uniforgeai/claustro/internal/mount"
)

func newUpCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start a sandbox",
		Long:  "Build the claustro image if needed, then create and start a sandbox container.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(cmd.Context(), name)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-generated)`)
	return cmd
}

func runUp(ctx context.Context, name string) error {
	nameWasEmpty := name == ""

	id, err := identity.FromCWD(name)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	existing, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if existing != nil && strings.Contains(existing.Status, "Up") {
		fmt.Printf("Sandbox %q is already running (%s)\n", id.ContainerName(), existing.Status)
		return nil
	}

	// If the name was auto-generated and a container with that name already exists,
	// retry with a new random name (up to 5 attempts).
	if nameWasEmpty && existing != nil {
		const maxRetries = 5
		var found bool
		for i := 0; i < maxRetries; i++ {
			newName := identity.RandomName()
			candidate, cerr := identity.FromCWD(newName)
			if cerr != nil {
				return fmt.Errorf("resolving identity: %w", cerr)
			}
			collision, cerr := container.FindByIdentity(ctx, cli, candidate)
			if cerr != nil {
				return fmt.Errorf("finding sandbox: %w", cerr)
			}
			if collision == nil {
				id = candidate
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("could not generate a unique sandbox name after %d attempts — try: claustro up --name <name>", maxRetries)
		}
	}

	cfg, err := config.Load(id.HostPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var opts container.CreateOptions
	if len(cfg.Image.Extra) > 0 {
		steps := extraRunSteps(cfg.Image.Extra)
		if err := image.EnsureExtended(ctx, cli, id.Project, steps, os.Stdout); err != nil {
			return fmt.Errorf("building extension image: %w", err)
		}
		opts.ImageName = image.ExtImageName(id.Project)
	} else {
		if err := image.EnsureBuilt(ctx, cli, os.Stdout); err != nil {
			return fmt.Errorf("building image: %w", err)
		}
	}

	mounts, err := internalMount.Assemble(id.HostPath, &cfg.Git)
	if err != nil {
		return fmt.Errorf("assembling mounts: %w", err)
	}

	// Ensure npm and pip cache volumes exist, then mount them.
	labels := id.Labels()
	for _, vol := range []struct {
		purpose string
		target  string
	}{
		{"npm", "/home/sandbox/.npm"},
		{"pip", "/home/sandbox/.cache/pip"},
	} {
		volName := id.VolumeName(vol.purpose)
		if err := container.EnsureVolume(ctx, cli, volName, labels); err != nil {
			return fmt.Errorf("ensuring volume %q: %w", volName, err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volName,
			Target: vol.target,
		})
	}

	slog.Info("creating sandbox", "container", id.ContainerName())
	containerID, err := container.Create(ctx, cli, id, mounts, opts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}
	if err := container.Start(ctx, cli, containerID); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	fmt.Printf("Sandbox started: %s\n", id.ContainerName())
	if nameWasEmpty {
		fmt.Printf("  Name: %s  (use --name %s to target it)\n", id.Name, id.Name)
		fmt.Printf("  Run: claustro shell --name %s\n", id.Name)
		fmt.Printf("  Run: claustro claude --name %s\n", id.Name)
	} else {
		fmt.Printf("  Run: claustro shell  —  open a shell\n")
		fmt.Printf("  Run: claustro claude —  start Claude Code\n")
	}
	return nil
}

// extraRunSteps extracts the Run strings from a slice of ExtraStep.
func extraRunSteps(steps []config.ExtraStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Run
	}
	return out
}
