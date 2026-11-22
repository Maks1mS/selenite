// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/infra/volume"
)

func infraCommand() *cli.Command {
	return &cli.Command{
		Name:  "infra",
		Usage: "Manage secrets, volumes, networks and images",
		Commands: []*cli.Command{
			secretCommand(),
			volumeCommand(),
			networkCommand(),
			imageCommand(),
		},
	}
}

var errVolumeNameRequired = fmt.Errorf("volume name required")

func requireVolumeName(ctx context.Context, c *cli.Command) (context.Context, error) {
	if c.NArg() == 0 {
		return ctx, errVolumeNameRequired
	}
	return ctx, nil
}

func volumeCommand() *cli.Command {
	return &cli.Command{
		Name:  "volume",
		Usage: "Manage podman volumes",
		Commands: []*cli.Command{
			volumeExportCommand(),
			volumeImportCommand(),
			volumePruneCommand(),
		},
	}
}

func volumeExportCommand() *cli.Command {
	return &cli.Command{
		Name:      "export",
		Usage:     "Export a volume to stdout as a zstd stream",
		ArgsUsage: "<name>",
		Before: beforeChecks(
			requireVolumeName,
			requirePermission("infra/volume/{0}/data", access.GetVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return volume.New().Export(ctx, c.Args().First(), c.Writer)
		},
	}
}

func volumeImportCommand() *cli.Command {
	return &cli.Command{
		Name:        "import",
		Usage:       "Import a volume from a zstd stream on stdin",
		ArgsUsage:   "<name>",
		Description: "Creates the volume if it does not exist.",
		Before: beforeChecks(
			requireVolumeName,
			requirePermission("infra/volume/{0}/data", access.UpdateVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return volume.New().Import(ctx, c.Args().First(), c.Reader)
		},
	}
}

func volumePruneCommand() *cli.Command {
	return &cli.Command{
		Name:  "prune",
		Usage: "Remove all unused volumes",
		Before: beforeChecks(
			requirePermission("infra/volume", access.DeleteVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return volume.New().Prune(ctx, c.Writer)
		},
	}
}
