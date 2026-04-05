// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/term"
)

// setRawTerminal puts the host terminal into raw mode and returns the saved
// state so that it can be restored later. Returns (nil, nil) when stdin is not
// a terminal.
func setRawTerminal() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// restoreTerminal restores the host terminal to the given saved state. It is a
// no-op when state is nil.
func restoreTerminal(state *term.State) {
	if state != nil {
		term.Restore(int(os.Stdin.Fd()), state) //nolint:errcheck
	}
}

// getTerminalSize returns the current host terminal dimensions.
// Falls back to (80, 24) if dimensions cannot be determined.
func getTerminalSize(fd int) (width, height uint) {
	w, h, err := term.GetSize(fd)
	if err != nil || w <= 0 || h <= 0 {
		return 80, 24
	}
	return uint(w), uint(h)
}

// monitorResizeEvents forwards SIGWINCH signals to the container exec PTY so
// the container tracks the host terminal size after window resizes. It runs
// until ctx is cancelled.
func monitorResizeEvents(ctx context.Context, apiClient client.APIClient, execID string, fd int) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			w, h := getTerminalSize(fd)
			_ = apiClient.ContainerExecResize(ctx, execID, containertypes.ResizeOptions{
				Width:  w,
				Height: h,
			})
		}
	}
}

// gitEnv returns Git environment variables to inject into interactive exec sessions.
// When the host SSH agent is available (SSH_AUTH_SOCK set), it overrides
// gpg.ssh.program to ssh-keygen, which uses the forwarded agent for commit
// signing. This replaces host-specific signing helpers (e.g. op-ssh-sign) that
// do not exist inside the container.
func gitEnv() []string {
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		return nil
	}
	return []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=gpg.ssh.program",
		"GIT_CONFIG_VALUE_0=ssh-keygen",
	}
}

// termEnv returns terminal-related environment variables read from the host,
// suitable for injecting into interactive exec sessions. TERM defaults to
// xterm-256color when not set on the host.
func termEnv() []string {
	termVal := os.Getenv("TERM")
	if termVal == "" {
		termVal = "xterm-256color"
	}
	env := []string{"TERM=" + termVal}
	for _, key := range []string{"COLORTERM", "LANG", "LC_ALL"} {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}
