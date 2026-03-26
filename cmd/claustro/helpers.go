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
