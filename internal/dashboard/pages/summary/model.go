// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
)

type page struct {
	width, height int
}

func NewSummaryPage() *page {
	return &page{}
}

func (m *page) Init() tea.Cmd {
	return nil
}

func (m *page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case base.ActivatedMsg:
		return m, nil
	}
	var cmd tea.Cmd
	return m, cmd
}

func (m *page) View() tea.View {
	v := tea.NewView(
		lipgloss.NewStyle().
			Width(m.width).
			Height(m.height + 1).
			MaxHeight(m.height + 1).
			Border(lipgloss.RoundedBorder()).
			Align(lipgloss.Center).
			Render("Summary not implemented yet :("),
	)
	return v
}

func New() *page {
	return &page{}
}
