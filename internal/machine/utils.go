// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package machine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

type PodmanVersion struct {
	Version        string
	ComposeVersion string
}

func podmanVersion(ctx context.Context, r executil.Runner) PodmanVersion {
	var composeBuf bytes.Buffer
	_ = r.Run(ctx, "podman-compose", []string{"--version"}, executil.WithStdout(&composeBuf))
	out := strings.TrimSpace(composeBuf.String())

	rePodman := regexp.MustCompile(`podman version (\d[\d.]*)`)
	reCompose := regexp.MustCompile(`podman-compose version (\d[\d.]*)`)

	podmanMatch := rePodman.FindStringSubmatch(out)
	composeMatch := reCompose.FindStringSubmatch(out)

	version := PodmanVersion{"N/A", "N/A"}

	if len(podmanMatch) >= 2 {
		version.Version = podmanMatch[1]
	}
	if len(composeMatch) >= 2 {
		version.ComposeVersion = composeMatch[1]
	}

	return version
}

func formatUptime(secs int) string {
	days := secs / 86400
	hours := (secs % 86400) / 3600
	mins := (secs % 3600) / 60
	s := secs % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d days", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hours", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%d minutes", mins))
	}
	parts = append(parts, fmt.Sprintf("%d seconds", s))
	return strings.Join(parts, " ")
}

func readCPUInfo() (model string, cores int, err error) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", 0, fmt.Errorf("open /proc/cpuinfo: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	if err := scanner.Err(); err != nil {
		return "", 0, fmt.Errorf("scanner: %w", err)
	}
	return model, cores, nil
}

func readMeminfo() (totalBytes, availBytes uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var totalKB, availKB uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var key string
		var val uint64
		if _, err := fmt.Sscanf(line, "%s %d kB", &key, &val); err != nil {
			continue
		}
		switch key {
		case "MemTotal:":
			totalKB = val
		case "MemAvailable:":
			availKB = val
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("scanner: %w", err)
	}
	return totalKB * 1024, availKB * 1024, nil
}

func readDiskInfo() (diskTotal, diskFree uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/var", &stat); err != nil {
		return 0, 0, fmt.Errorf("stats: statfs: %w", err)
	}
	if stat.Bsize < 0 {
		return 0, 0, fmt.Errorf("stats: statfs: negative block size %d", stat.Bsize)
	}
	return stat.Blocks * uint64(stat.Bsize), stat.Bfree * uint64(stat.Bsize), nil
}

func formatBytes(b uint64) string {
	gb := float64(b) / (1024 * 1024 * 1024)
	if gb >= 1 {
		return fmt.Sprintf("%.1fG", gb)
	}
	mb := float64(b) / (1024 * 1024)
	return fmt.Sprintf("%.1fM", mb)
}
