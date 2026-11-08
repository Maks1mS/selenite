// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/machine"
)

const (
	imagePlaceholder   = "<image>"
	servicePlaceholder = "<service>"
)

var errImageRequired = errors.New("image required")

func machineCommands() *cli.Command {
	return &cli.Command{
		Name:  "machine",
		Usage: "Manage machine lifecycle and configuration",
		Commands: []*cli.Command{
			logsCommand(),
			dmesgCommand(),
			infoCommand(),
			rebootCommand(),
			upgradeCommand(),
			rollbackCommand(),
			switchCommand(),
			configCommand(),
		},
	}
}

func dmesgCommand() *cli.Command {
	return &cli.Command{
		Name:   "dmesg",
		Usage:  "Show kernel ring buffer messages",
		Before: requirePermission("machine/dmesg", access.GetVerb),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "ctime",
				Aliases: []string{"T"},
				Usage:   "Print human readable timestamps",
			},
			&cli.StringFlag{
				Name:    "level",
				Aliases: []string{"l"},
				Usage:   "Restrict output to given levels (e.g. err,warn)",
			},
			&cli.BoolFlag{
				Name:    "follow",
				Aliases: []string{"w"},
				Usage:   "Wait for new messages",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Dmesg(ctx, c.Writer, c.Bool("ctime"), c.String("level"), c.Bool("follow"))
		},
	}
}

func logsCommand() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "Show system logs",
		ArgsUsage: fmt.Sprintf("[%s]", servicePlaceholder),
		Before:    requirePermission("machine/logs", access.GetVerb),
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "lines",
				Aliases: []string{"n"},
				Value:   100,
				Usage:   "Number of lines to show (-1 for all)",
			},
			&cli.BoolFlag{
				Name:    "follow",
				Aliases: []string{"f"},
				Usage:   "Follow log output",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Logs(ctx, c.Writer, c.Int("lines"), c.Bool("follow"), c.Args().First())
		},
	}
}

func infoCommand() *cli.Command {
	return &cli.Command{
		Name:   "info",
		Usage:  "Show machine info",
		Before: requirePermission("machine/info", access.GetVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Info(ctx, c.Writer)
		},
	}
}

func rebootCommand() *cli.Command {
	return &cli.Command{
		Name:   "reboot",
		Usage:  "Reboot the machine",
		Before: requirePermission("machine/reboot", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Reboot(ctx, c.Writer)
		},
	}
}

func upgradeCommand() *cli.Command {
	return &cli.Command{
		Name:  "upgrade",
		Usage: "Upgrade to the latest system image",
		Description: "Pulls the newest OS image with bootc and stages it for the next boot. " +
			"Run 'machine reboot' to apply it.",
		Before: requirePermission("machine/upgrade", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Upgrade(ctx, c.Writer)
		},
	}
}

func rollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "Roll back to the previous system image",
		Description: "Switches back to the previously staged OS image. " +
			"Run 'machine reboot' to apply it.",
		Before: requirePermission("machine/rollback", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Rollback(ctx, c.Writer)
		},
	}
}

func switchCommand() *cli.Command {
	return &cli.Command{
		Name:      "switch",
		Usage:     "Switch to a different system image",
		ArgsUsage: imagePlaceholder,
		Description: "Stages the given OS image for the next boot. " +
			"Run 'machine reboot' to apply it.",
		Before: requirePermission("machine/switch", access.InvokeVerb),
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
		Usage: "Manage machine config",
		Commands: []*cli.Command{
			applyConfigCommand(),
			patchConfigCommand(),
			showConfigCommand(),
		},
	}
}

func applyConfigCommand() *cli.Command {
	return &cli.Command{
		Name:   "apply",
		Usage:  "Apply machine config from stdin",
		Before: requirePermission("machine/config", access.UpdateVerb),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "reboot",
				Usage: "Reboot after the config is applied",
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

func patchConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "patch",
		Usage: "Patch machine config from stdin",
		Description: "Applies patches read from stdin to the machine config. " +
			"Each YAML document must be an RFC 7396 merge patch or an RFC 6902 JSON Patch." +
			"Documents are separated by '---' and applied in order.",
		Before: requirePermission("machine/config", access.UpdateVerb),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "reboot",
				Usage: "Reboot after the patch is applied",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			m := machine.New()
			if err := m.ApplyPatch(ctx, c.Reader, c.Writer); err != nil {
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

func showConfigCommand() *cli.Command {
	return &cli.Command{
		Name:   "show",
		Usage:  "Show the current machine config",
		Before: requirePermission("machine/config", access.GetVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().ShowConfig(ctx, c.Writer)
		},
	}
}
