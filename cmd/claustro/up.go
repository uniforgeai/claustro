// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/firewall"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	"github.com/uniforgeai/claustro/internal/mcp"
	internalMount "github.com/uniforgeai/claustro/internal/mount"
)

func newUpCmd() *cobra.Command {
	var (
		name          string
		workdir       string
		mounts        []string
		envs          []string
		readOnly      bool
		firewall      bool
		isolatedState bool
	)
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create and start a sandbox",
		Long:  "Build the claustro image if needed, then create and start a sandbox container.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliEnv := parseEnvFlags(envs)
			var readOnlyPtr *bool
			if cmd.Flags().Changed("readonly") {
				readOnlyPtr = &readOnly
			}
			var firewallPtr *bool
			if cmd.Flags().Changed("firewall") {
				firewallPtr = &firewall
			}
			return runUp(cmd.Context(), name, config.CLIOverrides{
				Name:          name,
				Workdir:       workdir,
				Mounts:        mounts,
				Env:           cliEnv,
				ReadOnly:      readOnlyPtr,
				Firewall:      firewallPtr,
				IsolatedState: isolatedState,
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-generated)`)
	cmd.Flags().StringVar(&workdir, "workdir", "", `Working directory inside the container`)
	cmd.Flags().StringSliceVar(&mounts, "mount", nil, `Additional bind mount (host:container[:ro|rw])`)
	cmd.Flags().StringSliceVar(&envs, "env", nil, `Environment variable (KEY=VALUE)`)
	cmd.Flags().BoolVar(&readOnly, "readonly", false, `Mount source directory as read-only`)
	cmd.Flags().BoolVar(&firewall, "firewall", false, `Enable egress firewall (restrict outbound traffic to allowlist)`)
	cmd.Flags().BoolVar(&isolatedState, "isolated-state", false, `Use a Docker volume for Claude state instead of bind-mounting ~/.claude`)
	return cmd
}

// parseEnvFlags converts ["KEY=VALUE", ...] into a map.
func parseEnvFlags(envs []string) map[string]string {
	if len(envs) == 0 {
		return nil
	}
	m := make(map[string]string, len(envs))
	for _, e := range envs {
		if k, v, ok := strings.Cut(e, "="); ok {
			m[k] = v
		}
	}
	return m
}

func runUp(ctx context.Context, name string, cliOverrides config.CLIOverrides) error {
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

	id, alreadyRunning, err := ensureRunning(ctx, cli, id, nameWasEmpty, false, cliOverrides)
	if err != nil {
		return err
	}
	if alreadyRunning {
		return nil
	}

	fmt.Printf("Sandbox started: %s\n", id.ContainerName())
	if nameWasEmpty {
		fmt.Printf("  Name: %s  (use --name %s to target it)\n", id.Name, id.Name)
		fmt.Printf("  Run: claustro shell --name %s\n", id.Name)
		fmt.Printf("  Run: claustro claude --name %s\n", id.Name)
	} else {
		fmt.Printf("  Run: claustro shell  —  open a shell\n")
		fmt.Printf("  Run: claustro claude —  start Claude Code\n")
	}
	return nil
}

// ensureRunning ensures a sandbox container is running for the given identity.
// If the sandbox is already running, it returns alreadyRunning=true and takes no action.
// When quiet is true, output is minimal (suitable for auto-up from the claude command).
// The returned identity may differ from the input if the name was auto-generated and
// required retry due to a collision.
func ensureRunning(ctx context.Context, cli *client.Client, id *identity.Identity, nameWasEmpty, quiet bool, cliOverrides config.CLIOverrides) (_ *identity.Identity, alreadyRunning bool, _ error) {
	existing, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return nil, false, fmt.Errorf("finding sandbox: %w", err)
	}
	if existing != nil && strings.Contains(existing.Status, "Up") {
		if !quiet {
			fmt.Printf("Sandbox %q is already running (%s)\n", id.ContainerName(), existing.Status)
		}
		return id, true, nil
	}

	// If the name was auto-generated and a container with that name already exists,
	// retry with a new random name (up to 5 attempts).
	if nameWasEmpty && existing != nil {
		const maxRetries = 5
		var found bool
		for i := 0; i < maxRetries; i++ {
			newName := identity.RandomName()
			candidate, cerr := identity.FromCWD(newName)
			if cerr != nil {
				return nil, false, fmt.Errorf("resolving identity: %w", cerr)
			}
			collision, cerr := container.FindByIdentity(ctx, cli, candidate)
			if cerr != nil {
				return nil, false, fmt.Errorf("finding sandbox: %w", cerr)
			}
			if collision == nil {
				id = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, false, fmt.Errorf("could not generate a unique sandbox name after %d attempts — try: claustro up --name <name>", maxRetries)
		}
	}

	if quiet {
		fmt.Fprintf(os.Stderr, "Starting sandbox %s...\n", id.ContainerName())
	}

	cfg, err := config.Load(id.HostPath)
	if err != nil {
		return nil, false, fmt.Errorf("loading config: %w", err)
	}

	dotenv, err := config.LoadDotenv(id.HostPath)
	if err != nil {
		return nil, false, fmt.Errorf("loading .env: %w", err)
	}

	resolved, err := cfg.Resolve(id.HostPath, cliOverrides, dotenv)
	if err != nil {
		return nil, false, fmt.Errorf("resolving config: %w", err)
	}
	slog.Debug("resolved sandbox config",
		"name", resolved.Name,
		"workdir", resolved.Workdir,
		"mounts", len(resolved.Mounts),
		"env_vars", len(resolved.Env),
		"image", resolved.ImageName,
	)

	var opts container.CreateOptions
	opts.Firewall = resolved.Firewall
	if len(cfg.ImageConfig.Extra) > 0 {
		steps := extraRunSteps(cfg.ImageConfig.Extra)
		if err := image.EnsureExtended(ctx, cli, id.Project, steps, os.Stdout); err != nil {
			return nil, false, fmt.Errorf("building extension image: %w", err)
		}
		opts.ImageName = image.ExtImageName(id.Project)
	} else {
		if err := image.EnsureBuilt(ctx, cli, &cfg.ImageBuild, os.Stdout); err != nil {
			return nil, false, fmt.Errorf("building image: %w", err)
		}
	}

	socketDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	mounts, err := internalMount.Assemble(id.HostPath, &cfg.Git, socketDir, resolved.ReadOnly, resolved.IsolatedState)
	if err != nil {
		return nil, false, fmt.Errorf("assembling mounts: %w", err)
	}

	// When isolated state is requested, create a project-scoped volume for Claude state.
	if resolved.IsolatedState {
		volName := identity.ProjectVolumeName(id.Project, "claude-state")
		if err := container.EnsureVolume(ctx, cli, volName, id.Labels()); err != nil {
			return nil, false, fmt.Errorf("ensuring claude state volume %q: %w", volName, err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volName,
			Target: "/home/sandbox/.claude",
		})
	}

	// Ensure npm and pip cache volumes exist, then mount them.
	labels := id.Labels()
	for _, vol := range []struct {
		purpose string
		target  string
	}{
		{"npm", "/home/sandbox/.npm"},
		{"pip", "/home/sandbox/.cache/pip"},
	} {
		volName := id.VolumeName(vol.purpose)
		if err := container.EnsureVolume(ctx, cli, volName, labels); err != nil {
			return nil, false, fmt.Errorf("ensuring volume %q: %w", volName, err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volName,
			Target: vol.target,
		})
	}

	slog.Info("creating sandbox", "container", id.ContainerName())
	containerID, err := container.Create(ctx, cli, id, mounts, opts)
	if err != nil {
		return nil, false, fmt.Errorf("creating container: %w", err)
	}
	if err := container.Start(ctx, cli, containerID); err != nil {
		return nil, false, fmt.Errorf("starting container: %w", err)
	}

	// Start MCP SSE sibling containers (non-fatal on failure).
	if len(cfg.MCP.SSE) > 0 {
		mcp.StartSSESiblings(ctx, cli, id, cfg.MCP.SSE)
	}

	// Write MCP config into the container.
	if err := writeMCPConfig(ctx, cli, containerID, cfg, resolved.IsolatedState); err != nil {
		slog.Warn("failed to write MCP config", "err", err)
	}

	// Apply egress firewall rules if enabled.
	if resolved.Firewall {
		slog.Info("applying egress firewall", "container", id.ContainerName())
		if err := firewall.Apply(ctx, cli, containerID, cfg.Firewall.Allow); err != nil {
			// Firewall failure is fatal — stop and remove the container.
			_ = container.Stop(ctx, cli, containerID)
			_ = container.Remove(ctx, cli, containerID)
			return nil, false, fmt.Errorf("applying firewall: %w", err)
		}
	}

	return id, false, nil
}

// writeMCPConfig builds the merged MCP config and writes it into the container.
func writeMCPConfig(ctx context.Context, cli *client.Client, containerID string, cfg *config.Config, isolatedState bool) error {
	mcpCfg := mcp.DefaultConfig()

	// When bind-mounted, read existing host mcp.json and merge it.
	if !isolatedState {
		home, err := os.UserHomeDir()
		if err == nil {
			hostMCPPath := filepath.Join(home, ".claude", "mcp.json")
			if data, err := os.ReadFile(hostMCPPath); err == nil {
				hostCfg, err := mcp.ParseJSON(data)
				if err != nil {
					slog.Warn("could not parse host mcp.json, using defaults only", "err", err)
				} else {
					mcpCfg = mcp.Merge(mcpCfg, hostCfg)
				}
			}
		}
	}

	// Merge project-level MCP stdio overrides.
	if len(cfg.MCP.Stdio) > 0 {
		projectCfg := mcp.FromProjectConfig(cfg.MCP.Stdio)
		mcpCfg = mcp.Merge(mcpCfg, projectCfg)
	}

	// Merge SSE MCP endpoint entries.
	if len(cfg.MCP.SSE) > 0 {
		sseCfg := mcp.SSEEntries(cfg.MCP.SSE)
		mcpCfg = mcp.Merge(mcpCfg, sseCfg)
	}

	cmd, err := mcp.WriteCommand(mcpCfg, mcp.MCPConfigPath)
	if err != nil {
		return fmt.Errorf("building mcp write command: %w", err)
	}

	return container.ExecSimple(ctx, cli, containerID, cmd)
}

// extraRunSteps extracts the Run strings from a slice of ExtraStep.
func extraRunSteps(steps []config.ExtraStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Run
	}
	return out
}
