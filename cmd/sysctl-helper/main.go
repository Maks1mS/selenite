package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic-lab/selenite/internal/cmdutils"
)

const (
	sysctlBin  = "/usr/sbin/sysctl"
	sysctlConf = "/etc/sysctl.d/99-selenite.conf"
)

func main() {
	runtime.LockOSThread()
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.Contains(line, "=") {
			cmdutils.Fatal("invalid sysctl entry (missing '='): %q\n", line)
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		cmdutils.Fatal("read stdin: %v\n", err)
	}

	if err := writeConf(lines); err != nil {
		cmdutils.Fatal("write %s: %v\n", sysctlConf, err)
	}

	var stderrBuf bytes.Buffer
	cmd := exec.Command(sysctlBin, "--system")
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		cmdutils.Fatal("sysctl --system: %v\nstderr: %s\n", err, stderrBuf.String())
	}
}

func writeConf(lines []string) error {
	if len(lines) == 0 {
		err := os.Remove(sysctlConf)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("remove: %w", err)
		}
		return nil
	}
	content := strings.Join(lines, "\n") + "\n"

	//gosec:disable G306 -- Expected
	if err := os.WriteFile(sysctlConf, []byte(content), 0o644); err != nil {
		return fmt.Errorf("sysctlConf: %w", err)
	}

	return nil
}
