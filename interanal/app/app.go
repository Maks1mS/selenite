package app

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func Run(ctx context.Context) error {
	rootCmd := &cli.Command{
		Name: "selenite-controller",
	}
	if err := rootCmd.Run(ctx, os.Args); err != nil {
		return fmt.Errorf("failed to run root command: %w", err)
	}
	return nil
}
