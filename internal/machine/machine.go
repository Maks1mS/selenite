package machine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
)

const (
	ExecutorBin = "/usr/libexec/selenite-machine-executor"
)

type machineManager struct {
	runner executil.Runner
}

func New() *machineManager {
	return &machineManager{
		runner: executil.ExecRunner{},
	}
}

func (m *machineManager) run(ctx context.Context, w io.Writer, args ...string) error {
	if err := m.runner.Run(ctx, w, ExecutorBin, args...); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func (m *machineManager) Stats(_ context.Context, w io.Writer) error {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return fmt.Errorf("stats: read uptime: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("stats: unexpected /proc/uptime format")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fmt.Errorf("stats: parse uptime: %w", err)
	}

	totalMem, availMem, err := readMeminfo()
	if err != nil {
		return fmt.Errorf("stats: read meminfo: %w", err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/var", &stat); err != nil {
		return fmt.Errorf("stats: statfs: %w", err)
	}
	if stat.Bsize < 0 {
		return fmt.Errorf("stats: statfs: negative block size %d", stat.Bsize)
	}
	diskTotal := stat.Blocks * uint64(stat.Bsize)
	diskFree := stat.Bfree * uint64(stat.Bsize)
	diskUsed := diskTotal - diskFree

	usedMem := totalMem - availMem
	_, _ = fmt.Fprintf(w,
		"Uptime:   %s\nMemory:   %s / %s used\nDisk:     %s / %s used\n",
		formatUptime(int(secs)),
		formatBytes(usedMem), formatBytes(totalMem),
		formatBytes(diskUsed), formatBytes(diskTotal),
	)
	return nil
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

func formatBytes(b uint64) string {
	gb := float64(b) / (1024 * 1024 * 1024)
	if gb >= 1 {
		return fmt.Sprintf("%.1fG", gb)
	}
	mb := float64(b) / (1024 * 1024)
	return fmt.Sprintf("%.1fM", mb)
}

func (m *machineManager) Reboot(ctx context.Context, w io.Writer) error {
	err := m.run(ctx, w, "reboot")
	if err != nil {
		return fmt.Errorf("reboot: %w", err)
	}
	_, _ = fmt.Fprintln(w, "Rebooting...")
	return nil
}
