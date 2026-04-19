// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

func TestDecide_NewlyObservedContainerGetsGracePeriod(t *testing.T) {
	state := map[string]Track{}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, newState := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
	assert.Equal(t, t0, newState["c1"].LastActive)
}

func TestDecide_ActiveExecResetsTimer(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-10 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 2, Timeout: 5 * time.Minute}}
	toPause, newState := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
	assert.Equal(t, t0, newState["c1"].LastActive)
}

func TestDecide_IdlePastTimeoutPauses(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_AlreadyPausedSkipped(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-10 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "paused", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Empty(t, toPause)
}

func TestDecide_PerContainerTimeoutHonored(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-2 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 1 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_FallsBackToDefaultTimeoutWhenZero(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 0}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_PauseDisabledViaTimeoutSentinel(t *testing.T) {
	state := map[string]Track{"c1": {LastActive: t0.Add(-6 * time.Minute)}}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	toPause, _ := Decide(state, containers, t0, 5*time.Minute)
	assert.Equal(t, []string{"c1"}, toPause)
}

func TestDecide_RemovedContainersDroppedFromState(t *testing.T) {
	state := map[string]Track{
		"c1": {LastActive: t0.Add(-1 * time.Minute)},
		"c2": {LastActive: t0.Add(-1 * time.Minute)},
	}
	containers := []ContainerView{{ID: "c1", State: "running", ActiveExecCount: 0, Timeout: 5 * time.Minute}}
	_, newState := Decide(state, containers, t0, 5*time.Minute)
	_, ok := newState["c2"]
	assert.False(t, ok, "c2 should be dropped from state when no longer listed")
}
