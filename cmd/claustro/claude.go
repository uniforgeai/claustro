// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/picker"
	"github.com/uniforgeai/claustro/internal/session"
)

func newClaudeCmd() *cobra.Command {
	var name string
	var resume bool
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Launch Claude Code inside a sandbox",
		Long:  "Runs 'claude --dangerously-skip-permissions' inside the sandbox. Automatically starts a sandbox if none is running. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaude(cmd.Context(), name, resume, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a previous Claude Code session")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runClaude(ctx context.Context, name string, resume bool, extraArgs []string) error {
	nameWasEmpty := name == ""

	id, err := identity.FromCWD(name)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	// If a name was given, look for that specific sandbox.
	// If no name was given, try to auto-select from running sandboxes.
	if nameWasEmpty {
		containers, err := container.ListByProject(ctx, cli, id.Project, false)
		if err != nil {
			return fmt.Errorf("listing sandboxes: %w", err)
		}
		switch len(containers) {
		case 0:
			// No sandbox running — auto-up.
		case 1:
			resolvedName := containers[0].Labels["claustro.name"]
			id, err = identity.FromCWD(resolvedName)
			if err != nil {
				return fmt.Errorf("resolving identity: %w", err)
			}
		default:
			names := make([]string, len(containers))
			for i, c := range containers {
				names[i] = "  " + c.Labels["claustro.name"]
			}
			return fmt.Errorf("multiple sandboxes running, specify --name:\n%s", strings.Join(names, "\n"))
		}
	}

	// Ensure the sandbox is running, creating it if needed.
	id, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name})
	if err != nil {
		return err
	}

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	execCmd := []string{"claude", "--dangerously-skip-permissions"}

	if resume {
		sessionID, err := pickSession()
		if err != nil {
			return err
		}
		if sessionID == "" {
			return nil
		}
		execCmd = append(execCmd, "--resume", sessionID)
	}

	execCmd = append(execCmd, extraArgs...)
	sockDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	return container.Exec(ctx, cli, c.ID, execCmd, container.ExecOptions{
		Interactive:      true,
		ClipboardSockDir: sockDir,
	})
}

// pickSession discovers sessions and presents the TUI picker.
// Returns the selected session ID, or empty string if cancelled.
func pickSession() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	sessions, err := session.List(claudeDir)
	if err != nil {
		return "", fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found for this project.")
		return "", nil
	}

	selected, err := picker.PickSession(sessions)
	if err != nil {
		return "", fmt.Errorf("session picker: %w", err)
	}
	if selected == nil {
		return "", nil
	}

	return selected.ID, nil
}
