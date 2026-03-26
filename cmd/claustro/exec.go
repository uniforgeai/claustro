package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

var execCmd = &cobra.Command{
	Use:   "exec -- <command> [args...]",
	Short: "Run a one-off command in a running sandbox",
	Long:  "Runs a command inside the sandbox and streams its output. Pass the command after '--'.",
	RunE:  runExec,
}

var execName string

func init() {
	execCmd.Flags().StringVar(&execName, "name", "", "Sandbox name (default: \"default\")")
	execCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command required after '--'")
	}

	ctx := context.Background()

	id, err := identity.FromCWD(execName)
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
		exitIfNotRunning(execName)
		return nil
	}

	if err := container.Exec(ctx, cli, c.ID, args, false); err != nil {
		// Exit code errors are propagated via os.Exit to preserve the code.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return nil
}
