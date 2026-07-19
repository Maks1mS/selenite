// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

const (
	bootcPath      = "/usr/bin/bootc"
	journalctlPath = "/usr/bin/journalctl"
	dmesgPath      = "/usr/bin/dmesg"
)

func main() {
	runtime.LockOSThread()

	if len(os.Args) < 2 {
		cmdutils.Fatal("usage: selenite-machine-helper <command>\n")
	}
	command := os.Args[1]

	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin", "LANG=C"}

	var err error
	switch command {
	case "reboot":
		err = runReboot(env)
	case "image-info":
		err = runImageInfo(env)
	case "bootc-upgrade":
		err = runBootcUpgrade(env)
	case "bootc-rollback":
		err = runBootcRollback(env)
	case "bootc-switch":
		err = runBootcSwitch(env)
	case "dmesg":
		err = runDmesg(env)
	case "journalctl":
		err = runJournalctl(env)
	default:
		cmdutils.Fatal("unknown command: %s\n", command)
	}

	if err != nil {
		cmdutils.Fatal("%s: %v\n", command, err)
	}
}

func runReboot(env []string) error {
	//gosec:disable G204 -- Expected
	if err := syscall.Exec("/sbin/reboot", []string{"reboot"}, env); err != nil {
		return fmt.Errorf("reboot: %w", err)
	}
	return nil
}

func runImageInfo(env []string) error {
	//gosec:disable G204 -- Expected
	if err := syscall.Exec("/usr/bin/apm", []string{"apm", "system", "image", "status", "-f", "json"}, env); err != nil {
		return fmt.Errorf("image-info: %w", err)
	}
	return nil
}

func runBootcUpgrade(env []string) error {
	//gosec:disable G204 -- Expected
	if err := syscall.Exec(bootcPath, []string{"bootc", "upgrade"}, env); err != nil {
		return fmt.Errorf("bootc-upgrade: %w", err)
	}
	return nil
}

func runBootcRollback(env []string) error {
	//gosec:disable G204 -- Expected
	if err := syscall.Exec(bootcPath, []string{"bootc", "rollback"}, env); err != nil {
		return fmt.Errorf("bootc-rollback: %w", err)
	}
	return nil
}

func runBootcSwitch(env []string) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("image argument required")
	}
	//gosec:disable G204,G702 -- Expected
	if err := syscall.Exec(bootcPath, []string{"bootc", "switch", os.Args[2]}, env); err != nil {
		return fmt.Errorf("bootc-switch: %w", err)
	}
	return nil
}

func buildDmesgArgs() ([]string, error) {
	args := []string{"dmesg"}
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--ctime", "-T", "--follow", "-w":
			args = append(args, arg)
		case "--level", "-l":
			if i+1 >= len(os.Args) {
				return nil, fmt.Errorf("%s requires an argument", arg)
			}
			i++
			levelArg := os.Args[i]
			validLevels := map[string]bool{
				"emerg": true, "alert": true, "crit": true, "err": true,
				"warn": true, "notice": true, "info": true, "debug": true,
			}
			for lvl := range strings.SplitSeq(levelArg, ",") {
				if !validLevels[lvl] {
					return nil, fmt.Errorf("invalid level: %s", lvl)
				}
			}
			args = append(args, arg, levelArg)
		default:
			return nil, fmt.Errorf("unsupported argument: %s", arg)
		}
	}
	return args, nil
}

func runDmesg(env []string) error {
	args, err := buildDmesgArgs()
	if err != nil {
		return fmt.Errorf("dmesg: %w", err)
	}
	//gosec:disable G204,G702 -- Expected
	if err := syscall.Exec(dmesgPath, args, env); err != nil {
		return fmt.Errorf("dmesg: %w", err)
	}
	return nil
}

func buildJournalctlArgs() ([]string, error) {
	args := []string{"journalctl"}
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--no-pager", "--output=short-iso", "-f", "--follow":
			args = append(args, arg)
		case "-n", "--lines", "-u", "--unit":
			if i+1 >= len(os.Args) {
				return nil, fmt.Errorf("%s requires an argument", arg)
			}
			i++
			args = append(args, arg, os.Args[i])
		default:
			return nil, fmt.Errorf("unsupported argument: %s", arg)
		}
	}
	return args, nil
}

func runJournalctl(env []string) error {
	args, err := buildJournalctlArgs()
	if err != nil {
		return fmt.Errorf("journalctl: %w", err)
	}
	//gosec:disable G204,G702 -- Expected
	if err := syscall.Exec(journalctlPath, args, env); err != nil {
		return fmt.Errorf("journalctl: %w", err)
	}
	return nil
}
