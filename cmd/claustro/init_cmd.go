// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/wizard"
)

func newInitCmd() *cobra.Command {
	var (
		flagProject   string
		flagLanguages string
		flagTools     string
		flagMCP       string
		flagCPUs      string
		flagMemory    string
		flagFirewall  bool
		flagReadOnly  bool
		flagYes       bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a claustro.yaml config in the current directory",
		Long:  "Run the interactive wizard (or apply flags directly) to create a claustro.yaml configuration file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitFlow(cmd, initFlags{
				project:   flagProject,
				languages: flagLanguages,
				tools:     flagTools,
				mcp:       flagMCP,
				cpus:      flagCPUs,
				memory:    flagMemory,
				firewall:  flagFirewall,
				readOnly:  flagReadOnly,
				yes:       flagYes,
			})
		},
	}

	cmd.Flags().StringVar(&flagProject, "project", "", "Project name (default: directory basename)")
	cmd.Flags().StringVar(&flagLanguages, "languages", "", "Comma-separated language runtimes to enable (go,rust,python)")
	cmd.Flags().StringVar(&flagTools, "tools", "", "Comma-separated tool groups to enable (dev,build)")
	cmd.Flags().StringVar(&flagMCP, "mcp", "", "Comma-separated MCP servers to enable (filesystem,memory,fetch)")
	cmd.Flags().StringVar(&flagCPUs, "cpus", "", "Number of CPUs (e.g. 4)")
	cmd.Flags().StringVar(&flagMemory, "memory", "", "Memory limit (e.g. 8G)")
	cmd.Flags().BoolVar(&flagFirewall, "firewall", false, "Enable egress firewall")
	cmd.Flags().BoolVar(&flagReadOnly, "readonly", false, "Mount source as read-only")
	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "Non-interactive: apply defaults and flag overrides without prompting")

	return cmd
}

// initFlags holds the parsed flag values for the init command.
type initFlags struct {
	project   string
	languages string
	tools     string
	mcp       string
	cpus      string
	memory    string
	firewall  bool
	readOnly  bool
	yes       bool
}

// runInitFlow orchestrates the init wizard: checks for existing config, applies flag
// overrides, optionally runs the interactive wizard, then writes claustro.yaml.
func runInitFlow(cmd *cobra.Command, flags initFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfgPath := filepath.Join(cwd, "claustro.yaml")

	// Check if claustro.yaml already exists.
	if _, err := os.Stat(cfgPath); err == nil {
		if !flags.yes {
			var overwrite bool
			confirmForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("claustro.yaml already exists. Overwrite?").
						Value(&overwrite),
				),
			)
			if runErr := confirmForm.Run(); runErr != nil {
				return fmt.Errorf("prompt error: %w", runErr)
			}
			if !overwrite {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	// Build default options from directory basename.
	project := filepath.Base(cwd)
	opts := wizard.DefaultOptions(project)

	// Apply flag overrides (only if the flag was explicitly set).
	if cmd.Flags().Changed("project") {
		opts.Project = flags.project
	}
	if cmd.Flags().Changed("languages") {
		opts.Languages = splitCSV(flags.languages)
	}
	if cmd.Flags().Changed("tools") {
		opts.Tools = splitCSV(flags.tools)
	}
	if cmd.Flags().Changed("mcp") {
		opts.MCPServers = splitCSV(flags.mcp)
	}
	if cmd.Flags().Changed("cpus") {
		opts.CPUs = flags.cpus
	}
	if cmd.Flags().Changed("memory") {
		opts.Memory = flags.memory
	}
	if cmd.Flags().Changed("firewall") {
		opts.Firewall = flags.firewall
	}
	if cmd.Flags().Changed("readonly") {
		opts.ReadOnly = flags.readOnly
	}

	// Run interactive wizard if not in --yes mode.
	if !flags.yes {
		wizardErr := runWizard(&opts)
		if wizardErr != nil {
			return fmt.Errorf("wizard error: %w", wizardErr)
		}
	}

	// Build and marshal config.
	cfg := wizard.BuildConfig(opts)
	data, err := wizard.MarshalConfig(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, configFileMode); err != nil {
		return fmt.Errorf("writing claustro.yaml: %w", err)
	}

	fmt.Printf("Created %s\n", cfgPath)
	fmt.Printf("  Project: %s\n", opts.Project)
	fmt.Printf("  Run: claustro up\n")
	return nil
}

// runWizard runs the interactive huh wizard and updates opts in place.
func runWizard(opts *wizard.Options) error {
	// Collect language selections as multi-select options.
	langValues := opts.Languages

	// Collect tool selections.
	toolValues := opts.Tools

	// Collect MCP server selections.
	mcpValues := opts.MCPServers

	// Git settings.
	forwardAgent := opts.ForwardAgent
	mountGitconfig := opts.MountGitconfig
	mountGhConfig := opts.MountGhConfig

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Value(&opts.Project),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Languages (Node is always enabled)").
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("Rust", "rust"),
					huh.NewOption("Python", "python"),
				).
				Value(&langValues),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Tool groups").
				Options(
					huh.NewOption("Dev tools (git, curl, jq, etc.)", "dev"),
					huh.NewOption("Build tools (make, cmake, etc.)", "build"),
				).
				Value(&toolValues),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("MCP servers").
				Options(
					huh.NewOption("Filesystem", "filesystem"),
					huh.NewOption("Memory", "memory"),
					huh.NewOption("Fetch", "fetch"),
				).
				Value(&mcpValues),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("CPUs").
				Value(&opts.CPUs),
			huh.NewInput().
				Title("Memory").
				Value(&opts.Memory),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable egress firewall?").
				Value(&opts.Firewall),
			huh.NewConfirm().
				Title("Mount source as read-only?").
				Value(&opts.ReadOnly),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Forward SSH agent into sandbox?").
				Value(&forwardAgent),
			huh.NewConfirm().
				Title("Mount ~/.gitconfig into sandbox?").
				Value(&mountGitconfig),
			huh.NewConfirm().
				Title("Mount ~/.config/gh into sandbox?").
				Value(&mountGhConfig),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	opts.Languages = langValues
	opts.Tools = toolValues
	opts.MCPServers = mcpValues
	opts.ForwardAgent = forwardAgent
	opts.MountGitconfig = mountGitconfig
	opts.MountGhConfig = mountGhConfig

	return nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

