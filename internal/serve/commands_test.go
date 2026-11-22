// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// runHelp runs the SSH CLI app with the given arguments and returns its output.
func runHelp(t *testing.T, args ...string) string {
	t.Helper()
	app := AppCmd(nil)
	out := &bytes.Buffer{}
	setWritersRecursive(app, out)
	err := app.Run(context.Background(), append([]string{""}, args...))
	require.NoError(t, err)
	return out.String()
}

func setWritersRecursive(cmd *cli.Command, w io.Writer) {
	cmd.Writer = w
	cmd.ErrWriter = w
	cmd.Reader = &bytes.Buffer{}
	for _, sub := range cmd.Commands {
		setWritersRecursive(sub, w)
	}
}

func TestHelpRootShowsCommandList(t *testing.T) {
	out := runHelp(t, "help")

	require.Contains(t, out, "Selenite is an immutable server OS built around podman compose stacks")
	require.Contains(t, out, "Manage compose stacks")
	require.Contains(t, out, "Manage secrets, volumes, networks and images")
	require.Contains(t, out, "Manage machine lifecycle and configuration")
	require.Contains(t, out, "Start a privileged debug container")
	require.Contains(t, out, "Provision the machine on first boot")
	require.Contains(t, out, "Run 'ssh <server> help <command>' for more information about a command.")
}

func TestHelpGroupListsSubcommands(t *testing.T) {
	out := runHelp(t, "help", "stack")

	require.Contains(t, out, "stack — Manage compose stacks")
	require.Contains(t, out, "ssh <server> stack <command> [command options]")
	require.Contains(t, out, "COMMANDS:")
	require.Contains(t, out, "Create or update a stack from stdin (compose.yml)")
	require.Contains(t, out, "Show stack logs")
}

func TestHelpIsConsistentAcrossInvocations(t *testing.T) {
	require.Equal(t, runHelp(t, "help", "stack"), runHelp(t, "stack", "help"))
	require.Equal(t, runHelp(t, "help", "machine", "config"), runHelp(t, "machine", "config", "help"))
}

func TestHelpLeafShowsFullPathAndArguments(t *testing.T) {
	out := runHelp(t, "help", "stack", "deploy")

	require.Contains(t, out, "stack deploy — Create or update a stack from stdin (compose.yml)")
	require.Contains(t, out, "ssh <server> stack deploy <name>")
	require.Contains(t, out, "Writes the compose.yml read from stdin and brings the stack up.")
}

func TestHelpLeafShowsFlagDescriptions(t *testing.T) {
	out := runHelp(t, "help", "machine", "config", "apply")

	require.Contains(t, out, "ssh <server> machine config apply [options]")
	require.Contains(t, out, "--reboot")
	require.Contains(t, out, "Reboot after the config is applied")
}

func TestHelpShowsOptionalArguments(t *testing.T) {
	out := runHelp(t, "help", "machine", "logs")

	require.Contains(t, out, "ssh <server> machine logs [<service>]")
}

func TestHelpDescribesStdinCommands(t *testing.T) {
	out := runHelp(t, "help", "infra", "secret", "create")
	require.Contains(t, out, "ssh <server> infra secret create <name>")
	require.Contains(t, out, "Create a secret from stdin")

	out = runHelp(t, "help", "infra", "volume", "export")
	require.Contains(t, out, "ssh <server> infra volume export <name>")

	out = runHelp(t, "help", "infra", "network", "inspect")
	require.Contains(t, out, "ssh <server> infra network inspect <name>")
}

func TestHelpDescribesImageCommands(t *testing.T) {
	out := runHelp(t, "help", "infra", "image")
	require.Contains(t, out, "infra image — Manage container images")
	require.Contains(t, out, "Load a container image from an image archive on stdin")

	out = runHelp(t, "help", "infra", "image", "load")
	require.Contains(t, out, "ssh <server> infra image load")
	require.Contains(t, out, "'docker save' or 'podman save'")

	out = runHelp(t, "help", "infra", "image", "remove")
	require.Contains(t, out, "ssh <server> infra image remove <name>")
}

func TestHelpUnknownCommand(t *testing.T) {
	out := runHelp(t, "help", "nosuch")
	require.Contains(t, out, "command not found: nosuch")
}
