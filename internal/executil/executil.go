package executil

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, w io.Writer, name string, args ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, w io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	return nil
}
