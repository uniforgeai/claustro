// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/mcp"
)

// shouldUnpause returns true when the inspected container state warrants unpausing.
func shouldUnpause(state string) bool { return state == "paused" }

// unpauseIfPaused inspects the container and, if paused, unpauses it and any
// MCP SSE siblings. Best-effort for siblings: a failed sibling stays paused
// until the next attach unpauses again. Failure to unpause the parent is fatal
// because the subsequent exec would fail.
func unpauseIfPaused(ctx context.Context, cli *client.Client, id *identity.Identity, parentID string) error {
	inspect, err := cli.ContainerInspect(ctx, parentID)
	if err != nil {
		return fmt.Errorf("inspecting container: %w", err)
	}
	if !shouldUnpause(inspect.State.Status) {
		return nil
	}
	if err := container.Unpause(ctx, cli, parentID); err != nil {
		return err
	}
	siblings, err := mcp.ListSSESiblings(ctx, cli, id)
	if err != nil {
		slog.Warn("listing MCP siblings for resume", "parent", parentID, "err", err)
		return nil
	}
	for _, sib := range siblings {
		if err := container.Unpause(ctx, cli, sib.ID); err != nil {
			slog.Warn("unpausing MCP sibling", "parent", parentID, "sibling", sib.ID, "err", err)
		}
	}
	return nil
}
