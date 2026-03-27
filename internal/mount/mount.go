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

// dockerDesktopRelayPath is the SSH agent socket that Docker Desktop on macOS
// exposes inside containers. The host SSH agent socket lives on the macOS side and
// cannot be reached from the Linux VM; Docker Desktop relays it at this fixed path.
const dockerDesktopRelayPath = "/run/host-services/ssh-auth.sock"

// SSHAgentContainerSock returns the SSH_AUTH_SOCK value to set inside the container.
// On macOS, Docker containers run in a Linux VM and cannot access the macOS-side SSH
// agent socket directly. Docker Desktop relays it at a well-known fixed path instead.
// On Linux, the host socket path is used unchanged.
func SSHAgentContainerSock(hostSock string) string {
	if runtime.GOOS == "darwin" {
		return dockerDesktopRelayPath
	}
	return hostSock
}

// Assemble returns the bind mounts needed for a sandbox:
//   - hostProjectPath      → /workspace  (source, read-write)
//   - ~/.claude            → /home/sandbox/.claude  (Claude state, read-write)
//   - ~/.claude.json       → /home/sandbox/.claude.json  (Claude config, read-write, if exists)
//
// Git/GitHub mounts are added conditionally based on git config:
//   - ~/.gitconfig         → /home/sandbox/.gitconfig  (read-only, if exists and enabled)
//   - ~/.config/gh/        → /home/sandbox/.config/gh/  (read-write, if exists and enabled)
//   - ~/.ssh/known_hosts   → /home/sandbox/.ssh/known_hosts  (read-only, if exists)
//   - SSH agent socket     → container path via SSHAgentContainerSock  (if agent forwarding enabled)
//   - ~/.ssh/*.pub         → /home/sandbox/.ssh/*.pub  (read-only, if agent forwarding enabled)
//   - ~/.ssh/              → /home/sandbox/.ssh/  (read-only, if exists and explicitly enabled)
//
// clipboardSockDir, when non-empty, is created on the host and mounted at /run/claustro
// inside the container so the clipboard bridge socket is accessible to shim scripts.
func Assemble(hostProjectPath string, git *config.GitConfig, clipboardSockDir string) ([]mount.Mount, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: hostProjectPath,
			Target: "/workspace",
		},
		{
			Type:   mount.TypeBind,
			Source: filepath.Join(home, ".claude"),
			Target: "/home/sandbox/.claude",
		},
	}

	claudeJSON := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudeJSON); err == nil {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: claudeJSON,
			Target: "/home/sandbox/.claude.json",
		})
	}

	if git == nil {
		git = &config.GitConfig{}
	}

	// ~/.gitconfig (read-only)
	if git.IsMountGitconfig() {
		gitconfig := filepath.Join(home, ".gitconfig")
		if _, err := os.Stat(gitconfig); err == nil {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   gitconfig,
				Target:   "/home/sandbox/.gitconfig",
				ReadOnly: true,
			})
		}
	}

	// ~/.config/gh/ (read-write)
	if git.IsMountGhConfig() {
		ghConfig := filepath.Join(home, ".config", "gh")
		if _, err := os.Stat(ghConfig); err == nil {
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: ghConfig,
				Target: "/home/sandbox/.config/gh",
			})
		}
	}

	// ~/.ssh/known_hosts (read-only, always mounted when present for SSH host key verification)
	knownHosts := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHosts); err == nil {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   knownHosts,
			Target:   "/home/sandbox/.ssh/known_hosts",
			ReadOnly: true,
		})
	}

	// SSH agent socket + public keys (when agent forwarding enabled)
	if git.IsForwardAgent() {
		if runtime.GOOS == "darwin" {
			// On macOS, Docker Desktop containers run in a Linux VM; the macOS SSH
			// agent socket is not visible there. Docker Desktop relays it at a
			// well-known path — mount that relay unconditionally so the agent is
			// reachable regardless of the host SSH_AUTH_SOCK value.
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: dockerDesktopRelayPath,
				Target: dockerDesktopRelayPath,
			})
		} else if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			// On Linux, the agent socket is on the same kernel; mount it directly.
			mounts = append(mounts, mount.Mount{
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
					mounts = append(mounts, mount.Mount{
						Type:     mount.TypeBind,
						Source:   src,
						Target:   "/home/sandbox/.ssh/" + e.Name(),
						ReadOnly: true,
					})
				}
			}
		}
	}

	// ~/.ssh/ (read-only, opt-in only)
	if git.IsMountSSHDir() {
		sshDir := filepath.Join(home, ".ssh")
		if _, err := os.Stat(sshDir); err == nil {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   sshDir,
				Target:   "/home/sandbox/.ssh",
				ReadOnly: true,
			})
		}
	}

	// Clipboard bridge socket directory
	if clipboardSockDir != "" {
		if err := os.MkdirAll(clipboardSockDir, 0o700); err != nil {
			return nil, fmt.Errorf("creating clipboard socket directory: %w", err)
		}
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: clipboardSockDir,
			Target: "/run/claustro",
		})
	}

	return mounts, nil
}
