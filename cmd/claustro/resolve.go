// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/container"
)

// resolveName returns the sandbox name to use for targeting commands.
// If name is non-empty, it is returned as-is.
// If name is empty and exactly one sandbox is running for the project, that name is returned.
// If name is empty and multiple sandboxes exist, an error listing available names is returned.
// If name is empty and no sandboxes exist, an error prompting to run `claustro up` is returned.
func resolveName(ctx context.Context, cli *client.Client, project, name string) (string, error) {
	if name != "" {
		return name, nil
	}
	containers, err := container.ListByProject(ctx, cli, project, false)
	if err != nil {
		return "", fmt.Errorf("listing sandboxes: %w", err)
	}
	switch len(containers) {
	case 0:
		return "", fmt.Errorf("no sandboxes running for project %q — run: claustro up", project)
	case 1:
		return containers[0].Labels["claustro.name"], nil
	default:
		names := make([]string, len(containers))
		for i, c := range containers {
			names[i] = "  " + c.Labels["claustro.name"]
		}
		return "", fmt.Errorf("multiple sandboxes running, specify --name:\n%s", strings.Join(names, "\n"))
	}
}
