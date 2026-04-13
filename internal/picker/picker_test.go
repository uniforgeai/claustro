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
