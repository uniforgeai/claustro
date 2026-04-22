// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUnpause_PausedReturnsTrue(t *testing.T) {
	assert.True(t, shouldUnpause("paused"))
}

func TestShouldUnpause_RunningReturnsFalse(t *testing.T) {
	assert.False(t, shouldUnpause("running"))
}

func TestShouldUnpause_EmptyReturnsFalse(t *testing.T) {
	assert.False(t, shouldUnpause(""))
}
