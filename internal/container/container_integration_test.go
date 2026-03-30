// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build integration

package container

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	"github.com/uniforgeai/claustro/internal/mount"
)

func testClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	t.Cleanup(func() { cli.Close() })
	return cli
}

func TestContainerLifecycle(t *testing.T) {
	ctx := context.Background()
	cli := testClient(t)

	// Ensure image exists
	require.NoError(t, image.EnsureBuilt(ctx, cli))

	id, err := identity.FromCWD("integration-test")
	require.NoError(t, err)

	cwd, _ := os.Getwd()
	mounts, err := mount.Assemble(cwd, nil, "", false, false)
	require.NoError(t, err)

	// Cleanup any leftover container from previous runs
	_ = Stop(ctx, cli, id.ContainerName())
	_ = Remove(ctx, cli, id.ContainerName())

	t.Cleanup(func() {
		_ = Stop(ctx, cli, id.ContainerName())
		_ = Remove(ctx, cli, id.ContainerName())
		cli.NetworkRemove(ctx, id.NetworkName()) //nolint:errcheck
	})

	// Create
	containerID, err := Create(ctx, cli, id, mounts)
	require.NoError(t, err)
	assert.NotEmpty(t, containerID)

	// Start
	require.NoError(t, Start(ctx, cli, containerID))

	// FindByIdentity — should find the running container
	found, err := FindByIdentity(ctx, cli, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, containerID, found.ID)

	// ListByProject — should include our container
	containers, err := ListByProject(ctx, cli, id.Project, false)
	require.NoError(t, err)
	assert.NotEmpty(t, containers)

	// ListByProject --all — should also include it
	all, err := ListByProject(ctx, cli, "", true)
	require.NoError(t, err)
	assert.NotEmpty(t, all)

	// Inspect — should return container details
	info, err := Inspect(ctx, cli, containerID)
	require.NoError(t, err)
	assert.Equal(t, id.Labels()["claustro.project"], info.Config.Labels["claustro.project"])

	// Stop + Remove
	require.NoError(t, Stop(ctx, cli, containerID))
	require.NoError(t, Remove(ctx, cli, containerID))

	// FindByIdentity — should return nil now
	gone, err := FindByIdentity(ctx, cli, id)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestRemoveNetwork(t *testing.T) {
	ctx := context.Background()
	cli := testClient(t)

	id, err := identity.FromCWD("net-test")
	require.NoError(t, err)

	// Ensure no leftover network
	_ = cli.NetworkRemove(ctx, id.NetworkName())

	// RemoveNetwork on non-existent network should be a no-op
	require.NoError(t, RemoveNetwork(ctx, cli, id.NetworkName()))

	// Create network, then remove it
	_, err = cli.NetworkCreate(ctx, id.NetworkName(), networktypes.CreateOptions{
		Driver: "bridge",
		Labels: id.Labels(),
	})
	require.NoError(t, err)

	require.NoError(t, RemoveNetwork(ctx, cli, id.NetworkName()))

	// Verify it's gone
	args := filters.NewArgs(filters.Arg("name", "^"+id.NetworkName()+"$"))
	networks, err := cli.NetworkList(ctx, networktypes.ListOptions{Filters: args})
	require.NoError(t, err)
	assert.Empty(t, networks)
}
