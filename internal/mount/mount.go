// Package mount assembles Docker bind mounts for a claustro sandbox.
package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/mount"
	"github.com/uniforgeai/claustro/internal/config"
)

// Assemble returns the bind mounts needed for a sandbox:
//   - hostProjectPath → /workspace  (source, read-write)
//   - ~/.claude       → /home/sandbox/.claude  (Claude state, read-write)
//   - ~/.claude.json  → /home/sandbox/.claude.json  (Claude config, read-write, if exists)
//
// Git/GitHub mounts are added conditionally based on git config:
//   - ~/.gitconfig    → /home/sandbox/.gitconfig  (read-only, if exists and enabled)
//   - ~/.config/gh/   → /home/sandbox/.config/gh/  (read-write, if exists and enabled)
//   - $SSH_AUTH_SOCK  → same path inside container  (if set and agent forwarding enabled)
//   - ~/.ssh/         → /home/sandbox/.ssh/  (read-only, if exists and explicitly enabled)
func Assemble(hostProjectPath string, git *config.GitConfig) ([]mount.Mount, error) {
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

	// SSH agent socket
	if git.IsForwardAgent() {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			mounts = append(mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: sock,
				Target: sock,
			})
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

	return mounts, nil
}
