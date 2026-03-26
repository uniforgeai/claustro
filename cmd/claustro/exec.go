package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

func newExecCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "exec -- <command> [args...]",
		Short: "Run a one-off command in a running sandbox",
		Long:  "Runs a command inside the sandbox and streams its output. Pass the command after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExec(cmd.Context(), name, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: "default")`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runExec(ctx context.Context, name string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command required after '--'")
	}

	id, err := identity.FromCWD(name)
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
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	return container.Exec(ctx, cli, c.ID, args, false)
}
