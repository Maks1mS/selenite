package serve

import (
	"context"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/machine"
)

func machineCommands() *cli.Command {
	return &cli.Command{
		Name:  "machine",
		Usage: "Manage machine",
		Commands: []*cli.Command{
			statsCommand(),
			rebootCommand(),
		},
	}
}

func statsCommand() *cli.Command {
	return &cli.Command{
		Name:  "stats",
		Usage: "Show machine stats",
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Stats(ctx, c.Writer)
		},
	}
}

func rebootCommand() *cli.Command {
	return &cli.Command{
		Name:  "reboot",
		Usage: "Reboot",
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Reboot(ctx, c.Writer)
		},
	}
}
