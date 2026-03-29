// Package firewall builds and applies iptables egress rules inside sandbox containers.
package firewall

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
