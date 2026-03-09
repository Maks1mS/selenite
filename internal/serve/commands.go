package serve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/ssh"
	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/shell"
	"altlinux.space/alt-atomic-lab/selenite/internal/version"
)

func appCmd(sess ssh.Session) *cli.Command {
	root := &cli.Command{
		Name:      "-",
		UsageText: "admin [...]",
		Usage:     "Control your Selenite server",
		// CustomRootCommandHelpTemplate: "no help yet\n\n_",
		Commands: []*cli.Command{
			shellCommand(sess),
			versionCommand(),
		},
	}
	prepareCmdForWish(root, sess)
	return root
}

// ptyWriter translates \n to \r\n for raw PTY mode.
type ptyWriter struct {
	w io.Writer
}

func (p ptyWriter) Write(b []byte) (int, error) {
	replaced := bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	_, err := p.w.Write(replaced)
	return len(b), err
}

// sessWriter returns a PTY-aware writer for session stdout.
func sessWriter(sess ssh.Session) io.Writer {
	var w io.Writer = sess
	if _, _, active := sess.Pty(); active {
		w = ptyWriter{w}
	}
	return w
}

func prepareCmdForWish(cmd *cli.Command, sess ssh.Session) {
	cmd.Reader = sess
	var w io.Writer = sess.Stderr()
	if _, _, active := sess.Pty(); active {
		w = ptyWriter{w}
	}
	cmd.Writer = w
	cmd.ErrWriter = w
	cmd.CommandNotFound = func(ctx context.Context, c *cli.Command, s string) {
		_, _ = fmt.Fprintf(c.ErrWriter, "command not found: %s\n", s)
	}
	cmd.EnableShellCompletion = false
	for _, subCmd := range cmd.Commands {
		prepareCmdForWish(subCmd, sess)
	}
}

func shellCommand(sess ssh.Session) *cli.Command {
	return &cli.Command{
		Name:  "shell",
		Usage: "Start a shell session",
		Action: func(ctx context.Context, c *cli.Command) error {
			_, _, active := sess.Pty()
			if !active {
				return errors.New("requires an active PTY")
			}
			return shell.Shell(ctx, sess)
		},
	}
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print the version",
		Action: func(ctx context.Context, c *cli.Command) error {
			_, _ = fmt.Fprintln(c.Writer, version.FormatVersion())
			return nil
		},
	}
}
