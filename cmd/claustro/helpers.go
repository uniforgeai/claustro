// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
)

func newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("connecting to Docker: %w", err)
	}
	return cli, nil
}

func errNotRunning(id *identity.Identity) error {
	return fmt.Errorf("no running sandbox %q found — run: claustro up --name %s", id.ContainerName(), id.Name)
}

// resolveTargetContainer resolves the name flag, finds the matching running
// container, and returns the Docker client, identity, and container summary.
// The caller is responsible for deferring cli.Close().
func resolveTargetContainer(ctx context.Context, nameFlag string) (*client.Client, *identity.Identity, *containertypes.Summary, error) {
	tmpID, err := identity.FromCWD("")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return nil, nil, nil, err
	}

	resolvedName, err := resolveName(ctx, cli, tmpID.Project, nameFlag)
	if err != nil {
		cli.Close() //nolint:errcheck
		return nil, nil, nil, err
	}

	id, err := identity.FromCWD(resolvedName)
	if err != nil {
		cli.Close() //nolint:errcheck
		return nil, nil, nil, fmt.Errorf("resolving identity: %w", err)
	}

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		cli.Close() //nolint:errcheck
		return nil, nil, nil, fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		cli.Close() //nolint:errcheck
		return nil, nil, nil, errNotRunning(id)
	}

	return cli, id, c, nil
}

// removeContainerSet stops and removes a set of containers, printing progress.
// The label parameter is used in log messages (e.g. "sandbox", "MCP sibling").
func removeContainerSet(ctx context.Context, cli *client.Client, containers []containertypes.Summary, label string, quiet bool) error {
	for _, c := range containers {
		cName := strings.TrimPrefix(c.Names[0], "/")
		if !quiet {
			fmt.Printf("Removing %s %s...\n", label, cName)
		}
		if err := container.Stop(ctx, cli, c.ID); err != nil {
			fmt.Printf("(stop: %v — continuing)\n", err)
		}
		if err := container.Remove(ctx, cli, c.ID); err != nil {
			fmt.Printf("error removing %s: %v\n", cName, err)
			continue
		}
		if !quiet {
			fmt.Printf("Burned: %s\n", cName)
		}
	}
	return nil
}
