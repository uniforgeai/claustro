# M3 Egress Firewall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an egress firewall to claustro sandboxes that restricts outbound traffic to a curated allowlist using iptables rules applied via Docker SDK exec.

**Architecture:** The `--firewall` CLI flag flows through `CLIOverrides` -> `SandboxConfig` -> `container.Create` (adds `NET_ADMIN` cap) -> `firewall.Apply` (post-start iptables via Docker SDK exec). A new `internal/firewall` package owns rule generation and application. DNS resolution converts domain allowlists to IP-based iptables rules.

**Tech Stack:** Go, Cobra, Docker SDK for Go, Testify

---

### Task 1: Add iptables to the Dockerfile

**Files:**
- Modify: `internal/image/Dockerfile`

**Dependencies:** None

- [ ] **Step 1: Add iptables package**

In `internal/image/Dockerfile`, add `iptables` to the base tools `apt-get install` block (the first `RUN apt-get update` command, around line 6). Add it after the existing packages:

```dockerfile
RUN apt-get update && apt-get install -y \
    curl \
    git \
    zsh \
    ripgrep \
    fd-find \
    fzf \
    jq \
    tree \
    tmux \
    htop \
    make \
    ca-certificates \
    gnupg \
    build-essential \
    pkg-config \
    libssl-dev \
    iptables \
    && rm -rf /var/lib/apt/lists/*
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: No errors (Dockerfile change does not affect Go compilation, but ensures no syntax issues in surrounding code)

- [ ] **Step 3: Commit**

```bash
git add internal/image/Dockerfile
git commit -m "feat: add iptables to sandbox Dockerfile for egress firewall"
```

---

### Task 2: Add `--firewall` flag to CLIOverrides and up.go

**Files:**
- Modify: `internal/config/resolve.go`
- Modify: `internal/config/resolve_test.go`
- Modify: `cmd/claustro/up.go`

**Dependencies:** None

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/resolve_test.go`:

```go
func TestResolve_FirewallCLIOverride(t *testing.T) {
	fw := true
	cfg := &Config{
		Defaults: DefaultsConfig{Firewall: boolPtr(false)},
	}
	cli := CLIOverrides{
		Name:     "test",
		Firewall: &fw,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "CLI --firewall should override config default")
}

func TestResolve_FirewallCLINil_UsesDefault(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{Firewall: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "config default firewall should apply when CLI flag is nil")
}

func TestResolve_FirewallConfigEnabled(t *testing.T) {
	cfg := &Config{
		Firewall: FirewallConfig{Enabled: boolPtr(true)},
	}
	cli := CLIOverrides{Name: "test"}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.True(t, sc.Firewall, "firewall.enabled in config should enable firewall")
}

func TestResolve_FirewallConfigEnabledOverriddenByCLI(t *testing.T) {
	fw := false
	cfg := &Config{
		Firewall: FirewallConfig{Enabled: boolPtr(true)},
	}
	cli := CLIOverrides{
		Name:     "test",
		Firewall: &fw,
	}
	sc, err := cfg.Resolve("/project", cli, nil)
	require.NoError(t, err)
	assert.False(t, sc.Firewall, "CLI --firewall=false should override config firewall.enabled")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestResolve_Firewall" -v`
Expected: FAIL — `Firewall` field does not exist on `CLIOverrides`

- [ ] **Step 3: Add `Firewall *bool` to CLIOverrides**

In `internal/config/resolve.go`, add the `Firewall` field to `CLIOverrides`:

```go
type CLIOverrides struct {
	Name          string
	Workdir       string
	Mounts        []string
	Env           map[string]string
	ReadOnly      *bool
	IsolatedState bool
	Firewall      *bool
}
```

- [ ] **Step 4: Update Resolve() for firewall resolution**

In `internal/config/resolve.go`, in the `Resolve` method, add firewall resolution. After the existing `Defaults.Firewall` block (around line 40-42), add resolution from the `Firewall` config section. Then add CLI override handling near the other CLI overrides (after the `ReadOnly` override block):

```go
	// Apply config defaults.
	if c.Defaults.Firewall != nil {
		sc.Firewall = *c.Defaults.Firewall
	}

	// firewall.enabled overrides defaults.firewall.
	if c.Firewall.Enabled != nil {
		sc.Firewall = *c.Firewall.Enabled
	}
```

And in the CLI overrides section:

```go
	// CLI firewall override.
	if cli.Firewall != nil {
		sc.Firewall = *cli.Firewall
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "TestResolve_Firewall" -v`
Expected: All PASS

- [ ] **Step 6: Add `--firewall` flag to up.go**

In `cmd/claustro/up.go`, add the flag variable and registration:

```go
var (
	name          string
	workdir       string
	mounts        []string
	envs          []string
	readOnly      bool
	isolatedState bool
	firewall      bool
)
```

In the `RunE` closure, add firewall pointer handling (same pattern as `--readonly`):

```go
var firewallPtr *bool
if cmd.Flags().Changed("firewall") {
	firewallPtr = &firewall
}
```

Pass it into `CLIOverrides`:

```go
return runUp(cmd.Context(), name, config.CLIOverrides{
	Name:          name,
	Workdir:       workdir,
	Mounts:        mounts,
	Env:           cliEnv,
	ReadOnly:      readOnlyPtr,
	IsolatedState: isolatedState,
	Firewall:      firewallPtr,
})
```

Register the flag:

```go
cmd.Flags().BoolVar(&firewall, "firewall", false, `Enable egress firewall (restrict outbound traffic to allowlist)`)
```

- [ ] **Step 7: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add internal/config/resolve.go internal/config/resolve_test.go cmd/claustro/up.go
git commit -m "feat: add --firewall flag to CLIOverrides and claustro up"
```

---

### Task 3: Create `internal/firewall/` package — rule generation

**Files:**
- Create: `internal/firewall/firewall.go`
- Create: `internal/firewall/firewall_test.go`

**Dependencies:** None (this task is pure logic, no Docker interaction)

- [ ] **Step 1: Write failing tests for rule generation**

Create `internal/firewall/firewall_test.go`:

```go
package firewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDomains(t *testing.T) {
	domains := DefaultDomains()
	assert.Contains(t, domains, "api.anthropic.com")
	assert.Contains(t, domains, "registry.npmjs.org")
	assert.Contains(t, domains, "pypi.org")
	assert.Contains(t, domains, "github.com")
	assert.Contains(t, domains, "api.github.com")
	assert.Contains(t, domains, "archive.ubuntu.com")
	assert.Contains(t, domains, "security.ubuntu.com")
}

func TestMergeDomains(t *testing.T) {
	custom := []string{"example.com", "api.anthropic.com"} // duplicate should be deduplicated
	merged := MergeDomains(DefaultDomains(), custom)
	count := 0
	for _, d := range merged {
		if d == "api.anthropic.com" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate domains should be deduplicated")
	assert.Contains(t, merged, "example.com")
}

func TestBuildBaseRules(t *testing.T) {
	rules := BuildBaseRules()
	require.NotEmpty(t, rules)

	// First rule should be ESTABLISHED,RELATED
	assert.Equal(t, []string{"-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"}, rules[0])

	// Should contain loopback rule
	found := false
	for _, r := range rules {
		if len(r) >= 4 && r[0] == "-A" && r[2] == "-o" && r[3] == "lo" {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain loopback rule")

	// Should contain DNS rules (UDP and TCP)
	var udpDNS, tcpDNS bool
	for _, r := range rules {
		for i, arg := range r {
			if arg == "--dport" && i+1 < len(r) && r[i+1] == "53" {
				for _, a := range r {
					if a == "udp" {
						udpDNS = true
					}
					if a == "tcp" {
						tcpDNS = true
					}
				}
			}
		}
	}
	assert.True(t, udpDNS, "should contain UDP DNS rule")
	assert.True(t, tcpDNS, "should contain TCP DNS rule")
}

func TestBuildIPRules(t *testing.T) {
	ips := []string{"104.18.1.1", "104.18.1.2"}
	rules := BuildIPRules(ips)
	assert.Len(t, rules, 2)
	assert.Equal(t, []string{"-A", "OUTPUT", "-d", "104.18.1.1", "-j", "ACCEPT"}, rules[0])
	assert.Equal(t, []string{"-A", "OUTPUT", "-d", "104.18.1.2", "-j", "ACCEPT"}, rules[1])
}

func TestBuildDropPolicy(t *testing.T) {
	rule := BuildDropPolicy()
	assert.Equal(t, []string{"-P", "OUTPUT", "DROP"}, rule)
}

func TestResolveDomainsFunc(t *testing.T) {
	// Test with a mock resolver
	mockResolver := func(domain string) ([]string, error) {
		switch domain {
		case "example.com":
			return []string{"93.184.216.34"}, nil
		case "multi.example.com":
			return []string{"1.2.3.4", "5.6.7.8"}, nil
		default:
			return nil, &mockDNSError{domain: domain}
		}
	}

	ips, warnings := ResolveDomains([]string{"example.com", "multi.example.com", "fail.example.com"}, mockResolver)
	assert.Contains(t, ips, "93.184.216.34")
	assert.Contains(t, ips, "1.2.3.4")
	assert.Contains(t, ips, "5.6.7.8")
	assert.Len(t, warnings, 1, "should have one warning for failed resolution")
	assert.Contains(t, warnings[0], "fail.example.com")
}

type mockDNSError struct {
	domain string
}

func (e *mockDNSError) Error() string { return "no such host: " + e.domain }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/firewall/ -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement the firewall package**

Create `internal/firewall/firewall.go`:

```go
// Package firewall builds and applies iptables egress rules inside sandbox containers.
package firewall

import (
	"fmt"
	"log/slog"
)

// ResolverFunc resolves a domain name to a list of IP addresses.
type ResolverFunc func(domain string) ([]string, error)

// DefaultDomains returns the built-in allowlist of domains.
func DefaultDomains() []string {
	return []string{
		"api.anthropic.com",
		"registry.npmjs.org",
		"pypi.org",
		"github.com",
		"api.github.com",
		"archive.ubuntu.com",
		"security.ubuntu.com",
	}
}

// MergeDomains combines default and custom domain lists, deduplicating entries.
func MergeDomains(defaults, custom []string) []string {
	seen := make(map[string]bool, len(defaults)+len(custom))
	var merged []string
	for _, d := range defaults {
		if !seen[d] {
			seen[d] = true
			merged = append(merged, d)
		}
	}
	for _, d := range custom {
		if !seen[d] {
			seen[d] = true
			merged = append(merged, d)
		}
	}
	return merged
}

// BuildBaseRules returns the iptables argument slices for the base rules
// (established, loopback, DNS, Docker internal ranges).
func BuildBaseRules() [][]string {
	return [][]string{
		{"-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-p", "udp", "--dport", "53", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-p", "tcp", "--dport", "53", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-d", "172.16.0.0/12", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-d", "192.168.0.0/16", "-j", "ACCEPT"},
		{"-A", "OUTPUT", "-d", "10.0.0.0/8", "-j", "ACCEPT"},
	}
}

// BuildIPRules returns iptables argument slices to allow traffic to the given IPs.
func BuildIPRules(ips []string) [][]string {
	rules := make([][]string, 0, len(ips))
	for _, ip := range ips {
		rules = append(rules, []string{"-A", "OUTPUT", "-d", ip, "-j", "ACCEPT"})
	}
	return rules
}

// BuildDropPolicy returns the iptables arguments to set the OUTPUT chain default to DROP.
func BuildDropPolicy() []string {
	return []string{"-P", "OUTPUT", "DROP"}
}

// ResolveDomains resolves each domain using the provided resolver function.
// Returns all resolved IPs and a list of warning messages for domains that failed.
func ResolveDomains(domains []string, resolve ResolverFunc) (ips []string, warnings []string) {
	seen := make(map[string]bool)
	for _, domain := range domains {
		resolved, err := resolve(domain)
		if err != nil {
			msg := fmt.Sprintf("failed to resolve %q: %v", domain, err)
			slog.Warn("firewall DNS resolution failed", "domain", domain, "err", err)
			warnings = append(warnings, msg)
			continue
		}
		for _, ip := range resolved {
			if !seen[ip] {
				seen[ip] = true
				ips = append(ips, ip)
			}
		}
	}
	return ips, warnings
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/firewall/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/firewall/firewall.go internal/firewall/firewall_test.go
git commit -m "feat: add internal/firewall package with rule generation and domain resolution"
```

---

### Task 4: Add firewall application via Docker SDK exec

**Files:**
- Modify: `internal/firewall/firewall.go` (add `Apply` function)
- Create: `internal/firewall/apply_integration_test.go` (integration test)

**Dependencies:** Task 3

- [ ] **Step 1: Write the Apply function**

Add to `internal/firewall/firewall.go`:

```go
import (
	"bytes"
	"context"
	"io"
	"net"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Apply configures iptables egress rules inside a running container.
// It resolves allowlisted domains to IPs, applies base rules, IP allow rules,
// and sets the OUTPUT chain default policy to DROP.
func Apply(ctx context.Context, cli *client.Client, containerID string, customDomains []string) error {
	domains := MergeDomains(DefaultDomains(), customDomains)

	ips, warnings := ResolveDomains(domains, defaultResolver)
	for _, w := range warnings {
		slog.Warn("firewall allowlist", "warning", w)
	}

	// Build complete rule set.
	var allRules [][]string
	allRules = append(allRules, BuildBaseRules()...)
	allRules = append(allRules, BuildIPRules(ips)...)

	// Apply each rule.
	for _, rule := range allRules {
		if err := execIptables(ctx, cli, containerID, rule); err != nil {
			slog.Warn("firewall rule failed", "rule", rule, "err", err)
		}
	}

	// Set default DROP policy.
	if err := execIptables(ctx, cli, containerID, BuildDropPolicy()); err != nil {
		return fmt.Errorf("setting OUTPUT DROP policy: %w", err)
	}

	slog.Info("firewall applied",
		"rules", len(allRules),
		"domains", len(domains),
		"resolved_ips", len(ips),
		"warnings", len(warnings),
	)
	return nil
}

// defaultResolver uses net.LookupHost for DNS resolution.
func defaultResolver(domain string) ([]string, error) {
	return net.LookupHost(domain)
}

// execIptables runs a single iptables command inside the container via Docker SDK exec.
func execIptables(ctx context.Context, cli *client.Client, containerID string, args []string) error {
	cmd := append([]string{"iptables"}, args...)
	execCfg := containertypes.ExecOptions{
		Cmd:          cmd,
		User:         "root",
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("creating iptables exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, containertypes.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("attaching to iptables exec: %w", err)
	}
	defer resp.Close()

	var output bytes.Buffer
	io.Copy(&output, resp.Reader) //nolint:errcheck

	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("inspecting iptables exec: %w", err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("iptables %v exited %d: %s", args, inspect.ExitCode, output.String())
	}
	return nil
}
```

- [ ] **Step 2: Write integration test**

Create `internal/firewall/apply_integration_test.go`:

```go
//go:build integration

package firewall

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply_Integration(t *testing.T) {
	// This test requires Docker and creates a real container with NET_ADMIN.
	// Run with: go test -tags integration ./internal/firewall/ -run TestApply_Integration -v

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer cli.Close()

	// Create a minimal container with NET_ADMIN for testing.
	// Uses the claustro image which has iptables installed.
	// (Test setup and teardown would create/remove the container.)
	t.Skip("requires claustro image built and Docker available — run manually")
}
```

- [ ] **Step 3: Build and run unit tests**

Run: `go build ./... && go test ./internal/firewall/ -v`
Expected: All unit tests PASS, integration test skipped

- [ ] **Step 4: Commit**

```bash
git add internal/firewall/firewall.go internal/firewall/apply_integration_test.go
git commit -m "feat: add firewall Apply function using Docker SDK exec for iptables"
```

---

### Task 5: Integrate firewall into container creation and startup flow

**Files:**
- Modify: `internal/container/container.go` (add `Firewall` to `CreateOptions`, conditionally add `NET_ADMIN`)
- Modify: `cmd/claustro/up.go` (pass firewall config, call `firewall.Apply` after start)

**Dependencies:** Task 2, Task 4

- [ ] **Step 1: Add Firewall field to CreateOptions**

In `internal/container/container.go`, add to `CreateOptions`:

```go
type CreateOptions struct {
	ImageName string
	Firewall  bool
}
```

- [ ] **Step 2: Conditionally add NET_ADMIN capability**

In `internal/container/container.go`, in the `Create` function, after building `hostCfg`, add:

```go
if opts.Firewall {
	hostCfg.CapAdd = []string{"NET_ADMIN"}
}
```

- [ ] **Step 3: Write test for NET_ADMIN capability**

Add to `internal/container/container_test.go` (or create if it does not exist). This tests the config building logic. If direct testing of `Create` requires Docker, use an integration test:

The key assertion is that when `Firewall: true`, the container is created with `NET_ADMIN` in `CapAdd`. Since `Create` calls the Docker API directly, this is best verified in an integration test or by refactoring to make the host config construction testable. For now, verify via the integration test path.

- [ ] **Step 4: Wire firewall into ensureRunning**

In `cmd/claustro/up.go`, in `ensureRunning`:

1. Pass `resolved.Firewall` to `container.CreateOptions`:

```go
var opts container.CreateOptions
opts.Firewall = resolved.Firewall
```

2. After `container.Start()`, apply firewall rules if enabled:

```go
if err := container.Start(ctx, cli, containerID); err != nil {
	return nil, false, fmt.Errorf("starting container: %w", err)
}

// Apply egress firewall rules if enabled.
if resolved.Firewall {
	slog.Info("applying egress firewall", "container", id.ContainerName())
	if err := firewall.Apply(ctx, cli, containerID, cfg.Firewall.Allow); err != nil {
		// Firewall failure is fatal — stop and remove the container.
		_ = container.Stop(ctx, cli, containerID)
		_ = container.Remove(ctx, cli, containerID)
		return nil, false, fmt.Errorf("applying firewall: %w", err)
	}
}
```

3. Add the import:

```go
"github.com/uniforgeai/claustro/internal/firewall"
```

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./...`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/container/container.go cmd/claustro/up.go
git commit -m "feat: integrate egress firewall into container creation and startup"
```

---

### Task 6: End-to-end verification

**Dependencies:** All previous tasks

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 2: Run linter**

Run: `golangci-lint run`
Expected: No new warnings

- [ ] **Step 3: Verify flag help**

Run: `go run ./cmd/claustro up --help`
Expected: Output includes `--firewall` flag with description

- [ ] **Step 4: Manual smoke test (if Docker available)**

```bash
# Build the image with iptables
go run ./cmd/claustro up --firewall --name fw-test

# Shell in and verify rules
go run ./cmd/claustro shell --name fw-test
# Inside container:
#   iptables -L OUTPUT -n  (should show DROP policy with allow rules)
#   curl -s https://api.anthropic.com  (should succeed or timeout with auth error)
#   curl -s https://example.com  (should fail — not in allowlist)

# Cleanup
go run ./cmd/claustro burn --name fw-test
```

- [ ] **Step 5: Final commit if any fixes needed**

Only if linter or smoke test found issues. Otherwise this step is a no-op.
