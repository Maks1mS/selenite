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

	"charm.land/ssh"
	"github.com/casbin/casbin/v3"
	"github.com/urfave/cli/v3"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic/selenite/internal/access"
	"altlinux.space/alt-atomic/selenite/internal/debug"
	"altlinux.space/alt-atomic/selenite/internal/machine"
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

func subCmdHelpTemplate(path string) string {
	return path + `{{if .Usage}} — {{.Usage}}{{end}}{{if .Description}}

{{wrap .Description 0}}{{end}}

USAGE:
   ssh <server> ` + path + `{{if .VisibleCommands}} <command> [command options]{{end}}{{if .ArgsUsage}} {{.ArgsUsage}}{{end}}{{if .VisibleFlags}} [options]{{end}}
{{- if .VisibleCommands}}

COMMANDS:{{range .VisibleCommands}}
   {{$s := join .Names ", "}}{{$s}}{{"\t"}}{{.Usage}}{{end}}
{{- end}}
{{- if .VisibleFlags}}

OPTIONS:{{range .VisibleFlags}}
   {{wrap .String 6}}{{end}}
{{- end}}
`
}

// helpCommand returns the help command used instead of the built-in cli help.
func helpCommand() *cli.Command {
	return &cli.Command{
		Name:      "help",
		Aliases:   []string{"h"},
		Usage:     "Show help for a command",
		ArgsUsage: "[command ...]",
		HideHelp:  true,
		Action: func(ctx context.Context, c *cli.Command) error {
			target := c.Root()
			if lineage := c.Lineage(); len(lineage) > 1 {
				target = lineage[1]
			}
			for _, name := range c.Args().Slice() {
				sub := target.Command(name)
				if sub == nil {
					return cli.ShowCommandHelp(ctx, target, name)
				}
				target = sub
			}
			if target.Root() == target {
				return cli.ShowRootCommandHelp(target)
			}
			return cli.ShowSubcommandHelp(target)
		},
	}
}

func AppCmd(sess ssh.Session) *cli.Command {
	root := &cli.Command{
		Name:                          "Selenite",
		UsageText:                     "ssh <server> [command [options]]",
		Usage:                         "Immutable server OS for managing podman compose stacks",
		CustomRootCommandHelpTemplate: helpTemplate,
		Commands: []*cli.Command{
			stackCommand(),
			infraCommand(),
			machineCommands(),
			debugCommand(sess),
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
	prepareCmdForWish(root, sess, "")
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

func prepareCmdForWish(cmd *cli.Command, sess ssh.Session, path string) {
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
	// The default help for commands with subcommands ignores CustomHelpTemplate
	cli.ShowSubcommandHelp = func(c *cli.Command) error {
		if c.CustomHelpTemplate != "" {
			cli.HelpPrinter(c.Root().Writer, c.CustomHelpTemplate, c)
			return nil
		}
		return cli.DefaultShowSubcommandHelp(c)
	}
	cmd.CommandNotFound = func(ctx context.Context, c *cli.Command, s string) {
		_, _ = fmt.Fprintf(c.ErrWriter, "command not found: %s\n", s)
	}
	cmd.EnableShellCompletion = false
	if !cmd.HideHelp {
		cmd.Commands = append(cmd.Commands, helpCommand())
	}
	for _, subCmd := range cmd.Commands {
		subPath := subCmd.Name
		if path != "" {
			subPath = path + " " + subCmd.Name
		}
		if subCmd.CustomHelpTemplate == "" {
			subCmd.CustomHelpTemplate = subCmdHelpTemplate(subPath)
		}
		prepareCmdForWish(subCmd, sess, subPath)
	}
}

func debugCommand(sess ssh.Session) *cli.Command {
	return &cli.Command{
		Name:        "debug",
		Usage:       "Start a privileged debug container",
		ArgsUsage:   "<image> [command...]",
		Description: "Runs the given image in a privileged container with the host root filesystem mounted at /host.",
		Before:      requirePermission("debug", access.InvokeVerb),
		Action: func(ctx context.Context, c *cli.Command) error {
			if sess == nil {
				return errors.New("requires an active PTY")
			}
			_, _, active := sess.Pty()
			if !active {
				return errors.New("requires an active PTY")
			}
			image := c.Args().Get(0)
			if image == "" {
				return errors.New("image is required")
			}
			var args []string
			if c.NArg() > 1 {
				args = c.Args().Slice()[1:]
			}
			return debug.Run(ctx, sess, image, args)
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
		Usage: "Provision the machine on first boot",
		Description: "Registers the SSH public key of the connecting user as the initial root user. " +
			"Only available before the machine has been provisioned.",
		Action: func(ctx context.Context, c *cli.Command) error {
			isProvision, _ := ctx.Value(ctxIsProvisionMode).(bool)
			if !isProvision {
				return fmt.Errorf("provision: machine is already provisioned")
			}

			keyStr := string(bytes.TrimRight(
				gossh.MarshalAuthorizedKey(sess.PublicKey()), "\n",
			))
			return machine.New().Provision(ctx, c.Writer, keyStr)
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
