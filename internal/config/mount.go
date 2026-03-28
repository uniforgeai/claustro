package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Mount is a parsed bind mount specification.
type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// ParseMount parses a Docker-style "host:container[:mode]" mount string.
// Relative host paths are resolved against projectRoot.
func ParseMount(raw string, projectRoot string) (Mount, error) {
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) < 2 {
		return Mount{}, fmt.Errorf("invalid mount %q: expected host:container[:mode]", raw)
	}

	hostPath := parts[0]
	containerPath := parts[1]

	if !filepath.IsAbs(hostPath) {
		hostPath = filepath.Join(projectRoot, hostPath)
	}

	var readOnly bool
	if len(parts) == 3 {
		switch parts[2] {
		case "ro":
			readOnly = true
		case "rw":
			readOnly = false
		default:
			return Mount{}, fmt.Errorf("invalid mount mode %q in %q: expected ro or rw", parts[2], raw)
		}
	}

	return Mount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      readOnly,
	}, nil
}
