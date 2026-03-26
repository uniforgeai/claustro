package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop and remove all sandboxes for the current project",
	Long:  "Stops and removes all sandbox containers and their networks. Image and ~/.claude are preserved.",
	RunE:  runNuke,
}

var nukeAll bool

func init() {
	nukeCmd.Flags().BoolVar(&nukeAll, "all", false, "Remove sandboxes across all projects")
	rootCmd.AddCommand(nukeCmd)
}

func runNuke(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD("")
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	containers, err := container.ListByProject(ctx, cli, id.Project, nukeAll)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		if nukeAll {
			fmt.Println("No claustro sandboxes found.")
		} else {
			fmt.Printf("No sandboxes for project %q — nothing to nuke.\n", id.Project)
		}
		return nil
	}

	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		networkName := fmt.Sprintf("claustro-%s-%s-net", c.Labels["claustro.project"], c.Labels["claustro.name"])

		fmt.Printf("Nuking %s...\n", name)

		if err := container.Stop(ctx, cli, c.ID); err != nil {
			fmt.Printf("  (stop: %v — continuing)\n", err)
		}
		if err := container.Remove(ctx, cli, c.ID); err != nil {
			fmt.Printf("  error removing container: %v\n", err)
			continue
		}
		if err := container.RemoveNetwork(ctx, cli, networkName); err != nil {
			fmt.Printf("  error removing network: %v\n", err)
		}
		fmt.Printf("  nuked: %s\n", name)
	}

	return nil
}
