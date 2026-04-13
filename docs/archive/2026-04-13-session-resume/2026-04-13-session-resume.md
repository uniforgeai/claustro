# Session Resume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `claustro claude --resume` to discover past Claude Code sessions and present an interactive TUI picker for easy resume.

**Architecture:** New `internal/session` package reads Claude Code's JSONL session files from `~/.claude/projects/-workspace/`. New `internal/picker` package presents a bubbletea TUI list. `cmd/claustro/claude.go` gains a `--resume` flag that wires the two together.

**Tech Stack:** Go, bubbletea/bubbles/lipgloss (already indirect deps via `huh`), testify.

**Spec:** `docs/specs/2026-04-13-session-resume-design.md`

---

### Task 1: `internal/session` — Session discovery and parsing

**Files:**
- Create: `internal/session/session.go`
- Create: `internal/session/session_test.go`
- Create: `internal/session/testdata/` (fixture JSONL files)

- [x] **Step 1: Create test fixtures**

Create three fixture JSONL files in `internal/session/testdata/projects/-workspace/`:

`internal/session/testdata/projects/-workspace/aaaa1111-2222-3333-4444-555566667777.jsonl`:
```jsonl
{"type":"user","sessionId":"aaaa1111-2222-3333-4444-555566667777","timestamp":"2026-04-10T09:00:00Z","cwd":"/workspace","message":"hello"}
{"type":"assistant","sessionId":"aaaa1111-2222-3333-4444-555566667777","timestamp":"2026-04-10T09:01:00Z","message":"hi"}
{"type":"custom-title","customTitle":"refactoring auth middleware","sessionId":"aaaa1111-2222-3333-4444-555566667777"}
```

`internal/session/testdata/projects/-workspace/bbbb1111-2222-3333-4444-555566667777.jsonl`:
```jsonl
{"type":"user","sessionId":"bbbb1111-2222-3333-4444-555566667777","timestamp":"2026-04-12T14:30:00Z","cwd":"/workspace","message":"fix bug"}
{"type":"assistant","sessionId":"bbbb1111-2222-3333-4444-555566667777","timestamp":"2026-04-12T14:31:00Z","message":"on it"}
```

`internal/session/testdata/projects/-workspace/cccc1111-2222-3333-4444-555566667777.jsonl`:
```jsonl
not-valid-json
```

- [x] **Step 2: Write the failing tests**

```go
// internal/session/session_test.go

// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package session

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	claudeDir := filepath.Join("testdata")

	sessions, err := List(claudeDir)
	require.NoError(t, err)

	// The malformed file (cccc...) should be skipped; we expect 2 sessions.
	require.Len(t, sessions, 2)

	// Sessions are sorted by UpdatedAt descending — bbbb is newer (Apr 12) than aaaa (Apr 10).
	assert.Equal(t, "bbbb1111-2222-3333-4444-555566667777", sessions[0].ID)
	assert.Equal(t, "(untitled)", sessions[0].Title)

	assert.Equal(t, "aaaa1111-2222-3333-4444-555566667777", sessions[1].ID)
	assert.Equal(t, "refactoring auth middleware", sessions[1].Title)
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := List(dir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestList_MissingDir(t *testing.T) {
	sessions, err := List("/nonexistent/path")
	require.NoError(t, err)
	assert.Empty(t, sessions)
}
```

- [x] **Step 3: Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/session/ -v`
Expected: FAIL — package does not exist yet.

- [x] **Step 4: Implement `internal/session/session.go`**

```go
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
```

- [x] **Step 5: Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/session/ -v`
Expected: PASS — all three tests pass.

- [x] **Step 6: Commit**

```bash
git add internal/session/
git commit -m "feat(session): add session discovery and JSONL parsing"
```

---

### Task 2: `internal/picker` — TUI session picker

**Files:**
- Create: `internal/picker/picker.go`
- Create: `internal/picker/picker_test.go`

- [x] **Step 1: Write the failing tests**

```go
// internal/picker/picker_test.go

// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package picker

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniforgeai/claustro/internal/session"
)

func testSessions() []session.Session {
	return []session.Session{
		{
			ID:        "aaaa1111-2222-3333-4444-555566667777",
			Title:     "refactoring auth",
			StartedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:        "bbbb1111-2222-3333-4444-555566667777",
			Title:     "fix volume naming",
			StartedAt: time.Now().Add(-48 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
	}
}

func TestModel_SelectFirst(t *testing.T) {
	m := newModel(testSessions())

	// Press enter to select the first item.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(model)

	require.NotNil(t, cmd)
	assert.Equal(t, "aaaa1111-2222-3333-4444-555566667777", model.selected.ID)
}

func TestModel_SelectSecond(t *testing.T) {
	m := newModel(testSessions())

	// Press down, then enter.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, cmd := updated.(model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(model)

	require.NotNil(t, cmd)
	assert.Equal(t, "bbbb1111-2222-3333-4444-555566667777", model.selected.ID)
}

func TestModel_Cancel(t *testing.T) {
	m := newModel(testSessions())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(model)

	require.NotNil(t, cmd)
	assert.Nil(t, model.selected)
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `cd /workspace && go test ./internal/picker/ -v`
Expected: FAIL — package does not exist yet.

- [x] **Step 3: Implement `internal/picker/picker.go`**

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package picker provides an interactive TUI session picker using bubbletea.
package picker

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/uniforgeai/claustro/internal/session"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	titleStyle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1)
)

type model struct {
	sessions []session.Session
	cursor   int
	selected *session.Session
	quitting bool
}

func newModel(sessions []session.Session) model {
	return model{sessions: sessions}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyShiftTab:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, tea.KeyTab:
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			s := m.sessions[m.cursor]
			m.selected = &s
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEsc:
			m.selected = nil
			m.quitting = true
			return m, tea.Quit
		case tea.KeyRunes:
			if string(msg.Runes) == "q" {
				m.selected = nil
				m.quitting = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Resume a session:"))
	b.WriteString("\n\n")

	for i, s := range m.sessions {
		cursor := "  "
		if i == m.cursor {
			cursor = selectedStyle.Render("▸ ")
		}

		title := s.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		title = fmt.Sprintf("%-40s", title)

		age := relativeTime(s.UpdatedAt)
		age = fmt.Sprintf("%-15s", age)

		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		line := fmt.Sprintf("%s%s %s %s", cursor, title, age, dimStyle.Render(shortID))
		if i == m.cursor {
			line = fmt.Sprintf("%s%s %s %s", cursor, selectedStyle.Render(title), age, dimStyle.Render(shortID))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ navigate • enter select • esc cancel"))
	return b.String()
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}

// PickSession presents an interactive picker and returns the selected session.
// Returns nil if the user cancels (esc/q).
func PickSession(sessions []session.Session) (*session.Session, error) {
	m := newModel(sessions)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running picker: %w", err)
	}
	return finalModel.(model).selected, nil
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `cd /workspace && go test ./internal/picker/ -v`
Expected: PASS — all three tests pass.

- [x] **Step 5: Commit**

```bash
git add internal/picker/
git commit -m "feat(picker): add bubbletea TUI session picker"
```

---

### Task 3: Wire `--resume` flag into `claustro claude`

**Files:**
- Modify: `cmd/claustro/claude.go`

- [x] **Step 1: Add `--resume` flag and session resume logic**

Add the `--resume` flag and the resume flow to `cmd/claustro/claude.go`. The modified file:

```go
// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
	"github.com/uniforgeai/claustro/internal/container"
	"github.com/uniforgeai/claustro/internal/identity"
	"github.com/uniforgeai/claustro/internal/picker"
	"github.com/uniforgeai/claustro/internal/session"
)

func newClaudeCmd() *cobra.Command {
	var name string
	var resume bool
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Launch Claude Code inside a sandbox",
		Long:  "Runs 'claude --dangerously-skip-permissions' inside the sandbox. Automatically starts a sandbox if none is running. Pass extra args after '--'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaude(cmd.Context(), name, resume, args)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", `Sandbox name (default: auto-select if only one running)`)
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume a previous Claude Code session")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runClaude(ctx context.Context, name string, resume bool, extraArgs []string) error {
	nameWasEmpty := name == ""

	id, err := identity.FromCWD(name)
	if err != nil {
		return fmt.Errorf("resolving identity: %w", err)
	}

	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close() //nolint:errcheck

	// If a name was given, look for that specific sandbox.
	// If no name was given, try to auto-select from running sandboxes.
	if nameWasEmpty {
		containers, err := container.ListByProject(ctx, cli, id.Project, false)
		if err != nil {
			return fmt.Errorf("listing sandboxes: %w", err)
		}
		switch len(containers) {
		case 0:
			// No sandbox running — auto-up.
		case 1:
			resolvedName := containers[0].Labels["claustro.name"]
			id, err = identity.FromCWD(resolvedName)
			if err != nil {
				return fmt.Errorf("resolving identity: %w", err)
			}
		default:
			names := make([]string, len(containers))
			for i, c := range containers {
				names[i] = "  " + c.Labels["claustro.name"]
			}
			return fmt.Errorf("multiple sandboxes running, specify --name:\n%s", strings.Join(names, "\n"))
		}
	}

	// Ensure the sandbox is running, creating it if needed.
	id, _, err = ensureRunning(ctx, cli, id, nameWasEmpty, true, config.CLIOverrides{Name: name})
	if err != nil {
		return err
	}

	c, err := container.FindByIdentity(ctx, cli, id)
	if err != nil {
		return fmt.Errorf("finding sandbox: %w", err)
	}
	if c == nil {
		return errNotRunning(id)
	}

	execCmd := []string{"claude", "--dangerously-skip-permissions"}

	if resume {
		sessionID, err := pickSession()
		if err != nil {
			return err
		}
		if sessionID == "" {
			return nil
		}
		execCmd = append(execCmd, "--resume", sessionID)
	}

	execCmd = append(execCmd, extraArgs...)
	sockDir := filepath.Join(os.TempDir(), "claustro-"+id.ContainerName())
	return container.Exec(ctx, cli, c.ID, execCmd, container.ExecOptions{
		Interactive:      true,
		ClipboardSockDir: sockDir,
	})
}

// pickSession discovers sessions and presents the TUI picker.
// Returns the selected session ID, or empty string if cancelled.
func pickSession() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}

	claudeDir := filepath.Join(home, ".claude")
	sessions, err := session.List(claudeDir)
	if err != nil {
		return "", fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found for this project.")
		return "", nil
	}

	selected, err := picker.PickSession(sessions)
	if err != nil {
		return "", fmt.Errorf("session picker: %w", err)
	}
	if selected == nil {
		return "", nil
	}

	return selected.ID, nil
}
```

- [x] **Step 2: Verify the build compiles**

Run: `cd /workspace && go build ./cmd/claustro/`
Expected: BUILD SUCCESS — no errors.

- [x] **Step 3: Run all tests**

Run: `cd /workspace && go test ./...`
Expected: PASS — all tests pass, including existing ones.

- [x] **Step 4: Run linter**

Run: `cd /workspace && golangci-lint run`
Expected: No new warnings.

- [x] **Step 5: Commit**

```bash
git add cmd/claustro/claude.go
git commit -m "feat(claude): add --resume flag with session picker"
```

---

### Task 4: Promote bubbletea to direct dependency

**Files:**
- Modify: `go.mod`

bubbletea, bubbles, and lipgloss are currently indirect dependencies (via `huh`). Since `internal/picker` now imports them directly, they should be listed as direct dependencies.

- [x] **Step 1: Tidy modules**

Run: `cd /workspace && go mod tidy`
Expected: `go.mod` updated — bubbletea, bubbles, and lipgloss move from `// indirect` to direct requires.

- [x] **Step 2: Verify build still works**

Run: `cd /workspace && go build ./cmd/claustro/ && go test ./...`
Expected: BUILD SUCCESS, all tests PASS.

- [x] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: promote bubbletea deps to direct dependencies"
```
