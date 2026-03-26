// Package identity derives sandbox identity from the host project path and name.
package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Identity holds the resolved sandbox identity and all derived Docker resource names.
type Identity struct {
	// Project is the sanitized project slug derived from the CWD directory name.
	Project string
	// Name is the sandbox name (default: "default").
	Name string
	// HostPath is the absolute host project directory path.
	HostPath string
}

// FromCWD derives an Identity from the current working directory and an optional name.
// If name is empty, "default" is used.
func FromCWD(name string) (*Identity, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	return fromPath(cwd, name)
}

func fromPath(path, name string) (*Identity, error) {
	basename := filepath.Base(path)
	slug := slugify(basename)
	if slug == "" {
		return nil, fmt.Errorf("cannot derive project slug from path %q", path)
	}
	if name == "" {
		name = "default"
	}
	return &Identity{
		Project:  slug,
		Name:     name,
		HostPath: path,
	}, nil
}

// slugify lowercases s and replaces runs of non-alphanumeric characters with "-".
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ContainerName returns the Docker container name for this sandbox.
// Format: claustro-{project}_{name}
// The underscore separator ensures unambiguous parsing since project slugs never contain underscores.
func (id *Identity) ContainerName() string {
	return fmt.Sprintf("claustro-%s_%s", id.Project, id.Name)
}

// NetworkName returns the Docker network name for this sandbox.
// Format: claustro-{project}_{name}_net
func (id *Identity) NetworkName() string {
	return fmt.Sprintf("claustro-%s_%s_net", id.Project, id.Name)
}

// NetworkNameFromLabels derives the network name for a container identified by its labels.
func NetworkNameFromLabels(labels map[string]string) string {
	return fmt.Sprintf("claustro-%s_%s_net", labels["claustro.project"], labels["claustro.name"])
}

// Labels returns the Docker labels to apply to all resources for this sandbox.
func (id *Identity) Labels() map[string]string {
	return map[string]string{
		"claustro.managed": "true",
		"claustro.project": id.Project,
		"claustro.name":    id.Name,
	}
}
