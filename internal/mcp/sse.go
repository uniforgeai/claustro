package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/identity"
)

const defaultSSEPort = 8000

// effectivePort returns the port to use, defaulting to 8000 if zero.
func effectivePort(port int) int {
	if port == 0 {
		return defaultSSEPort
	}
	return port
}

// EndpointURL returns the SSE endpoint URL for a server name and port.
func EndpointURL(serverName string, port int) string {
	return fmt.Sprintf("http://%s:%d/sse", serverName, effectivePort(port))
}

// SSEEntries builds mcp.json ServerEntry entries for SSE servers.
func SSEEntries(servers map[string]config.MCPSSE) Config {
	if len(servers) == 0 {
		return Config{}
	}
	entries := make(map[string]ServerEntry, len(servers))
	for name, srv := range servers {
		entries[name] = ServerEntry{
			URL: EndpointURL(name, srv.Port),
		}
	}
	return Config{MCPServers: entries}
}

// StartSSESiblings creates and starts sibling containers for all configured SSE MCP servers.
// Failures are logged as warnings but do not prevent the sandbox from running.
func StartSSESiblings(ctx context.Context, cli *client.Client, id *identity.Identity, servers map[string]config.MCPSSE) {
	for name, srv := range servers {
		if err := startOneSibling(ctx, cli, id, name, srv); err != nil {
			slog.Warn("MCP SSE sibling failed to start", "server", name, "err", err)
		}
	}
}

func startOneSibling(ctx context.Context, cli *client.Client, id *identity.Identity, serverName string, srv config.MCPSSE) error {
	containerName := id.MCPContainerName(serverName)

	env := make([]string, 0, len(srv.Env))
	for k, v := range srv.Env {
		env = append(env, k+"="+v)
	}

	cfg := &containertypes.Config{
		Image:  srv.Image,
		Labels: id.MCPLabels(serverName),
		Env:    env,
	}

	hostCfg := &containertypes.HostConfig{}

	netCfg := &networktypes.NetworkingConfig{
		EndpointsConfig: map[string]*networktypes.EndpointSettings{
			id.NetworkName(): {
				Aliases: []string{serverName},
			},
		},
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		return fmt.Errorf("creating MCP container %q: %w", containerName, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, containertypes.StartOptions{}); err != nil {
		return fmt.Errorf("starting MCP container %q: %w", containerName, err)
	}

	slog.Info("MCP SSE sibling started", "server", serverName, "container", containerName)
	return nil
}

// StopSSESiblings finds and removes all MCP SSE sibling containers for the given identity.
// Failures are logged as warnings but do not abort cleanup.
func StopSSESiblings(ctx context.Context, cli *client.Client, id *identity.Identity) {
	siblings, err := listSiblings(ctx, cli, id)
	if err != nil {
		slog.Warn("failed to list MCP siblings for cleanup", "err", err)
		return
	}

	for _, c := range siblings {
		name := strings.TrimPrefix(c.Names[0], "/")
		timeout := 10
		if err := cli.ContainerStop(ctx, c.ID, containertypes.StopOptions{Timeout: &timeout}); err != nil {
			slog.Warn("failed to stop MCP sibling", "container", name, "err", err)
		}
		if err := cli.ContainerRemove(ctx, c.ID, containertypes.RemoveOptions{}); err != nil {
			slog.Warn("failed to remove MCP sibling", "container", name, "err", err)
		} else {
			slog.Info("removed MCP sibling", "container", name)
		}
	}
}

func listSiblings(ctx context.Context, cli *client.Client, id *identity.Identity) ([]containertypes.Summary, error) {
	args := containertypes.ListOptions{
		All: true,
		Filters: filtersMCPSiblings(id),
	}
	return cli.ContainerList(ctx, args)
}

func filtersMCPSiblings(id *identity.Identity) filters.Args {
	return filters.NewArgs(
		filters.Arg("label", "claustro.project="+id.Project),
		filters.Arg("label", "claustro.name="+id.Name),
		filters.Arg("label", "claustro.role=mcp-sse"),
	)
}
