package serve

import (
	"context"
	"fmt"

	"github.com/charmbracelet/ssh"
	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/compose"
)

func stackCommand(sess ssh.Session) *cli.Command {
	cmd := &cli.Command{
		Name:  "stack",
		Usage: "Manage compose stacks",
		Commands: []*cli.Command{
			stackDeployCommand(sess),
			stackListCommand(),
			stackUpCommand(),
			stackDownCommand(),
			stackPsCommand(),
			stackLogsCommand(),
		},
	}
	return cmd
}

func stackDeployCommand(sess ssh.Session) *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "Deploy a stack from stdin (compose.yml)",
		ArgsUsage: "<name>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("stack name required")
			}
			return compose.NewManager().Deploy(ctx, c.Args().First(), sess, c.Writer)
		},
	}
}

func stackListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List all stacks",
		Action: func(ctx context.Context, c *cli.Command) error {
			names, err := compose.NewManager().List()
			if err != nil {
				return fmt.Errorf("failed to list stacks: %w", err)
			}
			if len(names) == 0 {
				_, _ = fmt.Fprintln(c.Writer, "no stacks deployed")
				return nil
			}
			for _, name := range names {
				_, _ = fmt.Fprintln(c.Writer, name)
			}
			return nil
		},
	}
}

func stackUpCommand() *cli.Command {
	return &cli.Command{
		Name:      "up",
		Usage:     "Start a stack",
		ArgsUsage: "<name>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("stack name required")
			}
			return compose.NewManager().Up(ctx, c.Args().First(), c.Writer)
		},
	}
}

func stackDownCommand() *cli.Command {
	return &cli.Command{
		Name:      "down",
		Usage:     "Stop a stack",
		ArgsUsage: "<name>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("stack name required")
			}
			return compose.NewManager().Down(ctx, c.Args().First(), c.Writer)
		},
	}
}

func stackPsCommand() *cli.Command {
	return &cli.Command{
		Name:      "ps",
		Usage:     "Show stack container status",
		ArgsUsage: "<name>",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("stack name required")
			}
			return compose.NewManager().Ps(ctx, c.Args().First(), c.Writer)
		},
	}
}

func stackLogsCommand() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "View stack logs",
		ArgsUsage: "<name> [service]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "follow",
				Aliases: []string{"f"},
				Usage:   "Follow log output",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return fmt.Errorf("stack name required")
			}
			return compose.NewManager().Logs(ctx, c.Args().Get(0), c.Bool("follow"), c.Args().Get(1), c.Writer)
		},
	}
}
