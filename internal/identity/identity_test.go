// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package identity

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adjNounPattern matches the auto-generated adjective_noun format.
var adjNounPattern = regexp.MustCompile(`^[a-z]+_[a-z]+$`)

func Test_slugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-saas", "my-saas"},
		{"My.Project", "my-project"},
		{"MyProject", "myproject"},
		{"my_project", "my-project"},
		{"MY PROJECT", "my-project"},
		{"  spaces  ", "spaces"},
		{"123numeric", "123numeric"},
		{"café", "caf"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, slugify(tt.input))
		})
	}
}

func Test_fromPath(t *testing.T) {
	t.Run("explicit name is used as-is", func(t *testing.T) {
		id, err := fromPath("/Users/pepusz/code/my-saas", "backend")
		require.NoError(t, err)
		assert.Equal(t, "my-saas", id.Project)
		assert.Equal(t, "backend", id.Name)
	})

	t.Run("explicit api name", func(t *testing.T) {
		id, err := fromPath("/home/user/MyApp", "api")
		require.NoError(t, err)
		assert.Equal(t, "myapp", id.Project)
		assert.Equal(t, "api", id.Name)
	})

	t.Run("empty name generates adjective_noun", func(t *testing.T) {
		id, err := fromPath("/Users/pepusz/code/my-saas", "")
		require.NoError(t, err)
		assert.Equal(t, "my-saas", id.Project)
		assert.Regexp(t, adjNounPattern, id.Name, "auto-generated name should match adjective_noun pattern")
	})

	t.Run("empty name from My.Project generates adjective_noun", func(t *testing.T) {
		id, err := fromPath("/home/user/My.Project", "")
		require.NoError(t, err)
		assert.Equal(t, "my-project", id.Project)
		assert.Regexp(t, adjNounPattern, id.Name, "auto-generated name should match adjective_noun pattern")
	})
}

func TestFromCWD(t *testing.T) {
	id, err := FromCWD("")
	require.NoError(t, err)
	assert.Regexp(t, adjNounPattern, id.Name, "auto-generated name should match adjective_noun pattern")
	assert.NotEmpty(t, id.Project)
	assert.NotEmpty(t, id.HostPath)
}

func TestFromCWD_ExplicitName(t *testing.T) {
	id, err := FromCWD("myname")
	require.NoError(t, err)
	assert.Equal(t, "myname", id.Name)
	assert.NotEmpty(t, id.Project)
}

func TestIdentity_ContainerName(t *testing.T) {
	tests := []struct {
		project string
		name    string
		want    string
	}{
		{"my-saas", "default", "claustro-my-saas_default"},
		{"my-saas", "backend", "claustro-my-saas_backend"},
		{"myapp", "api", "claustro-myapp_api"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			id := &Identity{Project: tt.project, Name: tt.name}
			assert.Equal(t, tt.want, id.ContainerName())
		})
	}
}

func TestIdentity_ContainerName_Unambiguous(t *testing.T) {
	// "my-saas" project + "default" name must differ from "my" project + "saas-default" name
	id1 := &Identity{Project: "my-saas", Name: "default"}
	id2 := &Identity{Project: "my", Name: "saas-default"}
	assert.NotEqual(t, id1.ContainerName(), id2.ContainerName())
}

func TestIdentity_NetworkName(t *testing.T) {
	id := &Identity{Project: "my-saas", Name: "default"}
	assert.Equal(t, "claustro-my-saas_default_net", id.NetworkName())
}

func TestNetworkNameFromLabels(t *testing.T) {
	labels := map[string]string{
		"claustro.project": "myproject",
		"claustro.name":    "default",
	}
	assert.Equal(t, "claustro-myproject_default_net", NetworkNameFromLabels(labels))
}

func TestIdentity_VolumeName(t *testing.T) {
	tests := []struct {
		project string
		name    string
		purpose string
		want    string
	}{
		{"myapp", "default", "npm", "claustro-myapp-default-npm"},
		{"myapp", "default", "pip", "claustro-myapp-default-pip"},
		{"myapp", "backend", "npm", "claustro-myapp-backend-npm"},
		{"my-saas", "default", "npm", "claustro-my-saas-default-npm"},
		{"my-saas", "backend", "pip", "claustro-my-saas-backend-pip"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			id := &Identity{Project: tt.project, Name: tt.name}
			assert.Equal(t, tt.want, id.VolumeName(tt.purpose))
		})
	}
}

func TestProjectVolumeName(t *testing.T) {
	tests := []struct {
		project string
		purpose string
		want    string
	}{
		{"myapp", "claude-state", "claustro-myapp-claude-state"},
		{"my-saas", "claude-state", "claustro-my-saas-claude-state"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, ProjectVolumeName(tt.project, tt.purpose))
		})
	}
}

func TestIdentity_MCPContainerName(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "brave-fox"}
	tests := []struct {
		server string
		want   string
	}{
		{"postgres", "claustro-myapp_brave-fox_mcp-postgres"},
		{"browser", "claustro-myapp_brave-fox_mcp-browser"},
	}
	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			assert.Equal(t, tt.want, id.MCPContainerName(tt.server))
		})
	}
}

func TestIdentity_MCPLabels(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "brave-fox"}
	labels := id.MCPLabels("postgres")

	assert.Equal(t, "true", labels["claustro.managed"])
	assert.Equal(t, "myapp", labels["claustro.project"])
	assert.Equal(t, "brave-fox", labels["claustro.name"])
	assert.Equal(t, "mcp-sse", labels["claustro.role"])
	assert.Equal(t, "postgres", labels["claustro.mcp-server"])
}

func TestIdentity_Labels(t *testing.T) {
	id := &Identity{Project: "my-saas", Name: "backend"}
	labels := id.Labels()
	assert.Equal(t, "true", labels["claustro.managed"])
	assert.Equal(t, "my-saas", labels["claustro.project"])
	assert.Equal(t, "backend", labels["claustro.name"])
}

func TestLabels_IncludesHostPath(t *testing.T) {
	id := &Identity{Project: "myapp", Name: "calm_river", HostPath: "/Users/peter/projects/myapp"}
	labels := id.Labels()
	assert.Equal(t, "/Users/peter/projects/myapp", labels[LabelHostPath])
}
