package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open an interactive shell in a running sandbox",
	RunE:  runShell,
}

var shellName string

func init() {
	shellCmd.Flags().StringVar(&shellName, "name", "", "Sandbox name (default: \"default\")")
	rootCmd.AddCommand(shellCmd)
}

func runShell(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(shellName)
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
		exitIfNotRunning(shellName)
		return nil
	}

	return container.Exec(ctx, cli, c.ID, []string{"/bin/zsh"}, true)
}
