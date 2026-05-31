// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/access"
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
		Usage:  "Reboot",
		Before: requirePermission("machine/reboot", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Reboot(ctx, c.Writer)
		},
	}
}

func upgradeCommand() *cli.Command {
	return &cli.Command{
		Name:   "upgrade",
		Usage:  "Upgrade",
		Before: requirePermission("machine/upgrade", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return machine.New().Upgrade(ctx, c.Writer)
		},
	}
}

func rollbackCommand() *cli.Command {
	return &cli.Command{
		Name:   "rollback",
		Usage:  "Rollback",
		Before: requirePermission("machine/rollback", access.InvokeVerb),
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
		Before:    requirePermission("machine/switch", access.InvokeVerb),
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
		Name:   "apply",
		Usage:  "Apply machine config",
		Before: requirePermission("machine/config", access.UpdateVerb),
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
