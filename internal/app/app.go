// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package app

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/machine"
	"altlinux.space/alt-atomic/selenite/internal/serve"
)

func Run(ctx context.Context) error {
	rootCmd := &cli.Command{
		Name: "selenite-controller",
		Commands: []*cli.Command{
			{
				Name: "serve",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "host",
						Value: "0.0.0.0",
					},
					&cli.IntFlag{
						Name:  "port",
						Value: 23234,
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					return serve.Serve(ctx, serve.ServeOptions{
						Host: c.String("host"),
						Port: c.Int("port"),
					})
				},
			},
			{
				Name:  "reconcile",
				Usage: "converge on-disk state to the stored machine config",
				Action: func(ctx context.Context, _ *cli.Command) error {
					return machine.New().Reconcile(ctx)
				},
			},
		},
	}
	if err := rootCmd.Run(ctx, os.Args); err != nil {
		return fmt.Errorf("failed to run root command: %w", err)
	}
	return nil
}
