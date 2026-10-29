// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

// TIOCL_SETKMSGREDIRECT, not exported by golang.org/x/sys/unix.
const tioclSetKmsgRedirect = 11

// maxVT is MAX_NR_CONSOLES in the kernel.
const maxVT = 63

const usage = "usage: selenite-vt-helper kmsg-redirect <vt>\n"

func main() {
	vt, err := parseArgs(os.Args[1:])
	if err != nil {
		cmdutils.Fatal("%v\n%s", err, usage)
	}

	if err := setKmsgRedirect(vt); err != nil {
		cmdutils.Fatal("kmsg-redirect: %v\n", err)
	}
}

func parseArgs(args []string) (int, error) {
	if len(args) != 2 {
		return 0, errors.New("expected exactly two arguments")
	}

	if args[0] != "kmsg-redirect" {
		return 0, fmt.Errorf("unknown command: %s", args[0])
	}

	vt, err := strconv.Atoi(args[1])
	if err != nil || vt < 1 || vt > maxVT {
		return 0, fmt.Errorf("invalid vt: %q", args[1])
	}

	return vt, nil
}

// setKmsgRedirect makes the kernel print its messages to the given VT
// instead of the foreground one.
func setKmsgRedirect(vt int) error {
	fd, err := unix.Open("/dev/tty0", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open /dev/tty0: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()

	//gosec:disable G115 -- vt is validated by parseArgs
	arg := [2]byte{tioclSetKmsgRedirect, byte(vt)}
	//gosec:disable G103 -- TIOCLINUX takes a pointer to a byte buffer
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCLINUX, uintptr(unsafe.Pointer(&arg[0])))
	if errno != 0 {
		return fmt.Errorf("ioctl TIOCLINUX: %w", errno)
	}
	return nil
}
