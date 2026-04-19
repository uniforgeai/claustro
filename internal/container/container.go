// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package container manages Docker container lifecycle for claustro sandboxes.
package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"golang.org/x/term"
	dockertypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/clipboard"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/image"
	claustromount "github.com/uniforgeai/claustro/internal/mount"
	"github.com/uniforgeai/claustro/internal/sysinfo"
)

// Default resource limits applied when no config override is provided.
const (
	defaultNanoCPUs = 4_000_000_000          // 4 CPUs
	defaultMemory   = 8 * 1024 * 1024 * 1024 // 8 GiB
)

// Magic-number constants used across container operations.
const (
	defaultStopTimeout = 10                            // seconds to wait before SIGKILL
	networkDriver      = "bridge"                      // Docker network driver
	containerUser      = "sandbox"                     // user inside container
	noNewPrivileges    = "no-new-privileges:true"      // security option
	capNetAdmin        = "NET_ADMIN"                   // capability for firewall
	nanosecondsPerCPU  = 1e9                           // nanoseconds per 1 CPU
	containerWorkdir   = "/workspace"                  // working directory for exec
	containerHome      = "/home/sandbox"               // home directory inside container
)

// CreateOptions configures optional parameters for container creation.
type CreateOptions struct {
	// ImageName overrides the default claustro:latest image.
	// If empty, image.ImageName is used.
	ImageName string
	// Firewall enables egress firewall. When true, the container is granted
	// NET_ADMIN capability so that iptables rules can be applied after start.
	Firewall bool
	// CPUs is the number of CPUs to allocate (e.g. "2", "0.5").
	// If empty and Host is set, smartCPUs(Host) is used; otherwise defaultNanoCPUs.
	CPUs string
	// Memory is the memory limit (e.g. "8G", "512M", "1024K").
	// If empty and Host is set, smartMemory(Host) is used; otherwise defaultMemory.
	Memory string
	// Host is the detected host machine. When set and CPUs/Memory are empty,
	// resource caps are computed proportional to the host.
	Host *sysinfo.Host
}

// sandboxEnv assembles environment variables for a new sandbox container.
func sandboxEnv(hostPath string) []string {
	env := []string{
		"CLAUSTRO_HOST_PATH=" + hostPath,
		"HOME=" + containerHome,
	}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		env = append(env, "SSH_AUTH_SOCK="+claustromount.SSHAgentContainerSock(sock))
	}
	// Forward API keys from host environment.
	for _, key := range []string{"OPENAI_API_KEY"} {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}
	return env
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

	env := sandboxEnv(id.HostPath)

	cfg := &containertypes.Config{
		Image:  imageName,
		Labels: id.Labels(),
		Env:    env,
		Tty:          false,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
	}

	nanoCPUs, err := parseNanoCPUsForHost(opts.CPUs, opts.Host)
	if err != nil {
		return "", fmt.Errorf("parsing cpus %q: %w", opts.CPUs, err)
	}
	memBytes, err := parseMemoryForHost(opts.Memory, opts.Host)
	if err != nil {
		return "", fmt.Errorf("parsing memory %q: %w", opts.Memory, err)
	}

	hostCfg := &containertypes.HostConfig{
		Mounts:      mounts,
		SecurityOpt: []string{noNewPrivileges},
		Resources: containertypes.Resources{
			NanoCPUs: nanoCPUs,
			Memory:   memBytes,
		},
	}
	if opts.Firewall {
		hostCfg.CapAdd = []string{capNetAdmin}
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

// parseNanoCPUs converts a CPU string (e.g. "2", "0.5") to Docker NanoCPUs.
// Returns defaultNanoCPUs if s is empty.
func parseNanoCPUs(s string) (int64, error) {
	if s == "" {
		return defaultNanoCPUs, nil
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cpu value: %w", err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("cpus must be positive, got %v", val)
	}
	return int64(val * nanosecondsPerCPU), nil
}

// parseMemory converts a memory string (e.g. "8G", "512M", "1024K") to bytes.
// Returns defaultMemory if s is empty.
func parseMemory(s string) (int64, error) {
	if s == "" {
		return defaultMemory, nil
	}
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid memory value %q", s)
	}
	suffix := strings.ToUpper(s[len(s)-1:])
	numStr := s[:len(s)-1]
	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory number %q: %w", numStr, err)
	}
	if val <= 0 {
		return 0, fmt.Errorf("memory must be positive, got %v", val)
	}
	switch suffix {
	case "K":
		return val * 1024, nil
	case "M":
		return val * 1024 * 1024, nil
	case "G":
		return val * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown memory suffix %q, expected G, M, or K", suffix)
	}
}

// smartCPUs returns nanoCPUs computed from the host: max(2, host_cores/4).
// Used when no explicit cpus value is set in claustro.yaml.
func smartCPUs(h *sysinfo.Host) int64 {
	cores := h.CPUs / 4
	if cores < 2 {
		cores = 2
	}
	return int64(cores) * nanosecondsPerCPU
}

// smartMemory returns bytes computed from the host: min(8 GiB, host_mem/4).
func smartMemory(h *sysinfo.Host) int64 {
	quarter := h.MemoryBytes / 4
	if quarter < defaultMemory {
		return quarter
	}
	return defaultMemory
}

// parseNanoCPUsForHost is parseNanoCPUs with a host-aware default.
// When s is empty, returns smartCPUs(host); otherwise delegates to parseNanoCPUs(s).
func parseNanoCPUsForHost(s string, host *sysinfo.Host) (int64, error) {
	if s == "" {
		if host == nil {
			return defaultNanoCPUs, nil
		}
		return smartCPUs(host), nil
	}
	return parseNanoCPUs(s)
}

// parseMemoryForHost is parseMemory with a host-aware default.
func parseMemoryForHost(s string, host *sysinfo.Host) (int64, error) {
	if s == "" {
		if host == nil {
			return defaultMemory, nil
		}
		return smartMemory(host), nil
	}
	return parseMemory(s)
}

// Start starts an existing container.
func Start(ctx context.Context, cli *client.Client, containerID string) error {
	if err := cli.ContainerStart(ctx, containerID, containertypes.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	return nil
}

// ExecOptions configures an Exec call.
type ExecOptions struct {
	// Interactive attaches stdin/stdout/stderr and allocates a TTY.
	Interactive bool
	// ClipboardSockDir is the host directory where the clipboard bridge socket will
	// be created. When non-empty and Interactive is true, a clipboard server is
	// started for the duration of the exec session.
	ClipboardSockDir string
}

// Exec runs a command inside a running container.
func Exec(ctx context.Context, cli *client.Client, containerID string, cmd []string, opts ExecOptions) error {
	execCfg := containertypes.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  opts.Interactive,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          opts.Interactive,
		User:         containerUser,
		WorkingDir:   containerWorkdir,
	}
	cleanup := setupClipboardBridge(opts)
	defer cleanup()

	if opts.Interactive {
		execCfg.Env = append(termEnv(), gitEnv()...)
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, containertypes.ExecStartOptions{Tty: opts.Interactive})
	if err != nil {
		return fmt.Errorf("attaching to exec: %w", err)
	}
	defer resp.Close()

	if opts.Interactive {
		teardown := setupInteractiveSession(ctx, cli, execID.ID, resp)
		defer teardown()
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

// setupClipboardBridge starts a clipboard bridge server for interactive sessions
// when a socket directory is provided. It returns a cleanup function that stops
// the server. On macOS, TCP is used because Unix sockets are not reachable from
// inside Docker's Linux VM.
func setupClipboardBridge(opts ExecOptions) func() {
	noop := func() {}
	if !opts.Interactive || opts.ClipboardSockDir == "" {
		return noop
	}

	srv := clipboard.New(clipboard.NewPlatformHandler())
	if runtime.GOOS == "darwin" {
		if _, err := srv.StartTCP(opts.ClipboardSockDir); err != nil {
			slog.Warn("clipboard bridge unavailable", "err", err)
			return noop
		}
	} else {
		sockPath := filepath.Join(opts.ClipboardSockDir, "clipboard.sock")
		if err := srv.Start(sockPath); err != nil {
			slog.Warn("clipboard bridge unavailable", "err", err)
			return noop
		}
	}
	return func() { srv.Close() } //nolint:errcheck
}

// setupInteractiveSession configures raw terminal mode, initial PTY sizing,
// resize monitoring, and I/O forwarding for an interactive exec session.
// It returns a cleanup function that restores the terminal and cancels resize
// monitoring.
func setupInteractiveSession(ctx context.Context, cli *client.Client, execID string, resp dockertypes.HijackedResponse) func() {
	var termState *term.State
	if state, err := setRawTerminal(); err == nil {
		termState = state
	}

	fd := int(os.Stdin.Fd())

	// Set the container PTY to the host terminal's current dimensions.
	w, h := getTerminalSize(fd)
	_ = cli.ContainerExecResize(ctx, execID, containertypes.ResizeOptions{Width: w, Height: h})

	// Forward future resize events for the lifetime of this session.
	resizeCtx, cancelResize := context.WithCancel(ctx)
	go monitorResizeEvents(resizeCtx, cli, execID, fd)

	// When Tty=true Docker streams raw PTY bytes without the 8-byte
	// multiplexing header, so plain io.Copy is correct here.
	go io.Copy(resp.Conn, os.Stdin) //nolint:errcheck
	io.Copy(os.Stdout, resp.Reader) //nolint:errcheck

	return func() {
		cancelResize()
		restoreTerminal(termState)
	}
}

// Stop stops a running container (10 second timeout).
func Stop(ctx context.Context, cli *client.Client, containerID string) error {
	timeout := defaultStopTimeout
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

// EnsureVolume creates a named Docker volume if it does not already exist (idempotent).
// Labels are applied to the volume on creation.
func EnsureVolume(ctx context.Context, cli *client.Client, name string, labels map[string]string) error {
	args := filters.NewArgs(filters.Arg("name", name))
	list, err := cli.VolumeList(ctx, volumetypes.ListOptions{Filters: args})
	if err != nil {
		return fmt.Errorf("listing volumes: %w", err)
	}
	for _, v := range list.Volumes {
		if v.Name == name {
			return nil
		}
	}
	_, err = cli.VolumeCreate(ctx, volumetypes.CreateOptions{
		Name:   name,
		Labels: labels,
	})
	if err != nil {
		return fmt.Errorf("creating volume %q: %w", name, err)
	}
	return nil
}

// RemoveVolume removes a named Docker volume, ignoring not-found errors.
func RemoveVolume(ctx context.Context, cli *client.Client, name string) error {
	err := cli.VolumeRemove(ctx, name, false)
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("removing volume %q: %w", name, err)
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
		Driver: networkDriver,
		Labels: id.Labels(),
	})
	if err != nil {
		return fmt.Errorf("creating network: %w", err)
	}
	return nil
}

// ExecSimple runs a non-interactive command inside a running container and returns any error.
// It captures stdout/stderr but does not stream them.
func ExecSimple(ctx context.Context, cli *client.Client, containerID string, cmd []string) error {
	execCfg := containertypes.ExecOptions{
		Cmd:          cmd,
		User:         containerUser,
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, containertypes.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("attaching to exec: %w", err)
	}
	defer resp.Close()

	var output bytes.Buffer
	io.Copy(&output, resp.Reader) //nolint:errcheck

	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("inspecting exec: %w", err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("command %v exited %d: %s", cmd, inspect.ExitCode, output.String())
	}
	return nil
}
