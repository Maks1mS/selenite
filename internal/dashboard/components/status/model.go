// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package status

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
	"altlinux.space/alt-atomic/selenite/internal/version"
)

const refreshInterval = 1 * time.Second

// tickMsg triggers the next stats poll.
type tickMsg time.Time

// statsMsg carries the results of a CPU/RAM measurement.
type statsMsg Stats

type Model struct {
	source Source

	osInfo string
	cpu    CPUInfo
	stats  Stats
}

func New(source Source) *Model {
	return &Model{
		source: source,
		osInfo: getOSInfo(),
		cpu:    source.CPUInfo(),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetchStatsCmd(), tickCmd())
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tea.Batch(m.fetchStatsCmd(), tickCmd())
	case statsMsg:
		m.stats = Stats(msg)
		return m, nil
	case base.NetworkUpdateMsg:
		m.stats.Hostname = m.source.Hostname()
		return m, nil
	}
	return m, nil
}

func (m *Model) View() tea.View {
	line := fmt.Sprintf(
		" %s │ %s │ CPU %s %5.1f%%, RAM %s%s, uptime %s ",
		m.stats.Hostname,
		m.osInfo,
		formatCPUInfo(m.cpu.Cores, m.cpu.FreqGHz),
		m.stats.CPUPercent,
		formatMem(m.stats.MemUsed, m.stats.MemTotal, m.stats.MemPercent),
		formatSwapPart(m.stats.SwapTotal, m.stats.SwapUsed, m.stats.SwapPercent),
		formatUptime(m.stats.Uptime),
	)
	return tea.NewView(line)
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		return statsMsg(m.source.Stats())
	}
}

func getOSInfo() string {
	return fmt.Sprintf("%s %s", version.Name, version.Version)
}

func formatBytesGB(b uint64) float64 {
	return float64(b) / (1024 * 1024 * 1024)
}

func formatMem(used, total uint64, pct float64) string {
	return fmt.Sprintf("%.1f/%.1f GB (%.1f%%)", formatBytesGB(used), formatBytesGB(total), pct)
}

func formatSwapPart(total, used uint64, pct float64) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf(", SWAP %.1f/%.1f GB (%.1f%%)", formatBytesGB(used), formatBytesGB(total), pct)
}

func formatUptime(d time.Duration) string {
	total := int64(d.Seconds())
	days := total / 86400
	hours := (total / 3600) % 24
	minutes := (total / 60) % 60
	seconds := total % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh%dm%ds", days, hours, minutes, seconds)
	case hours > 0:
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatCPUInfo(cores int, freqGHz float64) string {
	if freqGHz <= 0 {
		return fmt.Sprintf("%d cores", cores)
	}
	return fmt.Sprintf("%dx%.1fGHz", cores, freqGHz)
}
