// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

const (
	podmanPath = "/usr/bin/podman"
)

func checkCaller() error {
	u, err := user.Lookup("_selenite-controller")
	if err != nil {
		return fmt.Errorf("lookup user: %w", err)
	}
	expectedUID, _ := strconv.Atoi(u.Uid)
	if os.Getuid() != expectedUID {
		return fmt.Errorf("access denied: caller uid %d, expected %d", os.Getuid(), expectedUID)
	}

	ppid := os.Getppid()
	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", ppid))
	if err != nil {
		return fmt.Errorf("read parent exe: %w", err)
	}
	if filepath.Base(exePath) != "selenite-controller" {
		return fmt.Errorf("access denied: unexpected parent %s", exePath)
	}
	return nil
}

func main() {
	runtime.LockOSThread()

	if len(os.Args) < 2 {
		cmdutils.Fatal("usage: debug-helper <image> [command...]\n")
	}

	if err := checkCaller(); err != nil {
		cmdutils.Fatal("%v\n", err)
	}

	image := os.Args[1]
	command := os.Args[2:]

	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin", "LANG=C"}

	args := []string{
		"run",
		"--rm",
		"--privileged",
		"--pid", "host",
		"--ipc", "host",
		"--network", "host",
		"--uts", "host",
		"--cgroupns", "host",
		"--security-opt", "label=disable",
		"-i", "-t",
		"-v", "/:/host:rw",
		"--workdir", "/host",
		image,
	}
	if len(command) > 0 {
		args = append(args, command...)
	} else {
		args = append(args, "sh")
	}

	//nolint:gosec // podmanPath is a const, image and command are user input for the debug container
	cmd := exec.Command(podmanPath, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "run: %v\n", err)
	}

	// Remove the image so it does not accumulate on the host.
	//nolint:gosec // podmanPath is a const, image is user input
	rmCmd := exec.Command(podmanPath, "image", "rm", image)
	rmCmd.Env = env
	_ = rmCmd.Run()
}
