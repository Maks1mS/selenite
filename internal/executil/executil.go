package executil

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type Option func(*exec.Cmd)

func WithStdout(w io.Writer) Option         { return func(c *exec.Cmd) { c.Stdout = w } }
func WithStderr(w io.Writer) Option         { return func(c *exec.Cmd) { c.Stderr = w } }
func WithCombinedOutput(w io.Writer) Option { return func(c *exec.Cmd) { c.Stdout = w; c.Stderr = w } }
func WithStdin(r io.Reader) Option          { return func(c *exec.Cmd) { c.Stdin = r } }
func WithDir(dir string) Option             { return func(c *exec.Cmd) { c.Dir = dir } }
func WithEnv(k, v string) Option {
	return func(c *exec.Cmd) { c.Env = append(c.Environ(), k+"="+v) }
}

type Runner interface {
	Run(ctx context.Context, name string, args []string, opts ...Option) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, opts ...Option) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	for _, o := range opts {
		o(cmd)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	return nil
}
