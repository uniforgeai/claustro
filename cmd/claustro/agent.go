// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"

	"github.com/uniforgeai/claustro/internal/config"
)

// AgentSpec describes how a coding agent (Claude, Codex, ...) is launched
// inside a sandbox container.
type AgentSpec struct {
	// Name is the binary name invoked inside the container, e.g. "claude" or "codex".
	Name string
	// ConfigKey is the key passed to ImageBuildConfig.IsAgentEnabled to decide
	// whether the agent is installed in the image. Empty string disables the check
	// (used for agents that are unconditionally part of the image, like Claude Code).
	ConfigKey string
	// DefaultArgs are prepended after Name and before any caller-supplied extra args.
	DefaultArgs []string
	// DisplayName is the human-readable name used in error/help messages.
	DisplayName string
}

// claudeSpec launches Claude Code inside the sandbox.
// Claude Code is unconditionally installed in the image and has no config gate.
var claudeSpec = AgentSpec{
	Name:        "claude",
	ConfigKey:   "",
	DefaultArgs: []string{"--dangerously-skip-permissions"},
	DisplayName: "Claude Code",
}

// codexSpec launches OpenAI's Codex CLI inside the sandbox.
// Codex is opt-out via image.agents.codex in claustro.yaml.
// The sandbox container is itself an externally sandboxed environment, so we use
// the bypass flag whose own help text reads "Intended solely for running in
// environments that are externally sandboxed."
var codexSpec = AgentSpec{
	Name:        "codex",
	ConfigKey:   "codex",
	DefaultArgs: []string{"--dangerously-bypass-approvals-and-sandbox"},
	DisplayName: "Codex CLI",
}

// buildAgentCmd composes the in-container command line: [Name, DefaultArgs..., extraArgs...].
func buildAgentCmd(spec AgentSpec, extraArgs []string) []string {
	cmd := make([]string, 0, 1+len(spec.DefaultArgs)+len(extraArgs))
	cmd = append(cmd, spec.Name)
	cmd = append(cmd, spec.DefaultArgs...)
	cmd = append(cmd, extraArgs...)
	return cmd
}

// checkAgentEnabled returns a helpful error when the spec has a ConfigKey and the
// agent is disabled in the project's claustro.yaml. Returns nil when ConfigKey is
// empty (agent has no config gate) or when the agent is enabled.
func checkAgentEnabled(cfg *config.Config, spec AgentSpec) error {
	if spec.ConfigKey == "" {
		return nil
	}
	if cfg.ImageBuild.IsAgentEnabled(spec.ConfigKey) {
		return nil
	}
	return fmt.Errorf(
		"%s is disabled in claustro.yaml (image.agents.%s: false). "+
			"Enable it and run 'claustro rebuild', or run 'claustro shell' to use other tools.",
		spec.Name, spec.ConfigKey,
	)
}
