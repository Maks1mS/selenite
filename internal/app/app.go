package app

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/serve"
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
		},
	}
	if err := rootCmd.Run(ctx, os.Args); err != nil {
		return fmt.Errorf("failed to run root command: %w", err)
	}
	return nil
}
