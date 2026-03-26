package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream or tail container logs",
	RunE:  runLogs,
}

var logsName string
var logsFollow bool
var logsTail int

func init() {
	logsCmd.Flags().StringVar(&logsName, "name", "", "Sandbox name (default: \"default\")")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVar(&logsTail, "tail", 100, "Number of lines to show from the end")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(logsName)
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
		fmt.Fprintf(os.Stderr, "No sandbox %q found. Run: claustro up%s\n", id.ContainerName(), nameFlag(logsName))
		os.Exit(1)
	}

	tail := "all"
	if logsTail > 0 {
		tail = strconv.Itoa(logsTail)
	}

	rc, err := cli.ContainerLogs(ctx, c.ID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     logsFollow,
		Tail:       tail,
	})
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, rc); err != nil {
		return fmt.Errorf("streaming logs: %w", err)
	}
	return nil
}
