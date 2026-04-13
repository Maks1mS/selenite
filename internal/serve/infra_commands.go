package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/access"
	"altlinux.space/alt-atomic-lab/selenite/internal/infra/volume"
)

func infraCommand() *cli.Command {
	return &cli.Command{
		Name: "infra",
		Commands: []*cli.Command{
			secretCommand(),
			volumeCommand(),
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
		Name:  "export",
		Usage: "Export volume",
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
		Name:  "import",
		Usage: "Import volume",
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
