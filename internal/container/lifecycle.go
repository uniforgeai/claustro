package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
)

// NukeContainers stops and removes all sandbox containers and their networks for the given project.
// If allProjects is true, targets all claustro-managed containers across all projects.
// Progress messages are written to w.
func NukeContainers(ctx context.Context, cli *client.Client, project string, allProjects bool, w io.Writer) error {
	containers, err := ListByProject(ctx, cli, project, allProjects)
	if err != nil {
		return fmt.Errorf("listing sandboxes: %w", err)
	}

	if len(containers) == 0 {
		if allProjects {
			fmt.Fprintln(w, "No claustro sandboxes found.") //nolint:errcheck
		} else {
			fmt.Fprintf(w, "No sandboxes for project %q — nothing to nuke.\n", project) //nolint:errcheck
		}
		return nil
	}

	// Partition: remove MCP siblings first, then sandboxes.
	var siblings, sandboxes []containertypes.Summary
	for _, c := range containers {
		if c.Labels["claustro.role"] == "mcp-sse" {
			siblings = append(siblings, c)
		} else {
			sandboxes = append(sandboxes, c)
		}
	}

	for _, c := range siblings {
		name := strings.TrimPrefix(c.Names[0], "/")
		fmt.Fprintf(w, "Removing MCP sibling %s...\n", name) //nolint:errcheck
		if err := Stop(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  (stop: %v — continuing)\n", err) //nolint:errcheck
		}
		if err := Remove(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  error removing: %v\n", err) //nolint:errcheck
		}
	}

	for _, c := range sandboxes {
		name := strings.TrimPrefix(c.Names[0], "/")
		networkName := identity.NetworkNameFromLabels(c.Labels)
		sandboxName := c.Labels["claustro.name"]
		project := c.Labels["claustro.project"]

		fmt.Fprintf(w, "Nuking %s...\n", name) //nolint:errcheck

		if err := Stop(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  (stop: %v — continuing)\n", err) //nolint:errcheck
		}
		if err := Remove(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  error removing container: %v\n", err) //nolint:errcheck
			continue
		}
		if err := RemoveNetwork(ctx, cli, networkName); err != nil {
			fmt.Fprintf(w, "  error removing network: %v\n", err) //nolint:errcheck
		}
		// Remove cache volumes for this sandbox.
		id := &identity.Identity{Project: project, Name: sandboxName}
		for _, purpose := range []string{"npm", "pip"} {
			volName := id.VolumeName(purpose)
			if err := RemoveVolume(ctx, cli, volName); err != nil {
				fmt.Fprintf(w, "  error removing volume %q: %v\n", volName, err) //nolint:errcheck
			}
		}
		fmt.Fprintf(w, "  nuked: %s\n", name) //nolint:errcheck
	}

	return nil
}

// RebuildRestart stops all project sandboxes, rebuilds the claustro image, then restarts them.
// Progress messages are written to w.
func RebuildRestart(ctx context.Context, cli *client.Client, project string, w io.Writer) error {
	containers, err := ListByProject(ctx, cli, project, false)
	if err != nil {
		return fmt.Errorf("listing sandboxes: %w", err)
	}

	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		fmt.Fprintf(w, "Stopping %s...\n", name) //nolint:errcheck
		if err := Stop(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  (stop: %v — continuing)\n", err) //nolint:errcheck
		}
	}

	if err := image.Build(ctx, cli, w); err != nil {
		return fmt.Errorf("rebuilding image: %w", err)
	}

	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		fmt.Fprintf(w, "Restarting %s...\n", name) //nolint:errcheck
		if err := Start(ctx, cli, c.ID); err != nil {
			fmt.Fprintf(w, "  error restarting %s: %v\n", name, err) //nolint:errcheck
		}
	}
	return nil
}
