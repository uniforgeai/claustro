// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package updater

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// CheckInterval is the minimum time between update checks.
	CheckInterval = 24 * time.Hour
	// GitHubReleaseURL is the API endpoint for the latest release.
	GitHubReleaseURL = "https://api.github.com/repos/uniforgeai/claustro/releases/latest"
)

// ReminderCache stores the last check result.
type ReminderCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

// CachePath returns the path to the reminder cache file.
func CachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "claustro", "update-check.json"), nil
}

// LoadCache reads the cached check result. Returns zero value if not found or invalid.
func LoadCache() ReminderCache {
	path, err := CachePath()
	if err != nil {
		return ReminderCache{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ReminderCache{}
	}
	var cache ReminderCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ReminderCache{}
	}
	return cache
}

// SaveCache writes the check result to disk.
func SaveCache(cache ReminderCache) error {
	path, err := CachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ShouldCheck returns true if enough time has elapsed since the last check.
func ShouldCheck(cache ReminderCache) bool {
	return time.Since(cache.LastCheck) >= CheckInterval
}

// githubRelease represents the minimal GitHub release API response.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// FetchLatestVersion queries GitHub for the latest release tag.
func FetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(GitHubReleaseURL)
	if err != nil {
		return "", fmt.Errorf("checking for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decoding release response: %w", err)
	}
	return release.TagName, nil
}

// NormalizeVersion strips a leading "v" prefix for comparison.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// IsNewer returns true if latest is a different (presumably newer) version than current.
// For "dev" builds, never nag.
func IsNewer(current, latest string) bool {
	if current == "dev" || current == "" {
		return false
	}
	return NormalizeVersion(current) != NormalizeVersion(latest)
}

// CheckAndRemind runs the update check (if due) and returns a reminder message.
// Returns empty string if no update is available or check is not due.
// This is designed to be called in a goroutine so it never blocks.
func CheckAndRemind(currentVersion string) string {
	cache := LoadCache()
	if !ShouldCheck(cache) {
		// Still within the check interval, but we may have a cached result.
		if cache.LatestVersion != "" && IsNewer(currentVersion, cache.LatestVersion) {
			return formatReminder(currentVersion, cache.LatestVersion)
		}
		return ""
	}

	latest, err := FetchLatestVersion()
	if err != nil {
		slog.Debug("update check failed", "error", err)
		return ""
	}

	// Update cache regardless.
	newCache := ReminderCache{
		LastCheck:     time.Now(),
		LatestVersion: latest,
	}
	if err := SaveCache(newCache); err != nil {
		slog.Debug("failed to save update cache", "error", err)
	}

	if IsNewer(currentVersion, latest) {
		return formatReminder(currentVersion, latest)
	}
	return ""
}

func formatReminder(current, latest string) string {
	return fmt.Sprintf("A new version of claustro is available: %s -> %s\nRun `claustro update` to upgrade.", current, latest)
}
