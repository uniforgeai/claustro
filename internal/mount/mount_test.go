package mount

import (
	"os"
	"path/filepath"
	"testing"

	dockermount "github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble_basicMounts(t *testing.T) {
	mounts, err := Assemble("/some/project")
	require.NoError(t, err)

	// Must always have at least workspace + .claude
	assert.GreaterOrEqual(t, len(mounts), 2)

	assertMount(t, mounts, "/some/project", "/workspace", dockermount.TypeBind)

	home, _ := os.UserHomeDir()
	assertMount(t, mounts, filepath.Join(home, ".claude"), "/home/sandbox/.claude", dockermount.TypeBind)
}

func TestAssemble_claudeJSONIncludedWhenPresent(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	claudeJSON := filepath.Join(home, ".claude.json")
	exists := fileExists(claudeJSON)

	mounts, err := Assemble("/some/project")
	require.NoError(t, err)

	found := false
	for _, m := range mounts {
		if m.Target == "/home/sandbox/.claude.json" {
			found = true
			assert.Equal(t, claudeJSON, m.Source)
		}
	}
	assert.Equal(t, exists, found, ".claude.json mount presence should match file existence")
}

func TestAssemble_allMountsAreBind(t *testing.T) {
	mounts, err := Assemble("/any/path")
	require.NoError(t, err)
	for _, m := range mounts {
		assert.Equal(t, dockermount.TypeBind, m.Type)
	}
}

func assertMount(t *testing.T, mounts []dockermount.Mount, src, tgt string, typ dockermount.Type) {
	t.Helper()
	for _, m := range mounts {
		if m.Target == tgt {
			assert.Equal(t, src, m.Source)
			assert.Equal(t, typ, m.Type)
			return
		}
	}
	t.Errorf("mount with target %q not found", tgt)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
