package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

func newClaudeCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Launch Claude Code inside a running sandbox",
		Long:  "Runs 'claude --dangerously-skip-permissions' inside the sandbox. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaude(cmd.Context(), name, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: "default")`)
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runClaude(ctx context.Context, name string, extraArgs []string) error {
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

	execCmd := append([]string{"claude", "--dangerously-skip-permissions"}, extraArgs...)
	return container.Exec(ctx, cli, c.ID, execCmd, true)
}
