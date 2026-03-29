package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverity_String(t *testing.T) {
	assert.Equal(t, "error", SeverityError.String())
	assert.Equal(t, "warning", SeverityWarning.String())
}

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	results := cfg.Validate()
	assert.Empty(t, results, "empty config should produce no validation results")
}

func TestValidate_CPUs(t *testing.T) {
	tests := []struct {
		name     string
		cpus     string
		wantErr  bool
		wantWarn bool
	}{
		{"empty skips validation", "", false, false},
		{"valid integer", "4", false, false},
		{"valid float", "0.5", false, false},
		{"valid zero warns", "0", false, true},
		{"non-numeric errors", "abc", true, false},
		{"negative errors", "-1", true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Defaults: DefaultsConfig{
					Resources: ResourcesConfig{CPUs: tc.cpus},
				},
			}
			results := cfg.Validate()
			errors := cfg.Errors(results)
			warnings := cfg.Warnings(results)

			if tc.wantErr {
				require.NotEmpty(t, errors, "expected error for cpus=%q", tc.cpus)
				assert.Equal(t, "defaults.resources.cpus", errors[0].Field)
			} else {
				assert.Empty(t, errors, "unexpected error for cpus=%q", tc.cpus)
			}

			if tc.wantWarn {
				require.NotEmpty(t, warnings, "expected warning for cpus=%q", tc.cpus)
				assert.Equal(t, "defaults.resources.cpus", warnings[0].Field)
			} else {
				assert.Empty(t, warnings, "unexpected warning for cpus=%q", tc.cpus)
			}
		})
	}
}

func TestValidate_Memory(t *testing.T) {
	tests := []struct {
		name    string
		memory  string
		wantErr bool
	}{
		{"empty skips", "", false},
		{"valid gigabytes", "8G", false},
		{"valid megabytes", "512M", false},
		{"valid kilobytes", "1024K", false},
		{"lowercase g", "8g", false},
		{"lowercase m", "512m", false},
		{"invalid no unit", "8000", true},
		{"invalid unit", "8GB", true},
		{"invalid letters", "abc", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Defaults: DefaultsConfig{
					Resources: ResourcesConfig{Memory: tc.memory},
				},
			}
			results := cfg.Validate()
			errors := cfg.Errors(results)

			if tc.wantErr {
				require.NotEmpty(t, errors, "expected error for memory=%q", tc.memory)
				assert.Equal(t, "defaults.resources.memory", errors[0].Field)
			} else {
				assert.Empty(t, errors, "unexpected error for memory=%q", tc.memory)
			}
		})
	}
}

func TestValidate_Mounts(t *testing.T) {
	tests := []struct {
		name    string
		mounts  []string
		wantErr bool
	}{
		{"empty list skips", nil, false},
		{"valid two-part", []string{"./host:/container"}, false},
		{"valid with ro", []string{"./host:/container:ro"}, false},
		{"valid with rw", []string{"./host:/container:rw"}, false},
		{"invalid one-part", []string{"/onlyone"}, true},
		{"invalid mode", []string{"./host:/container:bad"}, true},
		{"multiple valid", []string{"./a:/b", "./c:/d:ro"}, false},
		{"one valid one invalid", []string{"./a:/b", "bad"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Sandboxes: map[string]SandboxDef{
					"test": {Mounts: tc.mounts},
				},
			}
			results := cfg.Validate()
			errors := cfg.Errors(results)

			if tc.wantErr {
				require.NotEmpty(t, errors, "expected error for mounts=%v", tc.mounts)
			} else {
				assert.Empty(t, errors, "unexpected error for mounts=%v", tc.mounts)
			}
		})
	}
}

func TestValidate_SandboxNames(t *testing.T) {
	tests := []struct {
		name        string
		sandboxName string
		wantErr     bool
	}{
		{"valid alphanumeric", "api", false},
		{"valid with dash", "my-api", false},
		{"valid with underscore", "my_api", false},
		{"valid mixed", "api-v2_prod", false},
		{"starts with number", "2api", false},
		{"starts with dash errors", "-api", true},
		{"starts with underscore errors", "_api", true},
		{"contains space errors", "my api", true},
		{"contains dot errors", "my.api", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Sandboxes: map[string]SandboxDef{
					tc.sandboxName: {},
				},
			}
			results := cfg.Validate()
			errors := cfg.Errors(results)

			if tc.wantErr {
				require.NotEmpty(t, errors, "expected error for sandbox name=%q", tc.sandboxName)
			} else {
				assert.Empty(t, errors, "unexpected error for sandbox name=%q", tc.sandboxName)
			}
		})
	}
}

func TestValidate_Errors_Warnings_Helpers(t *testing.T) {
	results := []ValidationResult{
		{Field: "f1", Message: "err msg", Severity: SeverityError},
		{Field: "f2", Message: "warn msg", Severity: SeverityWarning},
		{Field: "f3", Message: "err2", Severity: SeverityError},
	}
	cfg := &Config{}

	errors := cfg.Errors(results)
	require.Len(t, errors, 2)
	assert.Equal(t, SeverityError, errors[0].Severity)
	assert.Equal(t, SeverityError, errors[1].Severity)

	warnings := cfg.Warnings(results)
	require.Len(t, warnings, 1)
	assert.Equal(t, SeverityWarning, warnings[0].Severity)
}
