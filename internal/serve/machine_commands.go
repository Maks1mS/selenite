package serve

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/machine"
)

const imagePlaceholder = "<image>"

var errImageRequired = errors.New("image required")

func machineCommands() *cli.Command {
	return &cli.Command{
		Name:  "machine",
		Usage: "Manage machine",
		Commands: []*cli.Command{
			infoCommand(),
			statsCommand(),
			rebootCommand(),
			upgradeCommand(),
			rollbackCommand(),
			switchCommand(),
			configCommand(),
		},
	}
}

func infoCommand() *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: "Show machine info",
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Info(ctx, c.Writer)
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

func upgradeCommand() *cli.Command {
	return &cli.Command{
		Name:  "upgrade",
		Usage: "Upgrade",
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Upgrade(ctx, c.Writer)
		},
	}
}

func rollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback",
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Rollback(ctx, c.Writer)
		},
	}
}

func switchCommand() *cli.Command {
	return &cli.Command{
		Name:      "switch",
		Usage:     "Switch to a different image",
		ArgsUsage: imagePlaceholder,
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errImageRequired
			}
			return machine.New().Switch(ctx, c.Writer, c.Args().First())
		},
	}
}

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Machine config",
		Commands: []*cli.Command{
			applyConfigCommand(),
		},
	}
}

func applyConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "Apply machine config",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "reboot",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			m := machine.New()
			if err := m.ApplyConfig(ctx, c.Reader, c.Writer); err != nil {
				return fmt.Errorf("applyConfig: %w", err)
			}

			if c.Bool("reboot") {
				if err := m.Reboot(ctx, c.Writer); err != nil {
					return fmt.Errorf("reboot: %w", err)
				}
			}
			return nil
		},
	}
}
