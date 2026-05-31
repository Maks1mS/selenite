// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"altlinux.space/alt-atomic-lab/selenite/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	err := app.Run(ctx)
	if err != nil {
		panic(err)
	}
}
