// Package doctor implements health checks for the claustro environment.
package doctor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/uniforgeai/claustro/internal/config"
)

// CheckStatus represents the outcome of a health check.
type CheckStatus int

const (
	// Pass indicates the check succeeded.
	Pass CheckStatus = iota
	// Warn indicates a non-critical issue.
	Warn
	// Fail indicates a critical issue.
	Fail
	// Skip indicates the check was skipped.
	Skip
)

// String returns the lowercase name of the check status.
func (s CheckStatus) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	case Skip:
		return "skip"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// CheckResult represents the outcome of a single health check.
type CheckResult struct {
	Name    string
	Status  CheckStatus
	Detail  string
	FixHint string
}

// CheckDocker verifies that the Docker daemon is reachable using the Docker SDK.
func CheckDocker(ctx context.Context) CheckResult {
	name := "Docker Engine"

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Debug("failed to create docker client", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  fmt.Sprintf("cannot create Docker client: %v", err),
			FixHint: "Install or start Docker",
		}
	}
	defer func() { _ = cli.Close() }()

	ping, err := cli.Ping(ctx)
	if err != nil {
		slog.Debug("docker ping failed", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  fmt.Sprintf("cannot reach Docker daemon: %v", err),
			FixHint: "Install or start Docker",
		}
	}

	ver, err := cli.ServerVersion(ctx)
	if err != nil {
		slog.Debug("failed to get docker server version", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Pass,
			Detail:  fmt.Sprintf("Docker reachable, API %s", ping.APIVersion),
			FixHint: "",
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: fmt.Sprintf("Docker %s, API %s", ver.Version, ping.APIVersion),
	}
}

// CheckDockerSocket verifies the Docker socket file is accessible.
func CheckDockerSocket() CheckResult {
	name := "Docker Socket"

	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err == nil {
		return CheckResult{
			Name:   name,
			Status: Pass,
			Detail: socketPath,
		}
	}

	// On macOS, check OrbStack fallback path.
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			orbstackPath := filepath.Join(home, ".orbstack", "run", "docker.sock")
			if _, err := os.Stat(orbstackPath); err == nil {
				return CheckResult{
					Name:   name,
					Status: Pass,
					Detail: orbstackPath,
				}
			}
		}
	}

	return CheckResult{
		Name:    name,
		Status:  Fail,
		Detail:  "Docker socket not found",
		FixHint: "Install or start Docker",
	}
}

// CheckBaseImage verifies that the claustro-base:latest image exists locally.
func CheckBaseImage(ctx context.Context, cli client.APIClient) CheckResult {
	name := "Base Image"

	if cli == nil {
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  "no Docker client available",
			FixHint: "Run `claustro rebuild`",
		}
	}

	inspect, err := cli.ImageInspect(ctx, "claustro-base:latest")
	if err != nil {
		slog.Debug("base image inspect failed", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  "claustro-base:latest not found",
			FixHint: "Run `claustro rebuild`",
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: fmt.Sprintf("claustro-base:latest (created %s)", inspect.Created),
	}
}

// CheckGitConfig verifies that ~/.gitconfig exists.
func CheckGitConfig() CheckResult {
	name := "Git Config"

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("cannot determine home directory", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  fmt.Sprintf("cannot determine home directory: %v", err),
			FixHint: "Run `git config --global user.name ...`",
		}
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	if _, err := os.Stat(gitconfigPath); err != nil {
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "~/.gitconfig not found",
			FixHint: "Run `git config --global user.name ...`",
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: gitconfigPath,
	}
}

// CheckSSHAgent verifies that SSH agent is running and has keys loaded.
func CheckSSHAgent() CheckResult {
	name := "SSH Agent"

	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "SSH_AUTH_SOCK not set",
			FixHint: "Run `ssh-add`",
		}
	}

	out, err := exec.Command("ssh-add", "-l").Output()
	if err != nil {
		slog.Debug("ssh-add -l failed", "error", err)
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "SSH agent has no keys",
			FixHint: "Run `ssh-add`",
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	keyCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			keyCount++
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: fmt.Sprintf("%d key(s) loaded", keyCount),
	}
}

// CheckGitHubCLI verifies that the GitHub CLI is authenticated.
func CheckGitHubCLI() CheckResult {
	name := "GitHub CLI"

	cmd := exec.Command("gh", "auth", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("gh auth status failed", "error", err, "output", string(out))
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "not authenticated",
			FixHint: "Run `gh auth login`",
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: "authenticated",
	}
}

// CheckClipboard verifies that clipboard tools are available on the host.
func CheckClipboard() CheckResult {
	name := "Clipboard"

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pbpaste"); err != nil {
			return CheckResult{
				Name:    name,
				Status:  Warn,
				Detail:  "pbpaste not found",
				FixHint: "pbpaste should be available by default on macOS",
			}
		}
		return CheckResult{
			Name:   name,
			Status: Pass,
			Detail: "pbpaste available",
		}

	case "linux":
		// Check Wayland first.
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-paste"); err == nil {
				return CheckResult{
					Name:   name,
					Status: Pass,
					Detail: "wl-paste available (Wayland)",
				}
			}
		}

		// Check X11.
		if os.Getenv("DISPLAY") != "" {
			if _, err := exec.LookPath("xclip"); err == nil {
				return CheckResult{
					Name:   name,
					Status: Pass,
					Detail: "xclip available (X11)",
				}
			}
		}

		// Neither display server detected or tools missing.
		if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
			return CheckResult{
				Name:    name,
				Status:  Warn,
				Detail:  "no display server detected (DISPLAY and WAYLAND_DISPLAY unset)",
				FixHint: "Set DISPLAY or WAYLAND_DISPLAY, and install xclip or wl-paste",
			}
		}

		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "clipboard tool not found",
			FixHint: "Install xclip (X11) or wl-paste (Wayland)",
		}

	default:
		return CheckResult{
			Name:   name,
			Status: Skip,
			Detail: fmt.Sprintf("unsupported platform: %s", runtime.GOOS),
		}
	}
}

// CheckConfigFile checks if claustro.yaml exists in the given directory and validates it.
func CheckConfigFile(dir string) CheckResult {
	name := "Config File"

	configPath := filepath.Join(dir, "claustro.yaml")
	if _, err := os.Stat(configPath); err != nil {
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  "no claustro.yaml found (optional)",
			FixHint: "run: claustro init",
		}
	}

	cfg, err := config.LoadRaw(dir)
	if err != nil {
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  fmt.Sprintf("failed to parse claustro.yaml: %v", err),
			FixHint: "Fix the YAML syntax errors in claustro.yaml",
		}
	}

	results := cfg.Validate()
	errs := cfg.Errors(results)
	warns := cfg.Warnings(results)

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
		}
		return CheckResult{
			Name:    name,
			Status:  Fail,
			Detail:  strings.Join(msgs, "; "),
			FixHint: "Fix the errors in claustro.yaml",
		}
	}

	if len(warns) > 0 {
		msgs := make([]string, len(warns))
		for i, w := range warns {
			msgs[i] = fmt.Sprintf("%s: %s", w.Field, w.Message)
		}
		return CheckResult{
			Name:    name,
			Status:  Warn,
			Detail:  strings.Join(msgs, "; "),
			FixHint: "Review warnings in claustro.yaml",
		}
	}

	return CheckResult{
		Name:   name,
		Status: Pass,
		Detail: configPath,
	}
}
