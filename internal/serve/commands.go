// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/casbin/casbin/v2"
	"github.com/charmbracelet/ssh"
	"github.com/urfave/cli/v3"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic/selenite/internal/access"
	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic/selenite/internal/shell"
)

type ctxKey string

const (
	ctxIsProvisionMode ctxKey = "is-provision-mode"
	ctxRBACEnforcer    ctxKey = "rbac-enforcer"
	ctxRBACUsername    ctxKey = "rbac-username"
)

// WithRBACSession returns ctx with RBAC session values set.
// Used by the SSH middleware and test helpers.
func WithRBACSession(ctx context.Context, enforcer *casbin.Enforcer, username string) context.Context {
	ctx = context.WithValue(ctx, ctxIsProvisionMode, false)
	ctx = context.WithValue(ctx, ctxRBACEnforcer, enforcer)
	ctx = context.WithValue(ctx, ctxRBACUsername, username)
	return ctx
}

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

func AppCmd(sess ssh.Session) *cli.Command {
	root := &cli.Command{
		Name:                          "Selenite",
		UsageText:                     "ssh <server> [command [options]]",
		Usage:                         "Immutable server OS for managing podman compose stacks",
		CustomRootCommandHelpTemplate: helpTemplate,
		Commands: []*cli.Command{
			stackCommand(sess),
			infraCommand(),
			machineCommands(),
			shellCommand(sess),
			provisionCommand(sess),
			// versionCommand(),
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			isProvision, _ := ctx.Value(ctxIsProvisionMode).(bool)
			if !isProvision {
				return ctx, nil
			}

			if c.Args().Get(0) == "provision" {
				return ctx, nil
			}

			return ctx, fmt.Errorf("provision mode: only 'provision' command is allowed")
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
	if sess != nil {
		var stdout io.Writer = sess
		var stderr io.Writer = sess.Stderr()
		if _, _, active := sess.Pty(); active {
			stdout = ptyWriter{stdout}
			stderr = ptyWriter{stderr}
		}
		cmd.Reader = sess
		cmd.Writer = stdout
		cmd.ErrWriter = stderr
	}
	cli.HelpFlag = nil
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

func beforeChecks(funcs ...cli.BeforeFunc) cli.BeforeFunc {
	return func(ctx context.Context, c *cli.Command) (context.Context, error) {
		var err error
		for _, f := range funcs {
			ctx, err = f(ctx, c)
			if err != nil {
				return ctx, err
			}
		}
		return ctx, nil
	}
}

func provisionCommand(sess ssh.Session) *cli.Command {
	return &cli.Command{
		Name:  "provision",
		Usage: "Provision",
		Action: func(ctx context.Context, c *cli.Command) error {
			isProvision, _ := ctx.Value(ctxIsProvisionMode).(bool)
			if !isProvision {
				return fmt.Errorf("provision: machine is already provisioned")
			}

			store := v1.ConfigStore{}
			cfg, err := store.Load()
			if err != nil {
				return fmt.Errorf("provision: load config: %w", err)
			}

			keyStr := string(bytes.TrimRight(
				gossh.MarshalAuthorizedKey(sess.PublicKey()), "\n",
			))
			cfg.Access.Users = append(cfg.Access.Users, v1.User{
				Name:  "provisioner",
				Roles: []string{"root"},
				Keys:  []string{keyStr},
			})
			return store.Save(cfg)
		},
	}
}

// checkPermission checks whether the current user has permission for the given resource and verb.
// If no enforcer is set in context (provision mode), access is always granted.
func checkPermission(ctx context.Context, resource string, verb access.AccessVerb) error {
	enforcer, _ := ctx.Value(ctxRBACEnforcer).(*casbin.Enforcer)
	if enforcer == nil {
		return nil
	}
	username, _ := ctx.Value(ctxRBACUsername).(string)
	ok, err := enforcer.Enforce(username, resource, verb.String())
	if err != nil {
		return fmt.Errorf("rbac error: %w", err)
	}
	if !ok {
		return fmt.Errorf("permission denied: %s %s", verb.String(), resource)
	}
	return nil
}

var placeholderRe = regexp.MustCompile(`\{\d+\}`)

func requirePermission(template string, verb access.AccessVerb) cli.BeforeFunc {
	return func(ctx context.Context, c *cli.Command) (context.Context, error) {
		args := c.Args().Slice()
		resource := placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
			idxStr := match[1 : len(match)-1] // strip { }
			idx, err := strconv.Atoi(idxStr)
			if err != nil || idx >= len(args) {
				return "*"
			}
			return args[idx]
		})
		return ctx, checkPermission(ctx, resource, verb)
	}
}
