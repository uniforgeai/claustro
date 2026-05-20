// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGoReleaserConfigShipsDaemon(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.yaml")
	require.NoError(t, err)

	var cfg struct {
		Builds []struct {
			ID     string `yaml:"id"`
			Main   string `yaml:"main"`
			Binary string `yaml:"binary"`
		} `yaml:"builds"`
		Archives []struct {
			Builds []string `yaml:"builds"`
		} `yaml:"archives"`
		Brews []struct {
			License string `yaml:"license"`
			Install string `yaml:"install"`
		} `yaml:"brews"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	builds := map[string]string{}
	for _, b := range cfg.Builds {
		builds[b.ID] = b.Main + ":" + b.Binary
	}
	assert.Equal(t, "./cmd/claustro:claustro", builds["claustro"])
	assert.Equal(t, "./cmd/claustrod:claustrod", builds["claustrod"])

	require.NotEmpty(t, cfg.Archives)
	assert.Contains(t, cfg.Archives[0].Builds, "claustro")
	assert.Contains(t, cfg.Archives[0].Builds, "claustrod")

	require.NotEmpty(t, cfg.Brews)
	assert.Equal(t, "BUSL-1.1", cfg.Brews[0].License)
	assert.Contains(t, cfg.Brews[0].Install, `bin.install "claustro"`)
	assert.Contains(t, cfg.Brews[0].Install, `bin.install "claustrod"`)
}
