// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package firewall

import (
	"fmt"
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

func TestMergeDomains_Empty(t *testing.T) {
	merged := MergeDomains(nil, nil)
	assert.Empty(t, merged)
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

func TestBuildIPRules_Empty(t *testing.T) {
	rules := BuildIPRules(nil)
	assert.Empty(t, rules)
}

func TestBuildDropPolicy(t *testing.T) {
	rule := BuildDropPolicy()
	assert.Equal(t, []string{"-P", "OUTPUT", "DROP"}, rule)
}

func TestResolveDomains(t *testing.T) {
	mockResolver := func(domain string) ([]string, error) {
		switch domain {
		case "example.com":
			return []string{"93.184.216.34"}, nil
		case "multi.example.com":
			return []string{"1.2.3.4", "5.6.7.8"}, nil
		default:
			return nil, fmt.Errorf("no such host: %s", domain)
		}
	}

	ips, warnings := ResolveDomains([]string{"example.com", "multi.example.com", "fail.example.com"}, mockResolver)
	assert.Contains(t, ips, "93.184.216.34")
	assert.Contains(t, ips, "1.2.3.4")
	assert.Contains(t, ips, "5.6.7.8")
	assert.Len(t, warnings, 1, "should have one warning for failed resolution")
	assert.Contains(t, warnings[0], "fail.example.com")
}

func TestResolveDomains_DeduplicatesIPs(t *testing.T) {
	mockResolver := func(domain string) ([]string, error) {
		return []string{"1.2.3.4"}, nil
	}

	ips, warnings := ResolveDomains([]string{"a.com", "b.com"}, mockResolver)
	assert.Len(t, ips, 1, "duplicate IPs should be deduplicated")
	assert.Empty(t, warnings)
}
