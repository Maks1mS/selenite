// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package status

import (
	"math"
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type CPUInfo struct {
	Cores   int
	FreqGHz float64
}

type Stats struct {
	CPUPercent  float64
	MemPercent  float64
	MemTotal    uint64
	MemUsed     uint64
	SwapPercent float64
	SwapTotal   uint64
	SwapUsed    uint64
	Uptime      time.Duration
	Hostname    string
}

// Source provides the data shown in the status bar.
type Source interface {
	CPUInfo() CPUInfo
	Stats() Stats
	Hostname() string
}

// SystemSource reads the data from the local system.
type SystemSource struct{}

func (SystemSource) CPUInfo() CPUInfo {
	cores, err := cpu.Counts(true)
	if err != nil || cores == 0 {
		cores = 1
	}

	var freqGHz float64
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		freqGHz = infos[0].Mhz / 1000
	}

	return CPUInfo{Cores: cores, FreqGHz: freqGHz}
}

func (s SystemSource) Stats() Stats {
	var st Stats

	if percentages, err := cpu.Percent(0, false); err == nil && len(percentages) > 0 {
		st.CPUPercent = percentages[0]
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		st.MemPercent = vm.UsedPercent
		st.MemTotal = vm.Total
		st.MemUsed = vm.Used
	}

	if sw, err := mem.SwapMemory(); err == nil {
		st.SwapTotal = sw.Total
		st.SwapUsed = sw.Used
		st.SwapPercent = sw.UsedPercent
	}

	if secs, err := host.Uptime(); err == nil {
		if secs > math.MaxInt64/uint64(time.Second) {
			st.Uptime = time.Duration(math.MaxInt64)
		} else {
			st.Uptime = time.Duration(secs) * time.Second
		}
	}

	st.Hostname = s.Hostname()

	return st
}

func (SystemSource) Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
