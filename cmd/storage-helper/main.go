// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"os"
	"runtime"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/helpers/root/storage"
)

func main() {
	runtime.LockOSThread()
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	if len(os.Args) < 2 {
		cmdutils.Fatal("usage: storage-helper <apply|check|retry>\n")
	}

	ctx := context.Background()
	mgr := storage.NewManager(storage.HostSystemctl{})

	var err error
	switch os.Args[1] {
	case "apply":
		err = runApply(ctx, mgr)
	case "check":
		err = mgr.Check(ctx)
	case "retry":
		err = mgr.Retry(ctx)
	default:
		cmdutils.Fatal("unknown command: %s\n", os.Args[1])
	}
	if err != nil {
		cmdutils.Fatal("%s: %v\n", os.Args[1], err)
	}
}

func runApply(ctx context.Context, mgr *storage.Manager) error {
	mounts, err := storage.Decode(os.Stdin)
	if err != nil {
		return err //nolint:wrapcheck // errors are self-describing; the caller adds the command name
	}
	return mgr.Apply(ctx, mounts) //nolint:wrapcheck // errors are self-describing; the caller adds the command name
}
