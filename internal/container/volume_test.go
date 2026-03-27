package container

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

// TestVolumeFunction_Compile verifies that EnsureVolume and RemoveVolume compile with the
// correct signatures. The actual Docker behavior is tested in integration tests
// (//go:build integration). This test intentionally does not call the functions
// as that would require a running Docker daemon.
func TestVolumeFunction_Compile(t *testing.T) {
	// Type-check: ensure the functions have the expected signatures by assigning
	// them to typed variables. This will fail at compile time if signatures change.
	var _ func(context.Context, *client.Client, string, map[string]string) error = EnsureVolume //nolint:staticcheck
	var _ func(context.Context, *client.Client, string) error = RemoveVolume                   //nolint:staticcheck
}
