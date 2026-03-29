package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/wizard"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit claustro.yaml configuration",
		Long:  "Manage claustro project configuration interactively or via get/set.",
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigLanguagesCmd())
	cmd.AddCommand(newConfigToolsCmd())
	cmd.AddCommand(newConfigMCPCmd())
	cmd.AddCommand(newConfigDefaultsCmd())
	cmd.AddCommand(newConfigFirewallCmd())
	cmd.AddCommand(newConfigGitCmd())

	return cmd
}

// newConfigGetCmd returns the "config get <path>" subcommand.
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path>",
		Short: "Print a configuration value using dot-notation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			data, err := readConfigRaw(dir)
			if err != nil {
				return err
			}
			val, err := getNestedValue(data, args[0])
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		},
	}
}

// newConfigSetCmd returns the "config set <path> <value>" subcommand.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> <value>",
		Short: "Set a configuration value using dot-notation",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			data, err := readConfigRaw(dir)
			if err != nil {
				return err
			}
			setNestedValue(data, args[0], parseValue(args[1]))
			out, err := yaml.Marshal(data)
			if err != nil {
				return fmt.Errorf("marshalling config: %w", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), out, 0o644); err != nil {
				return fmt.Errorf("writing claustro.yaml: %w", err)
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

// newConfigLanguagesCmd returns the "config languages" interactive subcommand.
func newConfigLanguagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "languages",
		Short: "Interactively select language runtimes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("languages", func(cfg *config.Config) error {
				selected := []string{}
				if cfg.ImageBuild.IsLanguageEnabled("go") {
					selected = append(selected, "go")
				}
				if cfg.ImageBuild.IsLanguageEnabled("rust") {
					selected = append(selected, "rust")
				}
				if cfg.ImageBuild.IsLanguageEnabled("python") {
					selected = append(selected, "python")
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title("Language runtimes").
							Description("Select the language runtimes to install in the sandbox image.").
							Options(
								huh.NewOption("Go", "go"),
								huh.NewOption("Rust", "rust"),
								huh.NewOption("Python", "python"),
							).
							Value(&selected),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("languages form: %w", err)
				}

				sel := toSet(selected)
				goEnabled := sel["go"]
				rustEnabled := sel["rust"]
				pythonEnabled := sel["python"]
				cfg.ImageBuild.Languages.Go = boolPtr(goEnabled)
				cfg.ImageBuild.Languages.Rust = boolPtr(rustEnabled)
				cfg.ImageBuild.Languages.Python = boolPtr(pythonEnabled)
				return nil
			})
		},
	}
}

// newConfigToolsCmd returns the "config tools" interactive subcommand.
func newConfigToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "Interactively select tool groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("tools", func(cfg *config.Config) error {
				selected := []string{}
				if cfg.ImageBuild.IsToolGroupEnabled("dev") {
					selected = append(selected, "dev")
				}
				if cfg.ImageBuild.IsToolGroupEnabled("build") {
					selected = append(selected, "build")
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title("Tool groups").
							Description("Select the tool groups to install in the sandbox image.").
							Options(
								huh.NewOption("Dev tools", "dev"),
								huh.NewOption("Build tools", "build"),
							).
							Value(&selected),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("tools form: %w", err)
				}

				sel := toSet(selected)
				devEnabled := sel["dev"]
				buildEnabled := sel["build"]
				cfg.ImageBuild.Tools.Dev = boolPtr(devEnabled)
				cfg.ImageBuild.Tools.Build = boolPtr(buildEnabled)
				return nil
			})
		},
	}
}

// newConfigMCPCmd returns the "config mcp" interactive subcommand.
func newConfigMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Interactively select MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("mcp", func(cfg *config.Config) error {
				selected := []string{}
				if cfg.ImageBuild.IsMCPServerEnabled("filesystem") {
					selected = append(selected, "filesystem")
				}
				if cfg.ImageBuild.IsMCPServerEnabled("memory") {
					selected = append(selected, "memory")
				}
				if cfg.ImageBuild.IsMCPServerEnabled("fetch") {
					selected = append(selected, "fetch")
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title("Built-in MCP servers").
							Description("Select the built-in MCP servers to install in the sandbox image.").
							Options(
								huh.NewOption("Filesystem", "filesystem"),
								huh.NewOption("Memory", "memory"),
								huh.NewOption("Fetch", "fetch"),
							).
							Value(&selected),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("mcp form: %w", err)
				}

				sel := toSet(selected)
				fsEnabled := sel["filesystem"]
				memEnabled := sel["memory"]
				fetchEnabled := sel["fetch"]
				cfg.ImageBuild.MCPServers.Filesystem = boolPtr(fsEnabled)
				cfg.ImageBuild.MCPServers.Memory = boolPtr(memEnabled)
				cfg.ImageBuild.MCPServers.Fetch = boolPtr(fetchEnabled)
				return nil
			})
		},
	}
}

// newConfigDefaultsCmd returns the "config defaults" interactive subcommand.
func newConfigDefaultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "defaults",
		Short: "Interactively configure resource defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("defaults", func(cfg *config.Config) error {
				cpus := cfg.Defaults.Resources.CPUs
				memory := cfg.Defaults.Resources.Memory
				readOnly := cfg.Defaults.ReadOnly != nil && *cfg.Defaults.ReadOnly

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("CPUs").
							Description("Number of CPUs to allocate (e.g. 2.0). Leave empty for no limit.").
							Value(&cpus),
						huh.NewInput().
							Title("Memory").
							Description("Memory limit (e.g. 512m, 2g). Leave empty for no limit.").
							Value(&memory),
						huh.NewConfirm().
							Title("Read-only filesystem").
							Description("Mount the project as read-only inside the sandbox.").
							Value(&readOnly),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("defaults form: %w", err)
				}

				cfg.Defaults.Resources.CPUs = cpus
				cfg.Defaults.Resources.Memory = memory
				cfg.Defaults.ReadOnly = boolPtr(readOnly)
				return nil
			})
		},
	}
}

// newConfigFirewallCmd returns the "config firewall" interactive subcommand.
func newConfigFirewallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "firewall",
		Short: "Interactively configure egress firewall settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("firewall", func(cfg *config.Config) error {
				enabled := cfg.Firewall.Enabled != nil && *cfg.Firewall.Enabled

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Enable egress firewall").
							Description("Restrict outbound network access from sandbox containers.").
							Value(&enabled),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("firewall form: %w", err)
				}

				cfg.Firewall.Enabled = boolPtr(enabled)
				return nil
			})
		},
	}
}

// newConfigGitCmd returns the "config git" interactive subcommand.
func newConfigGitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "git",
		Short: "Interactively configure git credential forwarding",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSection("git", func(cfg *config.Config) error {
				forwardAgent := cfg.Git.IsForwardAgent()
				mountGitconfig := cfg.Git.IsMountGitconfig()
				mountGhConfig := cfg.Git.IsMountGhConfig()

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Forward SSH agent").
							Description("Forward the host SSH agent into the sandbox for git operations.").
							Value(&forwardAgent),
						huh.NewConfirm().
							Title("Mount .gitconfig").
							Description("Mount the host ~/.gitconfig into the sandbox.").
							Value(&mountGitconfig),
						huh.NewConfirm().
							Title("Mount GitHub CLI config").
							Description("Mount the host ~/.config/gh into the sandbox.").
							Value(&mountGhConfig),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("git form: %w", err)
				}

				cfg.Git.ForwardAgent = boolPtr(forwardAgent)
				cfg.Git.MountGitconfig = boolPtr(mountGitconfig)
				cfg.Git.MountGhConfig = boolPtr(mountGhConfig)
				return nil
			})
		},
	}
}

// runConfigSection loads config, runs editor, then writes back via wizard.MarshalConfig.
func runConfigSection(section string, editor func(*config.Config) error) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	cfg, err := config.LoadRaw(dir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	if err := editor(cfg); err != nil {
		return err
	}
	data, err := wizard.MarshalConfig(*cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claustro.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("writing claustro.yaml: %w", err)
	}
	fmt.Printf("Updated %s configuration\n", section)
	return nil
}

// readConfigRaw reads claustro.yaml as a raw map for get/set operations.
func readConfigRaw(dir string) (map[string]any, error) {
	path := filepath.Join(dir, "claustro.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading claustro.yaml: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing claustro.yaml: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// getNestedValue traverses a dot-notation path in a map and returns the value.
func getNestedValue(m map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var current any = m
	for _, part := range parts {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q: intermediate value is not a map", path)
		}
		val, exists := cm[part]
		if !exists {
			return nil, fmt.Errorf("path %q: key %q not found", path, part)
		}
		current = val
	}
	return current, nil
}

// setNestedValue sets a value at a dot-notation path, creating intermediate maps as needed.
func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m
	for _, part := range parts[:len(parts)-1] {
		if existing, ok := current[part]; ok {
			if nested, ok := existing.(map[string]any); ok {
				current = nested
			} else {
				// Overwrite non-map with a new map.
				nested := map[string]any{}
				current[part] = nested
				current = nested
			}
		} else {
			nested := map[string]any{}
			current[part] = nested
			current = nested
		}
	}
	current[parts[len(parts)-1]] = value
}

// parseValue converts a string to a typed value. "true"/"false" become bools; otherwise string.
func parseValue(s string) any {
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	// Try integer.
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Try float.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// toSet converts a slice of strings to a set (map[string]bool).
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}
