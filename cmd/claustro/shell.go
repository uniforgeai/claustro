// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/container"
)

func newShellCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open an interactive shell in a running sandbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(cmd.Context(), name)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	return cmd
}

func runShell(ctx context.Context, name string) error {
	cli, id, c, err := resolveTargetContainer(ctx, name)
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	if err := unpauseIfPaused(ctx, cli, id, c.ID); err != nil {
		return err
	}

	sockDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	return container.Exec(ctx, cli, c.ID, []string{"/bin/zsh"}, container.ExecOptions{
		Interactive:      true,
		ClipboardSockDir: sockDir,
	})
}
