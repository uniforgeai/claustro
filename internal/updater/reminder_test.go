// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package updater

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"same version", "v0.1.0", "v0.1.0", false},
		{"newer available", "v0.1.0", "v0.2.0", true},
		{"older than current", "v0.2.0", "v0.1.0", false},
		{"newer patch", "v0.2.0", "v0.2.1", true},
		{"older patch", "v0.2.1", "v0.2.0", false},
		{"newer major", "v0.2.0", "v1.0.0", true},
		{"older major", "v1.0.0", "v0.9.9", false},
		{"dev build", "dev", "v0.2.0", false},
		{"empty current", "", "v0.2.0", false},
		{"with and without v prefix", "0.1.0", "v0.1.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewer(tt.current, tt.latest))
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	assert.Equal(t, "1.2.3", NormalizeVersion("v1.2.3"))
	assert.Equal(t, "1.2.3", NormalizeVersion("1.2.3"))
}

func TestShouldCheck(t *testing.T) {
	t.Run("empty cache should check", func(t *testing.T) {
		assert.True(t, ShouldCheck(ReminderCache{}))
	})
	t.Run("recent check should not check", func(t *testing.T) {
		cache := ReminderCache{LastCheck: time.Now()}
		assert.False(t, ShouldCheck(cache))
	})
	t.Run("old check should check", func(t *testing.T) {
		cache := ReminderCache{LastCheck: time.Now().Add(-25 * time.Hour)}
		assert.True(t, ShouldCheck(cache))
	})
}

func TestLoadCache_MissingFile(t *testing.T) {
	// LoadCache should return zero value when file doesn't exist
	cache := LoadCache()
	assert.True(t, cache.LastCheck.IsZero())
	assert.Empty(t, cache.LatestVersion)
}

func TestSaveAndLoadCache(t *testing.T) {
	// Save to a temp location
	dir := t.TempDir()
	orig := getHome()
	t.Setenv("HOME", dir)
	defer func() { _ = orig }()

	cache := ReminderCache{
		LastCheck:     time.Now().Truncate(time.Second),
		LatestVersion: "v1.0.0",
	}
	err := SaveCache(cache)
	assert.NoError(t, err)

	loaded := LoadCache()
	assert.Equal(t, cache.LatestVersion, loaded.LatestVersion)
	assert.WithinDuration(t, cache.LastCheck, loaded.LastCheck, time.Second)
}

func TestFormatReminder(t *testing.T) {
	msg := formatReminder("v0.1.0", "v0.2.0")
	assert.Contains(t, msg, "v0.1.0")
	assert.Contains(t, msg, "v0.2.0")
	assert.Contains(t, msg, "claustro update")
}

func getHome() string {
	home, _ := os.UserHomeDir()
	return home
}
