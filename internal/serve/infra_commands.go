package serve

import "github.com/urfave/cli/v3"

func infraCommand() *cli.Command {
	return &cli.Command{
		Name: "infra",
		Commands: []*cli.Command{
			secretCommand(),
		},
	}
}
