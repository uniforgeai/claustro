// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"context"
	"fmt"
	"io"
	"strconv"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Logs streams or tails log output from a container.
// If follow is true, the stream stays open until the container stops.
// tail specifies the number of lines to show from the end; 0 means all lines.
func Logs(ctx context.Context, cli *client.Client, containerID string, stdout, stderr io.Writer, follow bool, tail int) error {
	tailStr := "all"
	if tail > 0 {
		tailStr = strconv.Itoa(tail)
	}

	rc, err := cli.ContainerLogs(ctx, containerID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tailStr,
	})
	if err != nil {
		return fmt.Errorf("fetching logs: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	if _, err := stdcopy.StdCopy(stdout, stderr, rc); err != nil {
		return fmt.Errorf("streaming logs: %w", err)
	}
	return nil
}
