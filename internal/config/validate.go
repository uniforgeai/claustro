package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Severity indicates whether a validation result is a hard error or advisory warning.
type Severity int

const (
	// SeverityError indicates a configuration value that will prevent correct operation.
	SeverityError Severity = iota
	// SeverityWarning indicates a configuration value that is suspicious but allowed.
	SeverityWarning
)

// String returns the lowercase name of the severity level.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// ValidationResult is a single validation finding for a config field.
type ValidationResult struct {
	Field    string
	Message  string
	Severity Severity
}

var (
	memoryPattern      = regexp.MustCompile(`(?i)^\d+[GMK]$`)
	sandboxNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
)

// Validate checks the Config for correctness and returns all findings.
// Empty / zero-value fields are skipped.
func (c *Config) Validate() []ValidationResult {
	var results []ValidationResult
	results = append(results, c.validateResources()...)
	results = append(results, c.validateSandboxes()...)
	results = append(results, c.validateImageBuild()...)
	return results
}

// Errors filters results to only SeverityError entries.
func (c *Config) Errors(results []ValidationResult) []ValidationResult {
	return filterBySeverity(results, SeverityError)
}

// Warnings filters results to only SeverityWarning entries.
func (c *Config) Warnings(results []ValidationResult) []ValidationResult {
	return filterBySeverity(results, SeverityWarning)
}

func filterBySeverity(results []ValidationResult, s Severity) []ValidationResult {
	var out []ValidationResult
	for _, r := range results {
		if r.Severity == s {
			out = append(out, r)
		}
	}
	return out
}

// validateResources checks defaults.resources.cpus and defaults.resources.memory.
func (c *Config) validateResources() []ValidationResult {
	var results []ValidationResult

	cpus := c.Defaults.Resources.CPUs
	if cpus != "" {
		val, err := strconv.ParseFloat(cpus, 64)
		if err != nil {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.cpus",
				Message:  fmt.Sprintf("cpus %q is not a valid number", cpus),
				Severity: SeverityError,
			})
		} else if val == 0 {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.cpus",
				Message:  "cpus is 0; container will inherit host default",
				Severity: SeverityWarning,
			})
		} else if val < 0 {
			results = append(results, ValidationResult{
				Field:    "defaults.resources.cpus",
				Message:  fmt.Sprintf("cpus %q must be a positive number", cpus),
				Severity: SeverityError,
			})
		}
	}

	memory := c.Defaults.Resources.Memory
	if memory != "" && !memoryPattern.MatchString(memory) {
		results = append(results, ValidationResult{
			Field:    "defaults.resources.memory",
			Message:  fmt.Sprintf("memory %q must match pattern <number>[G|M|K] (e.g. 8G, 512M)", memory),
			Severity: SeverityError,
		})
	}

	return results
}

// validateSandboxes checks sandbox names and mount formats.
func (c *Config) validateSandboxes() []ValidationResult {
	var results []ValidationResult

	for name, def := range c.Sandboxes {
		if !sandboxNamePattern.MatchString(name) {
			results = append(results, ValidationResult{
				Field:    fmt.Sprintf("sandboxes.%s", name),
				Message:  fmt.Sprintf("sandbox name %q must match ^[a-zA-Z0-9][a-zA-Z0-9_-]*$", name),
				Severity: SeverityError,
			})
		}

		for i, mount := range def.Mounts {
			field := fmt.Sprintf("sandboxes.%s.mounts[%d]", name, i)
			parts := strings.Split(mount, ":")
			if len(parts) < 2 {
				results = append(results, ValidationResult{
					Field:    field,
					Message:  fmt.Sprintf("mount %q must be in host:container[:ro|rw] format", mount),
					Severity: SeverityError,
				})
				continue
			}
			if len(parts) == 3 {
				mode := parts[2]
				if mode != "ro" && mode != "rw" {
					results = append(results, ValidationResult{
						Field:    field,
						Message:  fmt.Sprintf("mount mode %q must be ro or rw", mode),
						Severity: SeverityError,
					})
				}
			}
		}
	}

	return results
}

// validateImageBuild validates ImageBuild configuration.
// TODO: ImageBuildConfig is not yet defined on Config (planned for Task 3).
// Once ImageBuildConfig is added with Languages.Node, add a check here:
//
//	if c.ImageBuild.Languages.Node == false { ... error ... }
func (c *Config) validateImageBuild() []ValidationResult {
	return nil
}
