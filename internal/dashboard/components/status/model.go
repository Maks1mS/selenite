// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package status

import (
	"fmt"
	"math"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
	"altlinux.space/alt-atomic/selenite/internal/version"
)

const refreshInterval = 1 * time.Second

// tickMsg triggers the next stats poll.
type tickMsg time.Time

// statsMsg carries the results of a CPU/RAM measurement.
type statsMsg struct {
	cpuPercent float64
	memPercent float64
	memTotal   uint64
	memUsed    uint64
	swapTotal  uint64
	swapUsed   uint64
	swapPct    float64
	uptime     time.Duration
	hostname   string
}

type Model struct {
	hostname   string
	osInfo     string
	cpuPct     float64
	cpuCores   int
	cpuFreqGHz float64
	memPct     float64
	memTotal   uint64
	memUsed    uint64
	swapPct    float64
	swapTotal  uint64
	swapUsed   uint64
	uptime     time.Duration
}

func New() *Model {
	cores, err := cpu.Counts(true)
	if err != nil || cores == 0 {
		cores = 1
	}

	var freqGHz float64
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		freqGHz = infos[0].Mhz / 1000
	}

	return &Model{
		osInfo:     getOSInfo(),
		cpuCores:   cores,
		cpuFreqGHz: freqGHz,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(fetchStatsCmd(), tickCmd())
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tea.Batch(fetchStatsCmd(), tickCmd())
	case statsMsg:
		m.cpuPct = msg.cpuPercent
		m.memPct = msg.memPercent
		m.memTotal = msg.memTotal
		m.memUsed = msg.memUsed
		m.swapPct = msg.swapPct
		m.swapTotal = msg.swapTotal
		m.swapUsed = msg.swapUsed
		m.uptime = msg.uptime
		m.hostname = msg.hostname
		return m, nil
	case base.NetworkUpdateMsg:
		m.hostname = getHostname()
		return m, nil
	}
	return m, nil
}

func (m *Model) View() tea.View {
	line := fmt.Sprintf(
		" %s │ %s │ CPU %s %5.1f%%, RAM %s%s, uptime %s ",
		m.hostname,
		m.osInfo,
		formatCPUInfo(m.cpuCores, m.cpuFreqGHz),
		m.cpuPct,
		formatMem(m.memUsed, m.memTotal, m.memPct),
		formatSwapPart(m.swapTotal, m.swapUsed, m.swapPct),
		formatUptime(m.uptime),
	)
	return tea.NewView(line)
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		var cpuPct float64
		if percentages, err := cpu.Percent(0, false); err == nil && len(percentages) > 0 {
			cpuPct = percentages[0]
		}

		var memPct float64
		var memTotal, memUsed uint64
		if vm, err := mem.VirtualMemory(); err == nil {
			memPct = vm.UsedPercent
			memTotal = vm.Total
			memUsed = vm.Used
		}

		var swapPct float64
		var swapTotal, swapUsed uint64
		if sw, err := mem.SwapMemory(); err == nil {
			swapTotal = sw.Total
			swapUsed = sw.Used
			swapPct = sw.UsedPercent
		}

		var uptime time.Duration
		if secs, err := host.Uptime(); err == nil {
			if secs > math.MaxInt64/uint64(time.Second) {
				uptime = time.Duration(math.MaxInt64)
			} else {
				uptime = time.Duration(secs) * time.Second
			}
		}

		return statsMsg{
			cpuPercent: cpuPct,
			memPercent: memPct,
			memTotal:   memTotal,
			memUsed:    memUsed,
			swapPct:    swapPct,
			swapTotal:  swapTotal,
			swapUsed:   swapUsed,
			uptime:     uptime,
			hostname:   getHostname(),
		}
	}
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
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
