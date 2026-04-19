// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/mcp"
)

const defaultTimeout = 5 * time.Minute

// Run is the daemon entrypoint. Returns when no claustro containers remain or
// when ctx is cancelled. Logs go to ~/.claustro/claustrod.log (stderr is
// /dev/null in the detached process).
func Run(ctx context.Context) error {
	if err := setupLogging(); err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close() //nolint:errcheck

	pidPath, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("pidfile path: %w", err)
	}
	if err := writePidFile(pidPath); err != nil {
		return fmt.Errorf("writing pidfile: %w", err)
	}
	defer os.Remove(pidPath) //nolint:errcheck

	state := map[string]Track{}
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			done, err := tick(ctx, cli, &state, now)
			if err != nil {
				slog.Warn("daemon tick", "err", err)
				continue
			}
			if done {
				slog.Info("no claustro containers remain, daemon exiting")
				return nil
			}
		}
	}
}

// tick performs one poll cycle. Returns done=true when no claustro containers exist.
func tick(ctx context.Context, cli *client.Client, state *map[string]Track, now time.Time) (bool, error) {
	containers, err := listClaustroContainers(ctx, cli)
	if err != nil {
		return false, err
	}
	if len(containers) == 0 {
		return true, nil
	}

	views := make([]ContainerView, 0, len(containers))
	for _, c := range containers {
		view, ok := buildView(ctx, cli, c)
		if !ok {
			continue
		}
		views = append(views, view)
	}

	toPause, newState := Decide(*state, views, now, defaultTimeout)
	*state = newState

	for _, id := range toPause {
		if err := container.Pause(ctx, cli, id); err != nil {
			slog.Warn("pausing container", "id", id, "err", err)
			(*state)[id] = Track{LastActive: now} // back off retrying
			continue
		}
		slog.Info("paused idle sandbox", "id", id)
		pauseSiblings(ctx, cli, id, containers)
	}
	return false, nil
}

// listClaustroContainers returns containers labeled by the sandbox role
// (excludes MCP siblings — those are handled together with their parent).
func listClaustroContainers(ctx context.Context, cli *client.Client) ([]containertypes.Summary, error) {
	args := containertypes.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", identity.LabelManaged+"=true"),
		),
	}
	all, err := cli.ContainerList(ctx, args)
	if err != nil {
		return nil, err
	}
	out := make([]containertypes.Summary, 0, len(all))
	for _, c := range all {
		if c.Labels[identity.LabelRole] == "mcp-sse" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// buildView turns a container summary into the daemon's ContainerView.
// Returns ok=false if the container should be skipped this tick (e.g. project
// has pause disabled, or inspect failed).
func buildView(ctx context.Context, cli *client.Client, c containertypes.Summary) (ContainerView, bool) {
	hostPath := c.Labels[identity.LabelHostPath]
	timeout := time.Duration(0)
	if hostPath != "" {
		cfg, err := config.Load(hostPath)
		if err == nil {
			if !cfg.Pause.IsEnabled() {
				return ContainerView{}, false
			}
			timeout = cfg.Pause.Timeout()
		}
	}

	inspect, err := cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		return ContainerView{}, false
	}
	state := inspect.State.Status

	active := 0
	for _, execID := range inspect.ExecIDs {
		ei, err := cli.ContainerExecInspect(ctx, execID)
		if err != nil {
			continue
		}
		if ei.Running {
			active++
		}
	}
	return ContainerView{
		ID:              c.ID,
		State:           state,
		ActiveExecCount: active,
		Timeout:         timeout,
	}, true
}

// pauseSiblings pauses the MCP SSE siblings of the given parent.
func pauseSiblings(ctx context.Context, cli *client.Client, parentID string, all []containertypes.Summary) {
	var parent containertypes.Summary
	for _, c := range all {
		if c.ID == parentID {
			parent = c
			break
		}
	}
	if parent.ID == "" {
		return
	}
	id := &identity.Identity{
		Project: parent.Labels[identity.LabelProject],
		Name:    parent.Labels[identity.LabelName],
	}
	siblings, err := mcp.ListSSESiblings(ctx, cli, id)
	if err != nil {
		slog.Warn("listing siblings for pause", "parent", parentID, "err", err)
		return
	}
	for _, sib := range siblings {
		if err := container.Pause(ctx, cli, sib.ID); err != nil {
			slog.Warn("pausing MCP sibling", "parent", parentID, "sibling", sib.ID, "err", err)
		}
	}
}

func pidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claustro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "claustrod.pid"), nil
}

func writePidFile(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

// setupLogging routes slog output to ~/.claustro/claustrod.log (append).
// No rotation in v1; manual truncate is fine for a perf daemon.
// The log file is intentionally not closed: its lifetime equals the process
// lifetime and the OS reclaims the FD on exit.
func setupLogging() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".claustro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "claustrod.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return nil
}

// IsAlive returns true when a daemon process is recorded in the pidfile and is
// reachable (signal 0). False if no pidfile or stale.
//
// Note: this is not race-free. Two simultaneous `claustro up` invocations can
// both observe IsAlive==false and both spawn a daemon. The damage is minor: a
// duplicate poll loop until one notices the same pidfile got rewritten and
// exits next tick. v1 accepts this; flock-based singleton is a follow-up.
func IsAlive() bool {
	path, err := pidFilePath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
