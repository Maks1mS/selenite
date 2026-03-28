package serve

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/access/keys"
)

func accessCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "access",
		Usage: "Manage access",
		Commands: []*cli.Command{
			keysCommand(),
		},
	}
	return cmd
}

func keysCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "keys",
		Usage: "Manage keys",
		Commands: []*cli.Command{
			keysListCommand(),
			keysAddCommand(),
			keysRemoveCommand(),
		},
	}
	return cmd
}

func keysListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List keys",
		Action: func(ctx context.Context, c *cli.Command) error {
			keys, err := keys.NewKeysManager().List()
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}

			if len(keys) == 0 {
				_, _ = fmt.Fprintln(c.Writer, "no keys")
			}

			for _, key := range keys {
				_, _ = fmt.Fprintln(c.Writer, key.ToString())
			}

			return nil
		},
	}
}

func keysAddCommand() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "Add key",
		Action: func(ctx context.Context, c *cli.Command) error {
			buf := new(strings.Builder)
			_, err := io.Copy(buf, c.Reader)
			if err != nil {
				return fmt.Errorf("failed to read key from stdin: %w", err)
			}
			err = keys.NewKeysManager().Add(buf.String())
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}

			return nil
		},
	}
}

func keysRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:  "remove",
		Usage: "Remove key",
		Action: func(ctx context.Context, c *cli.Command) error {
			err := keys.NewKeysManager().Remove(c.Args().Get(0))
			if err != nil {
				return fmt.Errorf("failed to remove key: %w", err)
			}

			return nil
		},
	}
}
