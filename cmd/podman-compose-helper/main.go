// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"strings"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/compose"
	"altlinux.space/alt-atomic/selenite/internal/privdrop"
)

const (
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

	u, err := privdrop.ToUser(privdrop.PodmanExecutorUser)
	if err != nil {
		cmdutils.Fatal("%v\n", err)
	}

	if err := os.Chdir(stackPath); err != nil {
		cmdutils.Fatal("chdir %s: %v\n", stackPath, err)
	}

	args := append([]string{"podman-compose", subcommand}, extra...)
	if err := syscall.Exec(podmanCompose, args, privdrop.BuildEnv(u)); err != nil { //nolint:gosec // podmanCompose is a const, subcommand is allow-listed
		cmdutils.Fatal("exec: %v\n", err)
	}
}
