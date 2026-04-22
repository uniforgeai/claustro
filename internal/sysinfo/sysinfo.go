// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package sysinfo reports the host machine's CPU and memory capacity so claustro
// can compute resource caps proportional to the host. Used at `claustro up` time.
package sysinfo

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Host describes the machine claustro is running on. Always non-nil from Detect.
type Host struct {
	CPUs        int   // logical cores visible to the OS
	MemoryBytes int64 // total physical memory
}

// Detect probes the host. Always returns a usable *Host; on any error it logs
// nothing and returns the safe fallback alongside the error so callers may log it.
func Detect() (*Host, error) {
	cpus := runtime.NumCPU()
	if cpus <= 0 {
		return safeFallback(), errors.New("runtime.NumCPU returned non-positive")
	}
	mem, err := detectMemory()
	if err != nil {
		fb := safeFallback()
		fb.CPUs = cpus
		return fb, fmt.Errorf("memory detection failed: %w", err)
	}
	if mem <= 0 {
		fb := safeFallback()
		fb.CPUs = cpus
		return fb, errors.New("memory detection returned non-positive")
	}
	return &Host{CPUs: cpus, MemoryBytes: mem}, nil
}

func safeFallback() *Host {
	return &Host{CPUs: 4, MemoryBytes: 8 * 1024 * 1024 * 1024}
}

func detectMemory() (int64, error) {
	switch runtime.GOOS {
	case "darwin":
		return detectMemoryDarwin()
	case "linux":
		return detectMemoryLinux()
	default:
		return 0, fmt.Errorf("unsupported GOOS: %s", runtime.GOOS)
	}
}

func detectMemoryDarwin() (int64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("invoking sysctl hw.memsize: %w", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing sysctl output: %w", err)
	}
	return n, nil
}

func detectMemoryLinux() (int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("opening /proc/meminfo: %w", err)
	}
	defer f.Close() //nolint:errcheck
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("malformed MemTotal line: %q", line)
		}
		kib, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing MemTotal value %q: %w", fields[1], err)
		}
		return kib * 1024, nil
	}
	return 0, errors.New("MemTotal not found in /proc/meminfo")
}
