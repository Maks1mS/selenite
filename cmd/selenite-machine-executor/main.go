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

	switch command {
	case "reboot":
		env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin"}
		if err := syscall.Exec("/sbin/reboot", []string{"reboot"}, env); err != nil {
			fatal("reboot: %v\n", err)
		}
	default:
		fatal("unknown command: %s\n", command)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
