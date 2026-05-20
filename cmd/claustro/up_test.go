// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"testing"

	dockermount "github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/identity"
)

func TestWorkspaceHostPath_DefaultsToProjectRoot(t *testing.T) {
	assert.Equal(t, "/repo", workspaceHostPath("/repo", ""))
}

func TestWorkspaceHostPath_RelativeWorkdir(t *testing.T) {
	assert.Equal(t, "/repo/services/api", workspaceHostPath("/repo", "./services/api"))
}

func TestWorkspaceHostPath_AbsoluteWorkdir(t *testing.T) {
	assert.Equal(t, "/tmp/other", workspaceHostPath("/repo", "/tmp/other"))
}

func TestDockerMountsFromResolvedIncludesAdditionalMounts(t *testing.T) {
	got := dockerMountsFromResolved([]config.Mount{
		{HostPath: "/repo/libs", ContainerPath: "/workspace/libs", ReadOnly: true},
		{HostPath: "/repo/tmp", ContainerPath: "/tmp/repo", ReadOnly: false},
	})

	require.Len(t, got, 2)
	assert.Equal(t, dockermount.TypeBind, got[0].Type)
	assert.Equal(t, "/repo/libs", got[0].Source)
	assert.Equal(t, "/workspace/libs", got[0].Target)
	assert.True(t, got[0].ReadOnly)
	assert.Equal(t, "/repo/tmp", got[1].Source)
	assert.Equal(t, "/tmp/repo", got[1].Target)
	assert.False(t, got[1].ReadOnly)
}

func TestBuildImageIfNeeded_CustomImageSkipsBuild(t *testing.T) {
	opts, err := buildImageIfNeeded(context.Background(), nil, &identity.Identity{Project: "proj"}, &config.Config{
		ImageName: "registry.example.com/custom:latest",
	})

	require.NoError(t, err)
	assert.Equal(t, "registry.example.com/custom:latest", opts.ImageName)
}

func TestContainerStatusHelpers(t *testing.T) {
	assert.True(t, containerStatusIsUp("Up 2 minutes"))
	assert.True(t, containerStatusIsUp("Up 2 minutes (Paused)"))
	assert.False(t, containerStatusIsUp("Exited (0) 1 minute ago"))

	assert.True(t, shouldStartExistingContainer(false, "Exited (0) 1 minute ago"))
	assert.False(t, shouldStartExistingContainer(true, "Exited (0) 1 minute ago"))
	assert.False(t, shouldStartExistingContainer(false, "Up 2 minutes"))
}
