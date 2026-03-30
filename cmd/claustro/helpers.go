// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"

	"github.com/docker/docker/client"
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
