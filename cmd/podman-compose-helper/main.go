// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/compose"
)

const (
	executorUser  = "_selenite-podman-executor"
	podmanCompose = "/usr/bin/podman-compose"
)

var allowedSubcommands = map[string]struct{}{
	"up":   {},
	"down": {},
	"ps":   {},
	"logs": {},
	"pull": {},
}

func main() {
	runtime.LockOSThread()

	if len(os.Args) < 3 {
		cmdutils.Fatal("usage: selenite-podman-executor <stack-path> <subcommand> [args...]\n")
	}

	stackPath := os.Args[1]
	subcommand := os.Args[2]
	extra := os.Args[3:]

	if !strings.HasPrefix(stackPath, compose.BaseStackDir+"/") {
		cmdutils.Fatal("invalid stack path: %s\n", stackPath)
	}

	if _, has := allowedSubcommands[subcommand]; !has {
		cmdutils.Fatal("unknown subcommand: %s\n", subcommand)
	}

	u, err := user.Lookup(executorUser)
	if err != nil {
		cmdutils.Fatal("user not found: %s\n", executorUser)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	if err := os.Chdir(stackPath); err != nil {
		cmdutils.Fatal("chdir %s: %v\n", stackPath, err)
	}

	args := append([]string{"podman-compose", subcommand}, extra...)
	if err := syscall.Exec(podmanCompose, args, buildEnv(u)); err != nil { //nolint:gosec // podmanCompose is a const, subcommand is allow-listed
		cmdutils.Fatal("exec: %v\n", err)
	}
}

func buildEnv(u *user.User) []string {
	override := map[string]string{
		"HOME":            u.HomeDir,
		"USER":            u.Username,
		"LOGNAME":         u.Username,
		"PATH":            "/usr/local/bin:/usr/bin:/bin",
		"XDG_RUNTIME_DIR": fmt.Sprintf("/run/user/%s", u.Uid),
	}

	drop := map[string]struct{}{
		"DBUS_SESSION_BUS_ADDRESS": {},
		"XDG_SESSION_ID":           {},
	}

	result := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if _, skip := drop[key]; skip {
			continue
		}
		if val, ok := override[key]; ok {
			result = append(result, key+"="+val)
			delete(override, key)
		} else {
			result = append(result, e)
		}
	}
	for k, v := range override {
		result = append(result, k+"="+v)
	}
	return result
}
