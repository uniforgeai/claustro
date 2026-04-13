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
