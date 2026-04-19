// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package daemon implements claustrod, the background process that pauses idle
// claustro sandboxes and exits when none remain.
package daemon

import "time"

// Track is the per-container state retained between polls.
type Track struct {
	LastActive time.Time
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
				newState[c.ID] = prev
			} else {
				newState[c.ID] = Track{LastActive: now}
			}
			continue
		}
		if c.ActiveExecCount > 0 {
			newState[c.ID] = Track{LastActive: now}
			continue
		}
		prev, seen := state[c.ID]
		if !seen {
			newState[c.ID] = Track{LastActive: now}
			continue
		}
		timeout := c.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		if now.Sub(prev.LastActive) >= timeout {
			toPause = append(toPause, c.ID)
		}
		newState[c.ID] = prev
	}
	return toPause, newState
}
