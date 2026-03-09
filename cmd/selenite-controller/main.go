package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	_ = ctx
	// err := app.Run(ctx)
	// if err != nil {
	// 	panic(err)
	// }
}
