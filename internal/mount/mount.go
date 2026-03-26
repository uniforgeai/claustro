// Package mount assembles Docker bind mounts for a claustro sandbox.
package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/mount"
)

// Assemble returns the bind mounts needed for a sandbox:
//   - hostProjectPath → /workspace  (source, read-write)
//   - ~/.claude       → /home/sandbox/.claude  (Claude state, read-write)
//   - ~/.claude.json  → /home/sandbox/.claude.json  (Claude config, read-write, if exists)
func Assemble(hostProjectPath string) ([]mount.Mount, error) {
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

	return mounts, nil
}
