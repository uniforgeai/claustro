// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxEnv_AlwaysIncludesBase(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	env := sandboxEnv("/some/project")
	assert.Contains(t, env, "CLAUSTRO_HOST_PATH=/some/project")
	assert.Contains(t, env, "HOME=/home/sandbox")
}

func TestSandboxEnv_OpenAIKeyPassedThrough(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")

	env := sandboxEnv("/some/project")
	assert.Contains(t, env, "OPENAI_API_KEY=sk-test-123")
}

func TestSandboxEnv_OpenAIKeyOmittedWhenEmpty(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	env := sandboxEnv("/some/project")
	for _, e := range env {
		assert.NotContains(t, e, "OPENAI_API_KEY",
			"OPENAI_API_KEY should not appear when not set")
	}
}
