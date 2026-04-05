// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check host environment health",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context())
		},
	}
	return cmd
}

func runDoctor(ctx context.Context) error {
	colorEnabled := os.Getenv("NO_COLOR") == ""

	var results []doctor.CheckResult

	// Docker connectivity check.
	dockerResult := doctor.CheckDocker(ctx)
	results = append(results, dockerResult)

	dockerPassed := dockerResult.Status == doctor.Pass

	// Docker socket check — skip if Docker failed.
	if !dockerPassed {
		results = append(results, doctor.CheckResult{
			Name:   "Docker Socket",
			Status: doctor.Skip,
			Detail: "skipped (Docker not available)",
		})
	} else {
		results = append(results, doctor.CheckDockerSocket())
	}

	// Base image check — only if Docker passed.
	if !dockerPassed {
		results = append(results, doctor.CheckResult{
			Name:   "Base Image",
			Status: doctor.Skip,
			Detail: "skipped (Docker not available)",
		})
	} else {
		cli, err := newDockerClient()
		if err != nil {
			results = append(results, doctor.CheckResult{
				Name:    "Base Image",
				Status:  doctor.Fail,
				Detail:  fmt.Sprintf("cannot create client: %v", err),
				FixHint: "Run `claustro rebuild`",
			})
		} else {
			defer cli.Close() //nolint:errcheck
			results = append(results, doctor.CheckBaseImage(ctx, cli))
		}
	}

	// Non-Docker checks always run.
	results = append(results, doctor.CheckGitConfig())
	results = append(results, doctor.CheckSSHAgent())
	results = append(results, doctor.CheckGitHubCLI())
	results = append(results, doctor.CheckClipboard())
	results = append(results, doctor.CheckConfigFile("."))

	// Format output.
	hasFail := false
	passed := 0
	total := 0

	for _, r := range results {
		indicator := statusIndicator(r.Status, colorEnabled)
		fmt.Fprintf(os.Stdout, "  %-16s  %s  %s\n", r.Name, indicator, r.Detail) //nolint:errcheck
		if r.FixHint != "" && (r.Status == doctor.Fail || r.Status == doctor.Warn) {
			fmt.Fprintf(os.Stdout, "                      %s\n", r.FixHint) //nolint:errcheck
		}
		if r.Status == doctor.Skip {
			continue
		}
		total++
		if r.Status == doctor.Pass {
			passed++
		}
		if r.Status == doctor.Fail {
			hasFail = true
		}
	}

	issues := total - passed
	fmt.Fprintf(os.Stdout, "\n%d/%d checks passed. %d issues found.\n", passed, total, issues) //nolint:errcheck

	if hasFail {
		return fmt.Errorf("%d check(s) failed", issues)
	}
	return nil
}

func statusIndicator(s doctor.CheckStatus, color bool) string {
	switch s {
	case doctor.Pass:
		if color {
			return "\033[32m\u2713\033[0m"
		}
		return "\u2713"
	case doctor.Warn:
		if color {
			return "\033[33m\u26a0\033[0m"
		}
		return "\u26a0"
	case doctor.Fail:
		if color {
			return "\033[31m\u2717\033[0m"
		}
		return "\u2717"
	case doctor.Skip:
		return "-"
	default:
		return "?"
	}
}
