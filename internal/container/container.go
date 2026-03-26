// Package container manages Docker container lifecycle for claustro sandboxes.
package container

import (
	"context"
	"fmt"
	"io"
	"os"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
)

// CreateOptions configures optional parameters for container creation.
type CreateOptions struct {
	// ImageName overrides the default claustro:latest image.
	// If empty, image.ImageName is used.
	ImageName string
}

// Create creates (but does not start) a sandbox container.
func Create(ctx context.Context, cli *client.Client, id *identity.Identity, mounts []mount.Mount, opts CreateOptions) (string, error) {
	// Ensure the sandbox network exists
	if err := ensureNetwork(ctx, cli, id); err != nil {
		return "", fmt.Errorf("ensuring network: %w", err)
	}

	imageName := opts.ImageName
	if imageName == "" {
		imageName = image.ImageName
	}

	env := []string{
		"CLAUSTRO_HOST_PATH=" + id.HostPath,
		"HOME=/home/sandbox",
	}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		env = append(env, "SSH_AUTH_SOCK="+sock)
	}

	cfg := &containertypes.Config{
		Image:  imageName,
		Labels: id.Labels(),
		Env:    env,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
	}

	hostCfg := &containertypes.HostConfig{
		Mounts:      mounts,
		SecurityOpt: []string{"no-new-privileges:true"},
		Resources: containertypes.Resources{
			NanoCPUs: 4_000_000_000,
			Memory:   8 * 1024 * 1024 * 1024,
		},
	}

	netCfg := &networktypes.NetworkingConfig{
		EndpointsConfig: map[string]*networktypes.EndpointSettings{
			id.NetworkName(): {},
		},
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, id.ContainerName())
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}
	return resp.ID, nil
}

// Start starts an existing container.
func Start(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerStart(ctx, containerID, containertypes.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	return nil
}

// Exec runs a command inside a running container.
// If interactive is true, stdin/stdout/stderr are attached and a TTY is allocated.
func Exec(ctx context.Context, cli *client.Client, containerID string, cmd []string, interactive bool) error {
	execCfg := containertypes.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  interactive,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          interactive,
		User:         "sandbox",
		WorkingDir:   "/workspace",
	}
	if interactive {
		execCfg.Env = termEnv()
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, containertypes.ExecStartOptions{Tty: interactive})
	if err != nil {
		return fmt.Errorf("attaching to exec: %w", err)
	}
	defer resp.Close()

	if interactive {
		// Set raw terminal mode for interactive sessions.
		if err := setRawTerminal(); err == nil {
			defer restoreTerminal()
		}

		fd := int(os.Stdin.Fd())

		// Set the container PTY to the host terminal's current dimensions.
		w, h := getTerminalSize(fd)
		_ = cli.ContainerExecResize(ctx, execID.ID, containertypes.ResizeOptions{Width: w, Height: h})

		// Forward future resize events for the lifetime of this session.
		resizeCtx, cancelResize := context.WithCancel(ctx)
		defer cancelResize()
		go monitorResizeEvents(resizeCtx, cli, execID.ID, fd)

		// When Tty=true Docker streams raw PTY bytes without the 8-byte
		// multiplexing header, so plain io.Copy is correct here.
		go io.Copy(resp.Conn, os.Stdin)  //nolint:errcheck
		io.Copy(os.Stdout, resp.Reader)  //nolint:errcheck
	} else {
		io.Copy(os.Stdout, resp.Reader) //nolint:errcheck
	}

	// Check exit code
	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("inspecting exec: %w", err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}
	return nil
}

// Stop stops a running container (10 second timeout).
func Stop(ctx context.Context, cli *client.Client, containerID string) error {
	timeout := 10
	if err := cli.ContainerStop(ctx, containerID, containertypes.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stopping container: %w", err)
	}
	return nil
}

// Remove removes a stopped container.
func Remove(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerRemove(ctx, containerID, containertypes.RemoveOptions{}); err != nil {
		return fmt.Errorf("removing container: %w", err)
	}
	return nil
}

// FindByIdentity finds a container matching the given sandbox identity.
// Returns nil if no container is found.
func FindByIdentity(ctx context.Context, cli *client.Client, id *identity.Identity) (*containertypes.Summary, error) {
	args := filters.NewArgs(
		filters.Arg("label", "claustro.project="+id.Project),
		filters.Arg("label", "claustro.name="+id.Name),
	)
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}
	if len(containers) == 0 {
		return nil, nil
	}
	return &containers[0], nil
}

// ListByProject returns all claustro containers for the given project.
// If allProjects is true, returns all claustro-managed containers across all projects.
func ListByProject(ctx context.Context, cli *client.Client, project string, allProjects bool) ([]containertypes.Summary, error) {
	args := filters.NewArgs()
	if allProjects {
		args.Add("label", "claustro.managed=true")
	} else {
		args.Add("label", "claustro.project="+project)
	}
	containers, err := cli.ContainerList(ctx, containertypes.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}
	return containers, nil
}

// Inspect returns detailed information about a container.
func Inspect(ctx context.Context, cli *client.Client, containerID string) (containertypes.InspectResponse, error) {
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return containertypes.InspectResponse{}, fmt.Errorf("inspecting container: %w", err)
	}
	return info, nil
}

// RemoveNetwork removes a Docker network by name, ignoring not-found errors.
func RemoveNetwork(ctx context.Context, cli *client.Client, networkName string) error {
	args := filters.NewArgs(filters.Arg("name", "^"+networkName+"$"))
	networks, err := cli.NetworkList(ctx, networktypes.ListOptions{Filters: args})
	if err != nil {
		return fmt.Errorf("listing networks: %w", err)
	}
	if len(networks) == 0 {
		return nil
	}
	if err := cli.NetworkRemove(ctx, networks[0].ID); err != nil {
		return fmt.Errorf("removing network: %w", err)
	}
	return nil
}

func ensureNetwork(ctx context.Context, cli *client.Client, id *identity.Identity) error {
	args := filters.NewArgs(filters.Arg("name", "^"+id.NetworkName()+"$"))
	networks, err := cli.NetworkList(ctx, networktypes.ListOptions{Filters: args})
	if err != nil {
		return fmt.Errorf("listing networks: %w", err)
	}
	if len(networks) > 0 {
		return nil
	}
	_, err = cli.NetworkCreate(ctx, id.NetworkName(), networktypes.CreateOptions{
		Driver: "bridge",
		Labels: id.Labels(),
	})
	if err != nil {
		return fmt.Errorf("creating network: %w", err)
	}
	return nil
}
