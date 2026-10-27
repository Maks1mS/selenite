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
	"altlinux.space/alt-atomic/selenite/internal/net"
)

const refreshInterval = 10 * time.Second

type (
	tickMsg struct {
		generation uint64
	}
	dataMsg struct {
		ip           string
		dns          string
		gateway      string
		connectivity string
	}
)

type Model struct {
	ip           string
	dns          string
	gateway      string
	connectivity string
	generation   uint64
}

func New() *Model {
	return &Model{
		ip:           "",
		dns:          "",
		gateway:      "",
		connectivity: "",
	}
}

// Init restarts polling: ticks delivered while the page is hidden are dropped.
func (m *Model) Init() tea.Cmd {
	m.generation++
	return tea.Batch(fetchDataCmd(), m.tickCmd())
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		return m, tea.Batch(fetchDataCmd(), m.tickCmd())
	case base.NetworkUpdateMsg:
		return m, fetchDataCmd()
	case dataMsg:
		m.ip = msg.ip
		m.gateway = msg.gateway
		m.connectivity = msg.connectivity
		m.dns = msg.dns
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
	switch m.connectivity {
	case "OK":
		connectivity = successStyle.Render(m.connectivity)
	case "DOWN":
		connectivity = errorStyle.Render(m.connectivity)
	default:
		connectivity = warningStyle.Render(m.connectivity)
	}

	v := tea.NewView(
		lipgloss.
			NewStyle().
			Padding(1).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					fmt.Sprintf("IP: %s", m.ip),
					fmt.Sprintf("Gateway: %s", m.gateway),
					fmt.Sprintf("DNS: %s", m.dns),
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

func fetchDataCmd() tea.Cmd {
	return func() tea.Msg {
		ips, _ := net.ListRealIPs()
		dns, _ := net.ListDns()
		return dataMsg{
			ip:           strings.Join(ips, ", "),
			dns:          strings.Join(dns, ", "),
			gateway:      net.DefaultGateway(),
			connectivity: net.Connectivity(),
		}
	}
}
