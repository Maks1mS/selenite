// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary_card

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
)

const refreshInterval = 10 * time.Second

type (
	tickMsg struct {
		generation uint64
	}
	dataMsg Summary
)

type Model struct {
	source     Source
	data       Summary
	generation uint64
}

func New(source Source) *Model {
	return &Model{source: source}
}

// Init restarts polling: ticks delivered while the page is hidden are dropped.
func (m *Model) Init() tea.Cmd {
	m.generation++
	return tea.Batch(m.fetchDataCmd(), m.tickCmd())
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		return m, tea.Batch(m.fetchDataCmd(), m.tickCmd())
	case base.NetworkUpdateMsg:
		return m, m.fetchDataCmd()
	case dataMsg:
		m.data = Summary(msg)
		return m, nil
	}
	return m, nil
}

var (
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Green)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
)

func (m *Model) View() tea.View {
	var connectivity string
	switch m.data.Connectivity {
	case "OK":
		connectivity = successStyle.Render(m.data.Connectivity)
	case "DOWN":
		connectivity = errorStyle.Render(m.data.Connectivity)
	default:
		connectivity = warningStyle.Render(m.data.Connectivity)
	}

	v := tea.NewView(
		lipgloss.
			NewStyle().
			Padding(1).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					fmt.Sprintf("IP: %s", strings.Join(m.data.IPs, ", ")),
					fmt.Sprintf("Gateway: %s", m.data.Gateway),
					fmt.Sprintf("DNS: %s", strings.Join(m.data.DNS, ", ")),
					fmt.Sprintf("Connectivity: %s", connectivity),
				),
			),
	)

	return v
}

func (m *Model) tickCmd() tea.Cmd {
	generation := m.generation
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return tickMsg{generation: generation}
	})
}

func (m *Model) fetchDataCmd() tea.Cmd {
	return func() tea.Msg {
		return dataMsg(m.source.Summary())
	}
}
