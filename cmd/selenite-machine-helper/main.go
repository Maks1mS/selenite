// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"runtime"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

const (
	bootcPath      = "/usr/bin/bootc"
	journalctlPath = "/usr/bin/journalctl"
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

	switch command {
	case "reboot":
		if err := syscall.Exec("/sbin/reboot", []string{"reboot"}, env); err != nil {
			cmdutils.Fatal("reboot: %v\n", err)
		}
	case "image-info":
		if err := syscall.Exec("/usr/bin/apm", []string{"apm", "system", "image", "status", "-f", "json"}, env); err != nil {
			cmdutils.Fatal("image-info: %v\n", err)
		}
	case "bootc-upgrade":
		//gosec:disable G204 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "upgrade"}, env); err != nil {
			cmdutils.Fatal("bootc-upgrade: %v\n", err)
		}
	case "bootc-rollback":
		//gosec:disable G204 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "rollback"}, env); err != nil {
			cmdutils.Fatal("bootc-rollback: %v\n", err)
		}
	case "bootc-switch":
		if len(os.Args) < 3 {
			cmdutils.Fatal("switch: image argument required\n")
		}
		//gosec:disable G204,G702 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "switch", os.Args[2]}, env); err != nil {
			cmdutils.Fatal("bootc-switch: %v\n", err)
		}
	case "journalctl":
		args := []string{"journalctl"}
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch arg {
			case "--no-pager", "--output=short-iso", "-f", "--follow":
				args = append(args, arg)
			case "-n", "--lines", "-u", "--unit":
				if i+1 >= len(os.Args) {
					cmdutils.Fatal("journalctl: %s requires an argument\n", arg)
				}
				i++
				args = append(args, arg, os.Args[i])
			default:
				cmdutils.Fatal("journalctl: unsupported argument: %s\n", arg)
			}
		}
		//gosec:disable G204,G702 -- Expected
		if err := syscall.Exec(journalctlPath, args, env); err != nil {
			cmdutils.Fatal("journalctl: %v\n", err)
		}
	default:
		cmdutils.Fatal("unknown command: %s\n", command)
	}
}
