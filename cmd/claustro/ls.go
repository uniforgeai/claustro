// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

func newLsCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List sandboxes for the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(cmd.Context(), all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "List sandboxes across all projects")
	return cmd
}

func runLs(ctx context.Context, all bool) error {
	id, err := identity.FromCWD("")
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	containers, err := container.ListByProject(ctx, cli, id.Project, all)
	if err != nil {
		return fmt.Errorf("listing sandboxes: %w", err)
	}

	if len(containers) == 0 {
		if all {
			fmt.Println("No claustro sandboxes found.")
		} else {
			fmt.Printf("No sandboxes for project %q. Run: claustro up\n", id.Project)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if all {
		fmt.Fprintln(w, "PROJECT\tNAME\tCONTAINER\tSTATUS") //nolint:errcheck
	} else {
		fmt.Fprintln(w, "NAME\tCONTAINER\tSTATUS") //nolint:errcheck
	}

	for _, c := range containers {
		name := c.Labels[identity.LabelName]
		project := c.Labels[identity.LabelProject]
		containerName := strings.TrimPrefix(c.Names[0], "/")
		if all {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", project, name, containerName, c.Status) //nolint:errcheck
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, containerName, c.Status) //nolint:errcheck
		}
	}
	return w.Flush()
}
