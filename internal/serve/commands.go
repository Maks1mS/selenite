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

const helpTemplate = `Selenite is an immutable server OS built around podman compose stacks.

USAGE:
   ssh <server> [command [options]]

COMMANDS:{{range .VisibleCommands}}
   {{$s := join .Names ", "}}{{$s}}{{"\t"}}{{.Usage}}{{end}}

Run 'ssh <server> help <command>' for more information about a command.
`

const subCmdHelpTemplate = `{{.Name}}{{if .Usage}} — {{.Usage}}{{end}}{{if .Description}}

{{wrap .Description 0}}{{end}}

USAGE:
   ssh <server> {{.Name}}{{if .ArgsUsage}} {{.ArgsUsage}}{{end}}{{if .VisibleFlags}} [options]{{end}}
{{- if .VisibleFlags}}

OPTIONS:{{range .VisibleFlags}}
   {{wrap .String 6}}{{end}}
{{- end}}
`

func appCmd(sess ssh.Session) *cli.Command {
	root := &cli.Command{
		Name:                          "-",
		UsageText:                     "ssh <server> [command [options]]",
		Usage:                         "Immutable server OS for managing podman compose stacks",
		CustomRootCommandHelpTemplate: helpTemplate,
		Commands: []*cli.Command{
			shellCommand(sess),
			stackCommand(sess),
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

func prepareCmdForWish(cmd *cli.Command, sess ssh.Session) {
	cli.HelpFlag = nil

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
		if subCmd.CustomHelpTemplate == "" {
			subCmd.CustomHelpTemplate = subCmdHelpTemplate
		}
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
