// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package dashboard_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/dashboard"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/journal"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/status"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/components/summary_card"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/dashtest"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/mocks"
	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/network"
	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

const gib = 1024 * 1024 * 1024

// The size of a 1280x800 console with an 8x16 font.
const width, height = 160, 50

const currentConfig = `network:
  desired_state:
    interfaces:
    - name: eth0
      type: ethernet
      state: up
      ipv4:
        enabled: true
        dhcp: true
`

type sources struct {
	dashboard.Sources

	status  *mocks.MockStatusSource
	summary *mocks.MockSummarySource
	journal *mocks.MockJournalReader
	network *mocks.MockNetworkBackend
}

// newSources returns fakes describing a small VM with two network interfaces.
func newSources(t *testing.T) sources {
	t.Helper()

	s := sources{
		status:  mocks.NewMockStatusSource(t),
		summary: mocks.NewMockSummarySource(t),
		journal: mocks.NewMockJournalReader(t),
		network: mocks.NewMockNetworkBackend(t),
	}
	s.Sources = dashboard.Sources{Status: s.status, Summary: s.summary, Journal: s.journal, Network: s.network}

	s.status.EXPECT().CPUInfo().Return(status.CPUInfo{Cores: 2, FreqGHz: 2.4}).Maybe()
	s.status.EXPECT().Hostname().Return("selenite").Maybe()
	s.status.EXPECT().Stats().Return(status.Stats{
		CPUPercent: 12.5,
		MemPercent: 25,
		MemTotal:   2 * gib,
		MemUsed:    gib / 2,
		SwapTotal:  gib,
		Uptime:     3*time.Hour + 25*time.Minute,
		Hostname:   "selenite",
	}).Maybe()

	s.summary.EXPECT().Summary().Return(summary_card.Summary{
		IPs:          []string{"10.0.2.15"},
		DNS:          []string{"10.0.2.3"},
		Gateway:      "10.0.2.2",
		Connectivity: "OK",
	}).Maybe()

	records := make([]journal.Record, 30)
	for i := range records {
		records[i] = journal.Record{
			Cursor: fmt.Sprint(i),
			Text:   fmt.Sprintf("Oct 27 12:00:%02d systemd[1]: Started unit-%02d.service.", i, i),
		}
	}
	s.journal.EXPECT().Read("", false).Return(records, false, nil).Maybe()
	// Nothing new after the initial batch.
	s.journal.EXPECT().Read(mock.Anything, false).Return(nil, false, nil).Maybe()

	s.network.EXPECT().ListInterfaces().Return(map[string]nmstate.Interface{
		"eth0": {Name: "eth0", Type: "ethernet"},
		"eth1": {Name: "eth1", Type: "ethernet"},
	}, nil).Maybe()
	s.network.EXPECT().CurrentConfig().Return(currentConfig, nil).Maybe()

	return s
}

func start(t *testing.T, src dashboard.Sources) *dashtest.Program {
	t.Helper()

	m := dashboard.New(src)
	return dashtest.Start(t, &m, width, height)
}

func requireScreen(t *testing.T, p *dashtest.Program) {
	t.Helper()

	view := p.View()
	dashtest.RequireFits(t, view, width, height)
	dashtest.RequireGolden(t, view)
}

// fillStaticConfig fills the network form with a static address on eth0,
// stopping at the confirmation step.
func fillStaticConfig(p *dashtest.Program) {
	p.Press(tea.KeyF2)
	p.Type("node1")
	p.Press(tea.KeyEnter)
	p.Type("1.1.1.1")
	p.Press(tea.KeyEnter)
	p.Press(tea.KeyDown, tea.KeyEnter) // Interface: eth0
	p.Press(tea.KeyDown, tea.KeyEnter) // Address mode: Static
	p.Type("192.168.1.10/24")
	p.Press(tea.KeyEnter)
	p.Type("192.168.1.1")
	p.Press(tea.KeyEnter)
}

func TestSummaryScreen(t *testing.T) {
	t.Parallel()

	p := start(t, newSources(t).Sources)
	requireScreen(t, p)
}

func TestNetworkScreen(t *testing.T) {
	t.Parallel()

	p := start(t, newSources(t).Sources)
	p.Press(tea.KeyF2)
	requireScreen(t, p)
}

func TestNetworkStaticPreview(t *testing.T) {
	t.Parallel()

	p := start(t, newSources(t).Sources)
	fillStaticConfig(p)
	requireScreen(t, p)
}

func TestNetworkApply(t *testing.T) {
	t.Parallel()

	src := newSources(t)
	want, err := network.FormData{
		Hostname:      "node1",
		Dns:           "1.1.1.1",
		InterfaceName: "eth0",
		Kind:          "static",
		Address:       "192.168.1.10/24",
		Gateway:       "192.168.1.1",
	}.ToSeleniteConfig(map[string]nmstate.Interface{"eth0": {Name: "eth0", Type: "ethernet"}})
	require.NoError(t, err)
	src.network.EXPECT().ApplyConfig(want).Return(nil).Once()

	p := start(t, src.Sources)
	fillStaticConfig(p)
	p.Type("y")

	requireScreen(t, p)
}

func TestNetworkApplyError(t *testing.T) {
	t.Parallel()

	src := newSources(t)
	src.network.EXPECT().ApplyConfig(mock.Anything).Return(errors.New("nmstatectl: interface eth0 not found")).Once()

	p := start(t, src.Sources)
	fillStaticConfig(p)
	p.Type("y")

	requireScreen(t, p)
}

func TestNetworkCancel(t *testing.T) {
	t.Parallel()

	p := start(t, newSources(t).Sources)
	fillStaticConfig(p)
	p.Type("n")

	requireScreen(t, p)
}

func TestSwitchBackToSummary(t *testing.T) {
	t.Parallel()

	p := start(t, newSources(t).Sources)
	p.Press(tea.KeyF2, tea.KeyF1)
	requireScreen(t, p)
}
