// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package daemon implements claustrod, the background process that pauses idle
// claustro sandboxes and exits when none remain.
package daemon

import "time"

// Track is the per-container state retained between polls.
type Track struct {
	// LastActive records when the container was last seen with a running exec
	// session (or when it first entered state-tracking). A container that has
	// been idle for longer than the effective timeout is a pause candidate.
	LastActive time.Time
	// PrevState records the container state observed on the previous tick.
	// Used to detect paused→running transitions so we can grant a fresh grace
	// period instead of immediately re-pausing an externally-unpaused container.
	PrevState string
}

// ContainerView is the daemon's runtime input per container. The poll loop
// converts Docker SDK types into this minimal shape so Decide stays pure.
type ContainerView struct {
	ID              string
	State           string        // "running", "paused", etc. (Container.State.Status)
	ActiveExecCount int           // count of running exec sessions
	Timeout         time.Duration // per-project effective timeout; 0 = use default
}

// Decide returns the container IDs to pause and the next state map.
// Caller (the poll loop) is responsible for actually invoking ContainerPause and
// for filtering out containers whose project has disabled pause.
func Decide(state map[string]Track, containers []ContainerView, now time.Time, defaultTimeout time.Duration) (toPause []string, newState map[string]Track) {
	newState = make(map[string]Track, len(containers))
	for _, c := range containers {
		if c.State == "paused" {
			if prev, ok := state[c.ID]; ok {
				prev.PrevState = c.State
				newState[c.ID] = prev
			} else {
				newState[c.ID] = Track{LastActive: now, PrevState: c.State}
			}
			continue
		}
		// If we just observed a paused→running transition (e.g. user ran
		// `docker unpause` manually, or attach auto-resumed), refresh the
		// timer so the container isn't re-paused on the very next tick.
		if prev, seen := state[c.ID]; seen && prev.PrevState == "paused" {
			newState[c.ID] = Track{LastActive: now, PrevState: c.State}
			continue
		}
		if c.ActiveExecCount > 0 {
			newState[c.ID] = Track{LastActive: now, PrevState: c.State}
			continue
		}
		prev, seen := state[c.ID]
		if !seen {
			newState[c.ID] = Track{LastActive: now, PrevState: c.State}
			continue
		}
		timeout := c.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		if now.Sub(prev.LastActive) >= timeout {
			toPause = append(toPause, c.ID)
		}
		newState[c.ID] = Track{LastActive: prev.LastActive, PrevState: c.State}
	}
	return toPause, newState
}
