// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/privdrop"
)

const (
	podmanBin = "/usr/bin/podman"
)

var allowedSubcommands = map[string]struct{}{
	"create":  {},
	"ls":      {},
	"rm":      {},
	"inspect": {},
	"prune":   {},
}

func main() {
	if len(os.Args) < 2 {
		cmdutils.Fatal("usage: podman-network-helper <subcommand> [args...]\n")
	}

	subcommand := os.Args[1]
	extra := os.Args[2:]

	if _, has := allowedSubcommands[subcommand]; !has {
		cmdutils.Fatal("unknown subcommand: %s\n", subcommand)
	}

	u, err := privdrop.ToUser(privdrop.PodmanExecutorUser)
	if err != nil {
		cmdutils.Fatal("%v\n", err)
	}

	args := append([]string{"podman", "network", subcommand}, extra...)
	if err := syscall.Exec(podmanBin, args, privdrop.BuildEnv(u)); err != nil { //nolint:gosec // podmanBin is a const, subcommand is allow-listed
		cmdutils.Fatal("exec: %v\n", err)
	}
}
