// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/journal"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/summary_card"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
)

type page struct {
	width, height int

	summaryCard *summary_card.Model
	journal     *journal.Model
}

func (m *page) Init() tea.Cmd {
	return tea.Batch(
		m.summaryCard.Init(),
		m.journal.Init(),
	)
}

func (m *page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(0, msg.Width)
		m.height = max(0, msg.Height)
	case base.ActivatedMsg:
		return m, tea.Batch(m.summaryCard.Init(), m.journal.Init())
	}
	var summaryCmd, journalCmd tea.Cmd
	m.summaryCard, summaryCmd = m.summaryCard.Update(msg)

	m.journal.SetSize(
		max(0, m.width-2),
		max(0, m.height-1-lipgloss.Height(m.summaryView())),
	)

	if _, resized := msg.(tea.WindowSizeMsg); !resized {
		m.journal, journalCmd = m.journal.Update(msg)
	}

	return m, tea.Batch(summaryCmd, journalCmd)
}

func (m *page) summaryView() string {
	return lipgloss.NewStyle().
		Align(lipgloss.Left).
		Width(max(0, m.width-2)).
		MaxWidth(max(0, m.width-2)).
		BorderBottom(true).
		BorderStyle(lipgloss.RoundedBorder()).
		Render(m.summaryCard.View().Content)
}

func (m *page) View() tea.View {
	journalView := m.journal.View()
	v := tea.NewView("")
	v.MouseMode = journalView.MouseMode

	if m.width == 0 || m.height == 0 {
		return v
	}

	height := m.height + 1
	inner := ""
	if m.width > 2 && height > 2 {
		inner = lipgloss.JoinVertical(lipgloss.Left, m.summaryView(), journalView.Content)
		inner = lipgloss.NewStyle().
			MaxWidth(m.width - 2).
			MaxHeight(height - 2).
			Render(inner)
	}

	v.Content = lipgloss.NewStyle().
		Width(m.width).
		Height(height).
		MaxWidth(m.width).
		MaxHeight(height).
		Border(lipgloss.RoundedBorder()).
		Render(inner)

	return v
}

func New() *page {
	return &page{
		summaryCard: summary_card.New(),
		journal:     journal.New(),
	}
}
