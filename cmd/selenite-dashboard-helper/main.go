// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

var allowedSubcommands = map[string]struct{}{
	"_dashboard-set-network": {},
	"_dashboard-get-network": {},
}

func main() {
	subcommand := os.Args[1]
	extra := os.Args[2:]

	if _, has := allowedSubcommands[subcommand]; !has {
		cmdutils.Fatal("unknown subcommand: %s\n", subcommand)
	}

	args := append([]string{"selenite-controller", subcommand}, extra...)
	//gosec:disable G204,G702 -- Expected
	if err := syscall.Exec("/usr/bin/selenite-controller", args, []string{}); err != nil {
		cmdutils.Fatal("exec: %v\n", err)
	}
}
