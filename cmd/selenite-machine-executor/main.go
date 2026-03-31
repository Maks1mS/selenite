package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

const bootcPath = "/usr/bin/bootc"

func main() {
	runtime.LockOSThread()

	if len(os.Args) < 2 {
		fatal("usage: selenite-machine-executor <command>\n")
	}

	command := os.Args[1]

	if err := syscall.Setresgid(0, 0, 0); err != nil {
		fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		fatal("setresuid: %v\n", err)
	}

	env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin", "LANG=C"}

	switch command {
	case "reboot":
		if err := syscall.Exec("/sbin/reboot", []string{"reboot"}, env); err != nil {
			fatal("reboot: %v\n", err)
		}
	case "image-info":
		if err := syscall.Exec("/usr/bin/apm", []string{"apm", "system", "image", "status", "-f", "json"}, env); err != nil {
			fatal("image-info: %v\n", err)
		}
	case "bootc-upgrade":
		//gosec:disable G204 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "upgrade"}, env); err != nil {
			fatal("bootc-upgrade: %v\n", err)
		}
	case "bootc-rollback":
		//gosec:disable G204 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "rollback"}, env); err != nil {
			fatal("bootc-rollback: %v\n", err)
		}
	case "bootc-switch":
		if len(os.Args) < 3 {
			fatal("switch: image argument required\n")
		}
		//gosec:disable G204 -- Expected
		if err := syscall.Exec(bootcPath, []string{"bootc", "switch", os.Args[2]}, env); err != nil {
			fatal("bootc-switch: %v\n", err)
		}
	default:
		fatal("unknown command: %s\n", command)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
