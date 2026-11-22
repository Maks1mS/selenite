// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/infra/image"
)

func imageCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "image",
		Usage: "Manage container images",
		Commands: []*cli.Command{
			imageLoadCommand(),
			imageListCommand(),
			imageRemoveCommand(),
			imagePruneCommand(),
		},
	}
	return cmd
}

var errImageNameRequired = fmt.Errorf("image name required")

func requireImageName(ctx context.Context, c *cli.Command) (context.Context, error) {
	if c.NArg() == 0 {
		return ctx, errImageNameRequired
	}
	return ctx, nil
}

func imageLoadCommand() *cli.Command {
	return &cli.Command{
		Name:  "load",
		Usage: "Load a container image from an image archive on stdin",
		Description: "Reads an image archive produced by 'docker save' or 'podman save' " +
			"(gzip-compressed archives are supported) from stdin and loads it into the " +
			"local image storage.",
		Before: requirePermission("infra/image", access.CreateVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return image.New().Load(ctx, c.Reader, c.Writer)
		},
	}
}

func imageListCommand() *cli.Command {
	return &cli.Command{
		Name:   "list",
		Usage:  "List container images",
		Before: requirePermission("infra/image", access.ListVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return image.New().List(ctx, c.Writer)
		},
	}
}

func imageRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove a container image",
		ArgsUsage: "<name>",
		Before: beforeChecks(
			requireImageName,
			requirePermission("infra/image/{0}", access.DeleteVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return image.New().Remove(ctx, c.Args().First(), c.Writer)
		},
	}
}

func imagePruneCommand() *cli.Command {
	return &cli.Command{
		Name:   "prune",
		Usage:  "Remove all unused images",
		Before: requirePermission("infra/image", access.DeleteVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return image.New().Prune(ctx, c.Writer)
		},
	}
}
