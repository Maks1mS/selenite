package machine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic-lab/selenite/internal/executil"
	config "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic-lab/selenite/internal/version"
)

const (
	ExecutorBin = "/usr/libexec/selenite-controller/selenite-machine-helper"
)

type machineManager struct {
	runner executil.Runner
	store  config.Store
}

func New() *machineManager {
	return &machineManager{
		runner: executil.ExecRunner{},
		store:  config.ConfigStore{},
	}
}

func (m *machineManager) run(ctx context.Context, w io.Writer, args ...string) error {
	if err := m.runner.Run(ctx, ExecutorBin, args, executil.WithCombinedOutput(w)); err != nil {
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

type imageInfoResult struct {
	Data struct {
		BootedImage struct {
			Image struct {
				Status struct {
					Booted struct {
						Image struct {
							Image struct {
								Image     string `json:"image"`
								Transport string `json:"transport"`
							} `json:"image"`
							ImageDigest string `json:"imageDigest"`
							Timestamp   string `json:"timestamp"`
							Version     any    `json:"version"`
						} `json:"image"`
					} `json:"booted"`
				} `json:"status"`
			} `json:"image"`
			Status string `json:"status"`
		} `json:"bootedImage"`
	} `json:"data"`
	Error any `json:"error"`
}

func (m *machineManager) imageInfo(ctx context.Context) (imageInfoResult, error) {
	var imgBuf bytes.Buffer
	if err := m.runner.Run(ctx, ExecutorBin, []string{"image-info"}, executil.WithCombinedOutput(&imgBuf)); err != nil {
		return imageInfoResult{}, fmt.Errorf("info: image: %w", err)
	}
	var imgResult imageInfoResult
	if err := json.Unmarshal(imgBuf.Bytes(), &imgResult); err != nil {
		return imageInfoResult{}, fmt.Errorf("info: parse image: %w", err)
	}
	return imgResult, nil
}

func (m *machineManager) Info(ctx context.Context, w io.Writer) error {
	cpuModel, cpuCores, err := readCPUInfo()
	if err != nil {
		return fmt.Errorf("info: cpu: %w", err)
	}

	totalMem, availMem, err := readMeminfo()
	if err != nil {
		return fmt.Errorf("info: mem: %w", err)
	}

	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return fmt.Errorf("info: kernel: %w", err)
	}

	podmanVer := podmanVersion(ctx, m.runner)

	imgResult, err := m.imageInfo(ctx)
	if err != nil {
		return fmt.Errorf("imageInfo: %w", err)
	}

	booted := imgResult.Data.BootedImage
	img := booted.Image.Status.Booted.Image

	_, _ = fmt.Fprintf(w, "CPU:            %s (%d cores)\n", cpuModel, cpuCores)
	_, _ = fmt.Fprintf(w, "Memory:         %s / %s\n", formatBytes(availMem), formatBytes(totalMem))
	_, _ = fmt.Fprintf(w, "Kernel:         %s\n", strings.TrimSpace(string(kernel)))
	_, _ = fmt.Fprintf(w, "Podman:         %s\n", podmanVer.Version)
	_, _ = fmt.Fprintf(w, "Podman Compose: %s\n", podmanVer.ComposeVersion)
	_, _ = fmt.Fprintf(w, "Version:        %s\n", version.Version)
	_, _ = fmt.Fprintf(w, "Git:            %s\n", version.GitCommit)
	_, _ = fmt.Fprintf(w, "Go:             %s\n", runtime.Version())
	_, _ = fmt.Fprintf(w, "Image Status:   %s\n", booted.Status)
	if img.Image.Image != "" {
		_, _ = fmt.Fprintf(w, "Image:          %s:%s\n", img.Image.Transport, img.Image.Image)
	}
	if img.ImageDigest != "" {
		_, _ = fmt.Fprintf(w, "Digest:         %s\n", img.ImageDigest)
	}
	if img.Timestamp != "" {
		_, _ = fmt.Fprintf(w, "Timestamp:      %s\n", img.Timestamp)
	}
	return nil
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

func (m *machineManager) Reboot(ctx context.Context, w io.Writer) error {
	err := m.run(ctx, w, "reboot")
	if err != nil {
		return fmt.Errorf("reboot: %w", err)
	}
	_, _ = fmt.Fprintln(w, "Rebooting...")
	return nil
}

func (m *machineManager) Upgrade(ctx context.Context, w io.Writer) error {
	if err := m.run(ctx, w, "bootc-upgrade"); err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	return nil
}

func (m *machineManager) Rollback(ctx context.Context, w io.Writer) error {
	if err := m.run(ctx, w, "bootc-rollback"); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	return nil
}

func (m *machineManager) Switch(ctx context.Context, w io.Writer, image string) error {
	if err := m.run(ctx, w, "bootc-switch", image); err != nil {
		return fmt.Errorf("switch: %w", err)
	}
	return nil
}

func (m *machineManager) ApplyConfig(ctx context.Context, r io.Reader, w io.Writer) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("ReadAll: %w", err)
	}

	cfg, err := config.ParseMachineConfig(buf)
	if err != nil {
		return fmt.Errorf("parseMachineConfig: %w", err)
	}

	prev, err := m.store.Load()
	if err != nil {
		return fmt.Errorf("load previous config: %w", err)
	}

	applier := config.NewApplier(cfg, prev, m.runner, m.store)
	if err := applier.Apply(ctx); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	return nil
}
