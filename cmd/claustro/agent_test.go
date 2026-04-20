// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/config"
)

func ptrBool(b bool) *bool { return &b }

func TestBuildAgentCmd_OrderingClaude(t *testing.T) {
	got := buildAgentCmd(claudeSpec, []string{"--model", "opus", "fix bug"})
	assert.Equal(t, []string{
		"claude",
		"--dangerously-skip-permissions",
		"--model", "opus", "fix bug",
	}, got)
}

func TestBuildAgentCmd_OrderingCodex(t *testing.T) {
	got := buildAgentCmd(codexSpec, []string{"--model", "o3"})
	assert.Equal(t, []string{
		"codex",
		"--dangerously-bypass-approvals-and-sandbox",
		"--model", "o3",
	}, got)
}

func TestBuildAgentCmd_NoExtraArgs(t *testing.T) {
	got := buildAgentCmd(codexSpec, nil)
	assert.Equal(t, []string{
		"codex",
		"--dangerously-bypass-approvals-and-sandbox",
	}, got)
}

func TestAgentSpec_Claude(t *testing.T) {
	assert.Equal(t, "claude", claudeSpec.Name)
	assert.Equal(t, "", claudeSpec.ConfigKey, "claude has no config gate")
	assert.Contains(t, claudeSpec.DefaultArgs, "--dangerously-skip-permissions")
	assert.NotEmpty(t, claudeSpec.DisplayName)
}

func TestAgentSpec_Codex(t *testing.T) {
	assert.Equal(t, "codex", codexSpec.Name)
	assert.Equal(t, "codex", codexSpec.ConfigKey)
	assert.Contains(t, codexSpec.DefaultArgs, "--dangerously-bypass-approvals-and-sandbox")
	assert.NotEmpty(t, codexSpec.DisplayName)
}

func TestCheckAgentEnabled_NilConfigKeySkipsCheck(t *testing.T) {
	cfg := &config.Config{}
	err := checkAgentEnabled(cfg, claudeSpec) // claudeSpec.ConfigKey == ""
	require.NoError(t, err)
}

func TestCheckAgentEnabled_EnabledByDefault(t *testing.T) {
	cfg := &config.Config{ImageBuild: config.DefaultImageBuildConfig()}
	err := checkAgentEnabled(cfg, codexSpec)
	require.NoError(t, err)
}

func TestCheckAgentEnabled_DisabledReturnsHelpfulError(t *testing.T) {
	cfg := &config.Config{
		ImageBuild: config.ImageBuildConfig{
			Agents: config.AgentsConfig{Codex: ptrBool(false)},
		},
	}
	err := checkAgentEnabled(cfg, codexSpec)
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "codex is disabled")
	assert.Contains(t, msg, "claustro.yaml")
	assert.Contains(t, msg, "image.agents.codex")
	assert.Contains(t, msg, "claustro rebuild")
}
