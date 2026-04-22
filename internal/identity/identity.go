// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package identity derives sandbox identity from the host project path and name.
package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Label key constants used across packages for Docker container labels.
const (
	LabelProject   = "claustro.project"
	LabelName      = "claustro.name"
	LabelRole      = "claustro.role"
	LabelManaged   = "claustro.managed"
	LabelMCPServer = "claustro.mcp-server"
	LabelHostPath  = "claustro.host_path"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Identity holds the resolved sandbox identity and all derived Docker resource names.
type Identity struct {
	// Project is the sanitized project slug derived from the CWD directory name.
	Project string
	// Name is the sandbox name (default: auto-generated adjective_noun).
	Name string
	// HostPath is the absolute host project directory path.
	HostPath string
}

// FromCWD derives an Identity from the current working directory and an optional name.
// If name is empty, a random adjective_noun name is generated.
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
		name = RandomName()
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
	return fmt.Sprintf("claustro-%s_%s_net", labels[LabelProject], labels[LabelName])
}

// VolumeName returns the Docker volume name for the given purpose.
// Format: claustro-{project}-{name}-{purpose}
func (id *Identity) VolumeName(purpose string) string {
	return fmt.Sprintf("claustro-%s-%s-%s", id.Project, id.Name, purpose)
}

// ProjectVolumeName returns a Docker volume name scoped to the project (not a specific sandbox).
// Format: claustro-{project}-{purpose}
func ProjectVolumeName(project, purpose string) string {
	return fmt.Sprintf("claustro-%s-%s", project, purpose)
}

// MCPContainerName returns the Docker container name for an MCP sibling server.
// Format: claustro-{project}_{name}_mcp-{serverName}
func (id *Identity) MCPContainerName(serverName string) string {
	return fmt.Sprintf("claustro-%s_%s_mcp-%s", id.Project, id.Name, serverName)
}

// MCPLabels returns Docker labels for an MCP sibling container.
// Includes the base sandbox labels plus MCP-specific role and server name.
func (id *Identity) MCPLabels(serverName string) map[string]string {
	labels := id.Labels()
	labels[LabelRole] = "mcp-sse"
	labels[LabelMCPServer] = serverName
	return labels
}

// Labels returns the Docker labels to apply to all resources for this sandbox.
func (id *Identity) Labels() map[string]string {
	return map[string]string{
		LabelManaged:  "true",
		LabelProject:  id.Project,
		LabelName:     id.Name,
		LabelHostPath: id.HostPath,
	}
}
