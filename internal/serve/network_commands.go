// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/infra/network"
)

var errNetworkNameRequired = fmt.Errorf("network name required")

func requireNetworkName(ctx context.Context, c *cli.Command) (context.Context, error) {
	if c.NArg() == 0 {
		return ctx, errNetworkNameRequired
	}
	return ctx, nil
}

func networkCommand() *cli.Command {
	return &cli.Command{
		Name:  "network",
		Usage: "Manage podman networks",
		Commands: []*cli.Command{
			networkListCommand(),
			networkCreateCommand(),
			networkRemoveCommand(),
			networkInspectCommand(),
			networkPruneCommand(),
		},
	}
}

func networkListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List networks",
		Aliases: []string{"ls"},
		Before:  requirePermission("infra/network", access.ListVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return network.New().List(ctx, c.Writer)
		},
	}
}

func networkCreateCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a network",
		ArgsUsage: "<name>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "driver",
				Aliases: []string{"d"},
				Usage:   "Driver to manage the network",
			},
			&cli.StringFlag{
				Name:  "subnet",
				Usage: "Subnet in CIDR format",
			},
			&cli.StringFlag{
				Name:  "gateway",
				Usage: "Gateway for the subnet",
			},
			&cli.BoolFlag{
				Name:  "internal",
				Usage: "Restrict external access from this network",
			},
			&cli.BoolFlag{
				Name:  "ipv6",
				Usage: "Enable IPv6 networking",
			},
		},
		Before: beforeChecks(
			requireNetworkName,
			requirePermission("infra/network/{0}", access.CreateVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			opts := network.CreateOptions{
				Driver:   c.String("driver"),
				Subnet:   c.String("subnet"),
				Gateway:  c.String("gateway"),
				Internal: c.Bool("internal"),
				IPv6:     c.Bool("ipv6"),
			}
			return network.New().Create(ctx, c.Args().First(), opts, c.Writer)
		},
	}
}

func networkRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Usage:   "Remove a network",
		Aliases: []string{"rm"},
		Before: beforeChecks(
			requireNetworkName,
			requirePermission("infra/network/{0}", access.DeleteVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errNetworkNameRequired
			}
			return network.New().Rm(ctx, c.Args().First(), c.Writer)
		},
	}
}

func networkInspectCommand() *cli.Command {
	return &cli.Command{
		Name:  "inspect",
		Usage: "Inspect a network",
		Before: beforeChecks(
			requireNetworkName,
			requirePermission("infra/network/{0}", access.GetVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errNetworkNameRequired
			}
			return network.New().Inspect(ctx, c.Args().First(), c.Writer)
		},
	}
}

func networkPruneCommand() *cli.Command {
	return &cli.Command{
		Name:  "prune",
		Usage: "Remove all unused networks",
		Before: beforeChecks(
			requirePermission("infra/network", access.DeleteVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return network.New().Prune(ctx, c.Writer)
		},
	}
}
