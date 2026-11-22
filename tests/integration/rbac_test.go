// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build integration

package integration

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"altlinux.space/alt-atomic/selenite/internal/access"
	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic/selenite/internal/serve"
)

// commandRBACCase describes an RBAC-protected CLI command and the permission it requires.
type commandRBACCase struct {
	cliArgs  []string
	resource string
	verb     string
}

// protectedCommands lists every RBAC-protected CLI command.
var protectedCommands = []commandRBACCase{
	// machine
	{[]string{"", "machine", "info"}, "machine/info", "get"},
	{[]string{"", "machine", "reboot"}, "machine/reboot", "invoke"},
	{[]string{"", "machine", "upgrade"}, "machine/upgrade", "invoke"},
	{[]string{"", "machine", "rollback"}, "machine/rollback", "invoke"},
	{[]string{"", "machine", "switch", "myimage"}, "machine/switch", "invoke"},
	{[]string{"", "machine", "config", "apply"}, "machine/config", "update"},
	{[]string{"", "machine", "config", "patch"}, "machine/config", "update"},
	{[]string{"", "machine", "config", "show"}, "machine/config", "get"},
	{[]string{"", "machine", "logs"}, "machine/logs", "get"},
	{[]string{"", "debug", "busybox"}, "debug", "invoke"},
	// stack
	{[]string{"", "stack", "deploy", "myapp"}, "stack/myapp", "create"},
	{[]string{"", "stack", "list"}, "stack", "list"},
	{[]string{"", "stack", "up", "myapp"}, "stack/myapp", "update"},
	{[]string{"", "stack", "down", "myapp"}, "stack/myapp", "update"},
	{[]string{"", "stack", "remove", "myapp"}, "stack/myapp", "delete"},
	{[]string{"", "stack", "ps", "myapp"}, "stack/myapp", "get"},
	{[]string{"", "stack", "logs", "myapp"}, "stack/myapp", "get"},
	// secret
	{[]string{"", "infra", "secret", "create", "mysecret"}, "infra/secret/mysecret", "create"},
	{[]string{"", "infra", "secret", "remove", "mysecret"}, "infra/secret/mysecret", "delete"},
	{[]string{"", "infra", "secret", "list"}, "infra/secret", "list"},
	// volume
	{[]string{"", "infra", "volume", "export", "myvolume"}, "infra/volume/myvolume/data", "get"},
	{[]string{"", "infra", "volume", "import", "myvolume"}, "infra/volume/myvolume/data", "update"},
	{[]string{"", "infra", "volume", "prune"}, "infra/volume", "delete"},
	// network
	{[]string{"", "infra", "network", "list"}, "infra/network", "list"},
	{[]string{"", "infra", "network", "create", "mynet"}, "infra/network/mynet", "create"},
	{[]string{"", "infra", "network", "remove", "mynet"}, "infra/network/mynet", "delete"},
	{[]string{"", "infra", "network", "inspect", "mynet"}, "infra/network/mynet", "get"},
	{[]string{"", "infra", "network", "prune"}, "infra/network", "delete"},
	// image
	{[]string{"", "infra", "image", "load"}, "infra/image", "create"},
	{[]string{"", "infra", "image", "list"}, "infra/image", "list"},
	{[]string{"", "infra", "image", "remove", "myimage"}, "infra/image/myimage", "delete"},
	{[]string{"", "infra", "image", "prune"}, "infra/image", "delete"},
}

// rbacCtx builds a context that mimics what the SSH session middleware injects.
func rbacCtx(t *testing.T, cfg v1.AccessConfig, username string) context.Context {
	t.Helper()
	e, err := access.NewEnforcer(cfg)
	require.NoError(t, err)
	return serve.WithRBACSession(context.Background(), e, username)
}

// noAccessCfg returns an AccessConfig where the user exists but has no roles.
func noAccessCfg(user string) v1.AccessConfig {
	return v1.AccessConfig{
		Users: []v1.User{{Name: user, Roles: []string{}}},
	}
}

// exactPermCfg returns an AccessConfig granting user exactly one resource+verb.
func exactPermCfg(user, resource, verb string) v1.AccessConfig {
	return v1.AccessConfig{
		Roles: []v1.Role{{
			Name:  "test-role",
			Rules: []v1.Rules{{Resources: []string{resource}, Verbs: []string{verb}}},
		}},
		Users: []v1.User{{Name: user, Roles: []string{"test-role"}}},
	}
}

// newTestApp returns AppCmd(nil) with writers set to io.Discard.
func newTestApp() *cli.Command {
	app := serve.AppCmd(nil)
	configureTestAppRecursive(app)
	return app
}

func configureTestAppRecursive(app *cli.Command) {
	app.Writer = io.Discard
	app.ErrWriter = io.Discard
	app.Reader = strings.NewReader("")
	for _, sub := range app.Commands {
		configureTestAppRecursive(sub)
	}
}

// isRBACDenial reports whether err is an RBAC permission denial from checkPermission.
// The format is "permission denied: <verb> <resource>"
func isRBACDenial(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "permission denied:")
}

// TestRBACCommandDeniedWithoutPermission verifies that every RBAC-protected command
// rejects a user who has no permissions at all.
func TestRBACCommandDeniedWithoutPermission(t *testing.T) {
	for _, tc := range protectedCommands {
		name := strings.Join(tc.cliArgs[1:], " ")
		t.Run(name, func(t *testing.T) {
			ctx := rbacCtx(t, noAccessCfg("alice"), "alice")
			err := newTestApp().Run(ctx, tc.cliArgs)
			require.True(t, isRBACDenial(err),
				"expected RBAC denial, got: %v", err)
		})
	}
}

// TestRBACCommandAllowedWithExactPermission verifies that each command checks the
// correct resource and verb. A user granted only {tc.resource, tc.verb} should pass
// the Before hook for that specific command.
func TestRBACCommandAllowedWithExactPermission(t *testing.T) {
	for _, tc := range protectedCommands {
		name := strings.Join(tc.cliArgs[1:], " ")
		t.Run(name, func(t *testing.T) {
			ctx := rbacCtx(t, exactPermCfg("alice", tc.resource, tc.verb), "alice")
			err := newTestApp().Run(ctx, tc.cliArgs)
			// The Action may fail for unrelated reasons.
			// The only unacceptable outcome is an RBAC denial.
			require.False(t, isRBACDenial(err),
				"unexpected RBAC denial with exact permission: %v", err)
		})
	}
}

// TestRBACRootBypassesAllChecks verifies that a user with the root role is never
// blocked by RBAC for any of the protected commands.
func TestRBACRootBypassesAllChecks(t *testing.T) {
	cfg := v1.AccessConfig{
		Users: []v1.User{{Name: "admin", Roles: []string{"root"}}},
	}
	ctx := rbacCtx(t, cfg, "admin")

	for _, tc := range protectedCommands {
		name := strings.Join(tc.cliArgs[1:], " ")
		t.Run(name, func(t *testing.T) {
			err := newTestApp().Run(ctx, tc.cliArgs)
			require.False(t, isRBACDenial(err),
				"root should not be denied: %v", err)
		})
	}
}
