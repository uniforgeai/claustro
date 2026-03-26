package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detailed status of a sandbox",
	RunE:  runStatus,
}

var statusName string

func init() {
	statusCmd.Flags().StringVar(&statusName, "name", "", "Sandbox name (default: \"default\")")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(statusName)
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
		return err
	}
	if c == nil {
		fmt.Fprintf(os.Stderr, "No sandbox %q found. Run: claustro up%s\n", id.ContainerName(), nameFlag(statusName)) //nolint:errcheck
		os.Exit(1)
	}

	info, err := container.Inspect(ctx, cli, c.ID)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Container:\t%s\n", strings.TrimPrefix(info.Name, "/"))  //nolint:errcheck
	fmt.Fprintf(w, "State:\t%s\n", info.State.Status)                       //nolint:errcheck
	fmt.Fprintf(w, "Image:\t%s\n", info.Config.Image)                       //nolint:errcheck

	if info.State.Running {
		start, err := time.Parse(time.RFC3339Nano, info.State.StartedAt)
		if err == nil {
			fmt.Fprintf(w, "Uptime:\t%s\n", time.Since(start).Truncate(time.Second)) //nolint:errcheck
		}
	} else if info.State.FinishedAt != "" && info.State.FinishedAt != "0001-01-01T00:00:00Z" {
		fmt.Fprintf(w, "Exited at:\t%s\n", info.State.FinishedAt) //nolint:errcheck
	}

	for _, env := range info.Config.Env {
		if strings.HasPrefix(env, "CLAUSTRO_HOST_PATH=") {
			fmt.Fprintf(w, "Host path:\t%s\n", strings.TrimPrefix(env, "CLAUSTRO_HOST_PATH=")) //nolint:errcheck
			break
		}
	}

	fmt.Fprintln(w, "\nMounts:") //nolint:errcheck
	for _, m := range info.Mounts {
		fmt.Fprintf(w, "  %s\t→\t%s\n", m.Source, m.Destination) //nolint:errcheck
	}

	fmt.Fprintln(w, "\nNetworks:") //nolint:errcheck
	for name := range info.NetworkSettings.Networks {
		fmt.Fprintf(w, "  %s\n", name) //nolint:errcheck
	}

	return w.Flush()
}
