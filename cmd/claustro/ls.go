package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List sandboxes for the current project",
	RunE:  runLs,
}

var lsAll bool

func init() {
	lsCmd.Flags().BoolVar(&lsAll, "all", false, "List sandboxes across all projects")
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
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

	containers, err := container.ListByProject(ctx, cli, id.Project, lsAll)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		if lsAll {
			fmt.Println("No claustro sandboxes found.")
		} else {
			fmt.Printf("No sandboxes for project %q. Run: claustro up\n", id.Project)
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if lsAll {
		fmt.Fprintln(w, "PROJECT\tNAME\tCONTAINER\tSTATUS")
	} else {
		fmt.Fprintln(w, "NAME\tCONTAINER\tSTATUS")
	}

	for _, c := range containers {
		name := c.Labels["claustro.name"]
		project := c.Labels["claustro.project"]
		containerName := strings.TrimPrefix(c.Names[0], "/")
		if lsAll {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", project, name, containerName, c.Status)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", name, containerName, c.Status)
		}
	}
	return w.Flush()
}
