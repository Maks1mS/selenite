// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package dashboard

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/menu"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/status"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/network"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/summary"
)

type model struct {
	status *status.Model
	bar    *menu.Model
	active int

	pages []tea.Model

	size tea.WindowSizeMsg
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.status.Init(), m.pages[m.active].Init())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var statusCmd, barCmd tea.Cmd
	m.status, statusCmd = m.status.Update(msg)
	m.bar, barCmd = m.bar.Update(msg)

	switch tmsg := msg.(type) {
	case tea.KeyMsg:
		if tmsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case menu.SelectMsg:
		m.active = tmsg.Index
		var actCmd tea.Cmd
		if m.size.Width > 0 {
			m.pages[m.active], _ = m.pages[m.active].Update(m.size)
		}
		m.pages[m.active], actCmd = m.pages[m.active].Update(base.ActivatedMsg{})
		return m, tea.Batch(statusCmd, barCmd, actCmd)
	case tea.WindowSizeMsg:
		tmsg.Height -= 3
		msg = tmsg
		m.size = tmsg
	}

	var pageCmd tea.Cmd
	m.pages[m.active], pageCmd = m.pages[m.active].Update(msg)
	return m, tea.Batch(statusCmd, barCmd, pageCmd)
}

func (m *model) View() tea.View {
	v := tea.NewView(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.status.View().Content,
			m.pages[m.active].View().Content,
			m.bar.View().Content,
		),
	)

	v.AltScreen = true
	return v
}

func New() model {
	return model{
		status: status.New(),
		pages: []tea.Model{
			summary.New(),
			network.New(),
		},
		bar: menu.New(
			[]menu.Item{
				{Label: "Summary"},
				{Label: "Network"},
			},
		),
	}
}
