package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var claudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Launch Claude Code inside a running sandbox",
	Long:  "Runs 'claude --dangerously-skip-permissions' inside the sandbox. Pass extra args after '--'.",
	RunE:  runClaude,
}

var claudeName string

func init() {
	claudeCmd.Flags().StringVar(&claudeName, "name", "", "Sandbox name (default: \"default\")")
	claudeCmd.Flags().SetInterspersed(false) // allow -- to pass args through
	rootCmd.AddCommand(claudeCmd)
}

func runClaude(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	id, err := identity.FromCWD(claudeName)
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
		exitIfNotRunning(claudeName)
		return nil
	}

	execCmd := append([]string{"claude", "--dangerously-skip-permissions"}, args...)
	return container.Exec(ctx, cli, c.ID, execCmd, true)
}
