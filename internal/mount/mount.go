// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package mount assembles Docker bind mounts for a claustro sandbox.
package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/uniforgeai/claustro/internal/config"
)

// Container path constants.
const (
	containerHome       = "/home/sandbox"
	containerClaudeDir  = containerHome + "/.claude"
	containerClaudeJSON = containerHome + "/.claude.json"
	containerPluginsDir = containerClaudeDir + "/plugins"
	containerGitconfig  = containerHome + "/.gitconfig"
	containerGHConfig   = containerHome + "/.config/gh"
	containerSSHDir     = containerHome + "/.ssh"
	containerKnownHosts = containerSSHDir + "/known_hosts"
	containerCodexDir   = containerHome + "/.codex"
	containerWorkspace  = "/workspace"
	clipboardRunDir     = "/run/claustro"
)

// File permission constants.
const (
	dirModeWorld = 0o777
)

// dockerDesktopRelayPath is the SSH agent socket path that macOS container runtimes
// (Docker Desktop and OrbStack) expose inside containers. The host SSH agent socket
// lives on the macOS side and cannot be reached from the Linux VM; both runtimes
// synthesize this well-known relay path inside the container instead.
const dockerDesktopRelayPath = "/run/host-services/ssh-auth.sock"

// SSHAgentContainerSock returns the SSH_AUTH_SOCK value to set inside the container.
// On macOS, Docker containers run in a Linux VM and cannot access the macOS-side SSH
// agent socket directly. Docker Desktop and OrbStack both relay it at a well-known
// fixed path. On Linux, the host socket path is used unchanged.
func SSHAgentContainerSock(hostSock string) string {
	if runtime.GOOS == "darwin" {
		return dockerDesktopRelayPath
	}
	return hostSock
}

// Assemble returns the bind mounts needed for a sandbox:
//   - hostProjectPath      → /workspace  (source, read-write by default; read-only when readOnly=true)
//   - ~/.claude            → /home/sandbox/.claude  (Claude state, read-write, skipped when isolatedState=true)
//   - ~/.claude.json       → /home/sandbox/.claude.json  (Claude config, read-write, if exists; skipped when isolatedState=true)
//
// Git/GitHub mounts are added conditionally based on git config:
//   - ~/.gitconfig         → /home/sandbox/.gitconfig  (read-only, if exists and enabled)
//   - ~/.config/gh/        → /home/sandbox/.config/gh/  (read-write, if exists and enabled)
//   - ~/.ssh/known_hosts   → /home/sandbox/.ssh/known_hosts  (read-only, if exists)
//   - SSH agent socket     → container path via SSHAgentContainerSock  (if agent forwarding enabled)
//   - ~/.ssh/*.pub         → /home/sandbox/.ssh/*.pub  (read-only, if agent forwarding enabled)
//   - ~/.ssh/              → /home/sandbox/.ssh/  (read-only, if exists and explicitly enabled)
//
// When the host home differs from the container home (/home/sandbox), the plugins
// directory is also mounted at its original host path (read-only) so Claude Code can
// resolve the absolute paths stored in installed_plugins.json and known_marketplaces.json.
// This remount is skipped when isolatedState=true.
//
// clipboardSockDir, when non-empty, is created on the host and mounted at /run/claustro
// inside the container so the clipboard bridge socket is accessible to shim scripts.
func Assemble(hostProjectPath string, git *config.GitConfig, clipboardSockDir string, readOnly, isolatedState bool) ([]mount.Mount, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	if git == nil {
		git = &config.GitConfig{}
	}

	var mounts []mount.Mount
	addWorkspaceMount(&mounts, hostProjectPath, readOnly)
	addClaudeMounts(&mounts, home, isolatedState)
	addCodexMounts(&mounts, home, isolatedState)
	addGitMounts(&mounts, home, git)
	addSSHMounts(&mounts, home, git)
	addPluginMounts(&mounts, home, isolatedState)
	if err := addClipboardMount(&mounts, clipboardSockDir); err != nil {
		return nil, err
	}

	return mounts, nil
}

// addWorkspaceMount appends the host project bind mount at /workspace.
func addWorkspaceMount(mounts *[]mount.Mount, hostProjectPath string, readOnly bool) {
	*mounts = append(*mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   hostProjectPath,
		Target:   containerWorkspace,
		ReadOnly: readOnly,
	})
}

// addClaudeMounts appends the ~/.claude directory and ~/.claude.json mounts.
// Skipped when isolatedState is true.
func addClaudeMounts(mounts *[]mount.Mount, home string, isolatedState bool) {
	if isolatedState {
		return
	}

	*mounts = append(*mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: filepath.Join(home, ".claude"),
		Target: containerClaudeDir,
	})

	claudeJSON := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudeJSON); err == nil {
		*mounts = append(*mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: claudeJSON,
			Target: containerClaudeJSON,
		})
	}
}

// addCodexMounts appends the ~/.codex directory mount for Codex CLI state.
// Skipped when isolatedState is true or when ~/.codex does not exist on the host.
func addCodexMounts(mounts *[]mount.Mount, home string, isolatedState bool) {
	if isolatedState {
		return
	}

	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); err != nil {
		return
	}

	*mounts = append(*mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: codexDir,
		Target: containerCodexDir,
	})
}

// addGitMounts appends .gitconfig, .config/gh/, and .ssh/known_hosts mounts
// based on git configuration.
func addGitMounts(mounts *[]mount.Mount, home string, git *config.GitConfig) {
	// ~/.gitconfig (read-only)
	if git.IsMountGitconfig() {
		gitconfig := filepath.Join(home, ".gitconfig")
		if _, err := os.Stat(gitconfig); err == nil {
			*mounts = append(*mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   gitconfig,
				Target:   containerGitconfig,
				ReadOnly: true,
			})
		}
	}

	// ~/.config/gh/ (read-write)
	if git.IsMountGhConfig() {
		ghConfig := filepath.Join(home, ".config", "gh")
		if _, err := os.Stat(ghConfig); err == nil {
			*mounts = append(*mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: ghConfig,
				Target: containerGHConfig,
			})
		}
	}

	// ~/.ssh/known_hosts (read-only, always mounted when present for SSH host key verification)
	knownHosts := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHosts); err == nil {
		*mounts = append(*mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   knownHosts,
			Target:   containerKnownHosts,
			ReadOnly: true,
		})
	}
}

// addSSHMounts appends SSH agent socket, public key, and full ~/.ssh/ mounts
// based on git configuration.
func addSSHMounts(mounts *[]mount.Mount, home string, git *config.GitConfig) {
	// SSH agent socket + public keys (when agent forwarding enabled)
	if git.IsForwardAgent() {
		if runtime.GOOS == "darwin" {
			// On macOS, containers run in a Linux VM; the macOS SSH agent socket is
			// not visible there. Both Docker Desktop and OrbStack relay it at a
			// well-known fixed path — mount that relay unconditionally so the agent
			// is reachable regardless of the host SSH_AUTH_SOCK value.
			*mounts = append(*mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: dockerDesktopRelayPath,
				Target: dockerDesktopRelayPath,
			})
		} else if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			// On Linux, the agent socket is on the same kernel; mount it directly.
			*mounts = append(*mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: sock,
				Target: sock,
			})
		}

		// Mount individual ~/.ssh/*.pub files so ssh-keygen can identify which
		// agent key to use for commit signing without exposing private keys.
		sshDir := filepath.Join(home, ".ssh")
		if entries, err := os.ReadDir(sshDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".pub") {
					src := filepath.Join(sshDir, e.Name())
					*mounts = append(*mounts, mount.Mount{
						Type:     mount.TypeBind,
						Source:   src,
						Target:   containerSSHDir + "/" + e.Name(),
						ReadOnly: true,
					})
				}
			}
		}
	}

	// ~/.ssh/ (read-only, opt-in only)
	if !git.IsMountSSHDir() {
		return
	}
	sshDir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(sshDir); err == nil {
		*mounts = append(*mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   sshDir,
			Target:   containerSSHDir,
			ReadOnly: true,
		})
	}
}

// addPluginMounts appends the plugin directory remount when the host home
// differs from the container home. Skipped when isolatedState is true.
func addPluginMounts(mounts *[]mount.Mount, home string, isolatedState bool) {
	if isolatedState {
		return
	}

	// Plugin path remapping: installed_plugins.json and known_marketplaces.json
	// store absolute host paths (e.g. /Users/alice/.claude/plugins/...). Inside
	// the container the home dir is /home/sandbox, so those paths don't resolve.
	// Mount the entire plugins directory at its original host path (read-only)
	// so Claude Code can find both plugin cache and marketplace data.
	pluginDir := filepath.Join(home, ".claude", "plugins")
	if pluginDir == containerPluginsDir {
		return
	}
	if _, err := os.Stat(pluginDir); err == nil {
		*mounts = append(*mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   pluginDir,
			Target:   pluginDir,
			ReadOnly: true,
		})
	}
}

// addClipboardMount creates the clipboard socket directory on the host and
// appends a bind mount at /run/claustro. No-op when clipboardSockDir is empty.
func addClipboardMount(mounts *[]mount.Mount, clipboardSockDir string) error {
	if clipboardSockDir == "" {
		return nil
	}

	if err := os.MkdirAll(clipboardSockDir, dirModeWorld); err != nil {
		return fmt.Errorf("creating clipboard socket directory: %w", err)
	}
	// Explicitly chmod to override umask — the sandbox user (uid 1000) must be
	// able to traverse this directory even though it is owned by the host uid.
	if err := os.Chmod(clipboardSockDir, dirModeWorld); err != nil {
		return fmt.Errorf("setting clipboard socket directory permissions: %w", err)
	}
	*mounts = append(*mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: clipboardSockDir,
		Target: clipboardRunDir,
	})
	return nil
}
