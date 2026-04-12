package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/access"
	"altlinux.space/alt-atomic-lab/selenite/internal/secrets"
)

var errSecretNameRequired = fmt.Errorf("secret name required")

func secretCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "secret",
		Usage: "Manage podman secrets",
		Commands: []*cli.Command{
			secretCreateCommand(),
			secretListCommand(),
			secretRemoveCommand(),
		},
	}
	return cmd
}

func secretCreateCommand() *cli.Command {
	return &cli.Command{
		Name:   "create",
		Usage:  "Create secret",
		Before: requirePermissionForArg("infra/secret", access.CreateVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errSecretNameRequired
			}
			return secrets.New().Create(ctx, c.Args().First(), c.Reader, c.Writer)
		},
	}
}

func secretRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Usage:   "Remove secret",
		Aliases: []string{"rm"},
		Before:  requirePermissionForArg("infra/secret", access.DeleteVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errSecretNameRequired
			}
			return secrets.New().Rm(ctx, c.Args().First(), c.Writer)
		},
	}
}

func secretListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List secrets",
		Aliases: []string{"ls"},
		Before:  requirePermission("infra/secret", access.ListVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			return secrets.New().List(ctx, c.Writer)
		},
	}
}
