package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

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

	switch command {
	case "reboot":
		env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin"}
		if err := syscall.Exec("/sbin/reboot", []string{"reboot"}, env); err != nil {
			fatal("reboot: %v\n", err)
		}
	case "image-info":
		env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin", "LANG=C"}
		if err := syscall.Exec("/usr/bin/apm", []string{"apm", "system", "image", "status", "-f", "json"}, env); err != nil {
			fatal("image-info: %v\n", err)
		}
	default:
		fatal("unknown command: %s\n", command)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
