// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

const zramGeneratorConf = "/etc/systemd/zram-generator.conf"

func main() {
	runtime.LockOSThread()
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	if len(os.Args) < 2 {
		cmdutils.Fatal("usage: zram-helper <command>\n")
	}
	command := os.Args[1]

	switch command {
	case "apply":
		if err := runApply(); err != nil {
			cmdutils.Fatal("apply: %v\n", err)
		}
	default:
		cmdutils.Fatal("unknown command: %s\n", command)
	}
}

func runApply() error {
	enabled := os.Getenv("SELENITE_ZRAM_ENABLED") == "1"

	if !enabled {
		if unitExists("dev-zram0.swap") {
			if err := runSystemctl("stop", "dev-zram0.swap"); err != nil {
				return fmt.Errorf("stop dev-zram0.swap: %w", err)
			}
		}
		if err := os.Remove(zramGeneratorConf); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", zramGeneratorConf, err)
		}
	} else {
		size := os.Getenv("SELENITE_ZRAM_SIZE")
		compression := os.Getenv("SELENITE_ZRAM_COMPRESSION")
		priority := os.Getenv("SELENITE_ZRAM_PRIORITY")

		var b strings.Builder
		fmt.Fprintf(&b, "[zram0]\n")
		if size != "" {
			fmt.Fprintf(&b, "zram-size=%s\n", size)
		}
		if compression != "" {
			fmt.Fprintf(&b, "compression-algorithm=%s\n", compression)
		}
		if priority != "" {
			fmt.Fprintf(&b, "swap-priority=%s\n", priority)
		}

		//gosec:disable G306 -- Expected
		if err := os.WriteFile(zramGeneratorConf, []byte(b.String()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", zramGeneratorConf, err)
		}
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	if enabled {
		if err := runSystemctl("restart", "dev-zram0.swap"); err != nil {
			return fmt.Errorf("restart dev-zram0.swap: %w", err)
		}
	}

	return nil
}

func runSystemctl(args ...string) error {
	//gosec:disable G204 -- Expected
	cmd := exec.Command("/usr/bin/systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func unitExists(unit string) bool {
	//gosec:disable G204 -- Expected
	cmd := exec.Command("/usr/bin/systemctl", "cat", unit)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
