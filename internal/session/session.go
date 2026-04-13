// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package session discovers and parses Claude Code session files.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session represents a discovered Claude Code session.
type Session struct {
	ID        string
	Title     string
	StartedAt time.Time
	UpdatedAt time.Time
}

// List discovers sessions for the current project. claudeDir is the path to
// the ~/.claude directory (on the host). It looks for JSONL files in
// claudeDir/projects/-workspace/. Returns sessions sorted by UpdatedAt
// descending (most recent first). Returns an empty slice (not an error) if
// the directory does not exist or contains no valid sessions.
func List(claudeDir string) ([]Session, error) {
	dir := filepath.Join(claudeDir, "projects", "-workspace")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session directory %s: %w", dir, err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		s, err := parseSessionFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			slog.Warn("skipping session file", "file", entry.Name(), "err", err)
			continue
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// firstRecord holds the fields we need from the first JSONL line.
type firstRecord struct {
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
}

// titleRecord holds the fields from a custom-title line.
type titleRecord struct {
	Type  string `json:"type"`
	Title string `json:"customTitle"`
}

func parseSessionFile(path string) (Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var s Session
	s.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	s.Title = "(untitled)"

	info, err := f.Stat()
	if err != nil {
		return Session{}, fmt.Errorf("stat %s: %w", path, err)
	}
	s.UpdatedAt = info.ModTime()

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		lineNum++

		if lineNum == 1 {
			var rec firstRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return Session{}, fmt.Errorf("parsing first line: %w", err)
			}
			t, err := time.Parse(time.RFC3339, rec.Timestamp)
			if err != nil {
				return Session{}, fmt.Errorf("parsing timestamp %q: %w", rec.Timestamp, err)
			}
			s.StartedAt = t
			continue
		}

		var rec titleRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Type == "custom-title" && rec.Title != "" {
			s.Title = rec.Title
		}
	}

	if lineNum == 0 {
		return Session{}, fmt.Errorf("empty file")
	}

	return s, nil
}
