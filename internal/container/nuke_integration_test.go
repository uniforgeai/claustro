//go:build integration

package container

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	"github.com/uniforgeai/claustro/internal/mount"
)

func TestNuke(t *testing.T) {
	ctx := context.Background()
	cli := testClient(t)

	require.NoError(t, image.EnsureBuilt(ctx, cli))

	id, err := identity.FromCWD("nuke-test")
	require.NoError(t, err)

	cwd, _ := os.Getwd()
	mounts, err := mount.Assemble(cwd, nil, "", false, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = Stop(ctx, cli, id.ContainerName())
		_ = Remove(ctx, cli, id.ContainerName())
		_ = RemoveNetwork(ctx, cli, id.NetworkName())
	})

	// Create and start a sandbox
	containerID, err := Create(ctx, cli, id, mounts)
	require.NoError(t, err)
	require.NoError(t, Start(ctx, cli, containerID))

	// Verify it exists
	found, err := FindByIdentity(ctx, cli, id)
	require.NoError(t, err)
	require.NotNil(t, found)

	// Nuke: stop + remove + remove network
	require.NoError(t, Stop(ctx, cli, found.ID))
	require.NoError(t, Remove(ctx, cli, found.ID))
	require.NoError(t, RemoveNetwork(ctx, cli, id.NetworkName()))

	// Verify container is gone
	gone, err := FindByIdentity(ctx, cli, id)
	require.NoError(t, err)
	assert.Nil(t, gone)
}
