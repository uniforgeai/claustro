// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

import (
	"fmt"
	"os"
	"os/exec"
)

// EnsureRunning checks the pidfile; if no live daemon, spawns a new one detached.
// claustrodPath is the absolute path to the claustrod binary (typically resolved
// via exec.LookPath at the call site).
func EnsureRunning(claustrodPath string) error {
	if IsAlive() {
		return nil
	}
	cmd := exec.Command(claustrodPath, "run")
	// Detach from parent: own session, no stdio.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = sysprocattrDetach()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting claustrod: %w", err)
	}
	// Release so the child outlives this process.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing claustrod: %w", err)
	}
	return nil
}

// LookupBinary resolves the claustrod binary path.
// Looks first next to the current claustro binary, then in PATH.
func LookupBinary() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		dir := exe[:lastSlash(exe)]
		candidate := dir + "/claustrod"
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return exec.LookPath("claustrod")
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return 0
}
