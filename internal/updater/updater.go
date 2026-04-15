// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package updater

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// Method represents how claustro was installed.
type Method int

const (
	MethodUnknown Method = iota
	MethodHomebrew
	MethodGoInstall
)

// DetectMethod determines how claustro was installed based on the binary path.
func DetectMethod() Method {
	exe, err := os.Executable()
	if err != nil {
		slog.Debug("could not determine executable path", "error", err)
		return MethodUnknown
	}
	slog.Debug("detected executable path", "path", exe)

	if strings.Contains(exe, "/Cellar/") || strings.Contains(exe, "/homebrew/") {
		return MethodHomebrew
	}
	if strings.Contains(exe, "/go/bin/") || strings.Contains(exe, "/gopath/bin/") {
		return MethodGoInstall
	}
	return MethodUnknown
}

// Update performs the update using the detected or provided method.
// It returns the output message and any error.
func Update(method Method, currentVersion string) (string, error) {
	switch method {
	case MethodHomebrew:
		return updateHomebrew()
	case MethodGoInstall:
		return updateGoInstall()
	default:
		return "", fmt.Errorf("cannot auto-update: unknown install method\n\nDownload the latest release from:\n  https://github.com/uniforgeai/claustro/releases")
	}
}

func updateHomebrew() (string, error) {
	slog.Info("updating via Homebrew")

	// Refresh the tap index so Homebrew sees the latest version.
	slog.Info("refreshing Homebrew tap")
	refresh := exec.Command("brew", "update")
	if out, err := refresh.CombinedOutput(); err != nil {
		return "", fmt.Errorf("brew update failed: %w\n%s", err, string(out))
	}

	cmd := exec.Command("brew", "upgrade", "claustro")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("brew upgrade failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func updateGoInstall() (string, error) {
	slog.Info("updating via go install")
	cmd := exec.Command("go", "install", "github.com/uniforgeai/claustro/cmd/claustro@latest")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go install failed: %w\n%s", err, string(out))
	}
	return "Updated via go install", nil
}
