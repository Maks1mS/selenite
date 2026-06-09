// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/infra/secrets"
)

var errSecretNameRequired = fmt.Errorf("secret name required")

func secretCommand() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "Manage podman secrets",
		Commands: []*cli.Command{
			secretCreateCommand(),
			secretListCommand(),
			secretRemoveCommand(),
		},
	}
}

func requireSecretName(ctx context.Context, c *cli.Command) (context.Context, error) {
	if c.NArg() == 0 {
		return ctx, errSecretNameRequired
	}
	return ctx, nil
}

func secretCreateCommand() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create secret",
		Before: beforeChecks(
			requireSecretName,
			requirePermission("infra/secret/{0}", access.CreateVerb),
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return secrets.New().Create(ctx, c.Args().First(), c.Reader, c.Writer)
		},
	}
}

func secretRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Usage:   "Remove secret",
		Aliases: []string{"rm"},
		Before: beforeChecks(
			requireSecretName,
			requirePermission("infra/secret/{0}", access.DeleteVerb),
		),
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
