package serve

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	access "altlinux.space/alt-atomic-lab/selenite/internal/access2"
	v1 "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
)

const testUser = "alice"

// makeRBACCtx builds a context that mimics what the SSH session middleware sets.
func makeRBACCtx(t *testing.T, cfg v1.AccessConfig) context.Context {
	t.Helper()
	e, err := access.NewEnforcer(cfg)
	require.NoError(t, err)
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxRBACEnforcer, e)
	ctx = context.WithValue(ctx, ctxRBACUsername, testUser)
	return ctx
}

func TestCheckPermissionNilEnforcer(t *testing.T) {
	err := checkPermission(context.Background(), "machine/reboot", "invoke")
	require.NoError(t, err)
}

func TestCheckPermissionAllowed(t *testing.T) {
	cfg := v1.AccessConfig{
		Roles: []v1.Role{{
			Name:  "viewer",
			Rules: []v1.Rules{{Resources: []string{"machine/info"}, Verbs: []string{"get"}}},
		}},
		Users: []v1.User{{Name: testUser, Roles: []string{"viewer"}}},
	}
	ctx := makeRBACCtx(t, cfg)

	require.NoError(t, checkPermission(ctx, "machine/info", "get"))
}

func TestCheckPermissionDenied(t *testing.T) {
	cfg := v1.AccessConfig{
		Users: []v1.User{{Name: testUser, Roles: []string{}}},
	}
	ctx := makeRBACCtx(t, cfg)

	err := checkPermission(ctx, "machine/info", "get")
	require.ErrorContains(t, err, "permission denied:")
}

func TestRequirePermission(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		cfg := v1.AccessConfig{
			Roles: []v1.Role{{
				Name:  "viewer",
				Rules: []v1.Rules{{Resources: []string{"machine/info"}, Verbs: []string{"get"}}},
			}},
			Users: []v1.User{{Name: testUser, Roles: []string{"viewer"}}},
		}
		ctx := makeRBACCtx(t, cfg)
		before := requirePermission("machine/info", "get")
		_, err := before(ctx, &cli.Command{})
		require.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		ctx := makeRBACCtx(t, v1.AccessConfig{Users: []v1.User{{Name: testUser}}})
		before := requirePermission("machine/info", "get")
		_, err := before(ctx, &cli.Command{})
		require.ErrorContains(t, err, "permission denied:")
	})
}

func TestRequirePermissionForArg(t *testing.T) {
	// Build a minimal command tree so that positional args are parsed normally.
	makeApp := func() *cli.Command {
		return &cli.Command{
			Name: "test",
			Commands: []*cli.Command{
				{
					Name:   "deploy",
					Before: requirePermissionForArg("stack", "create"),
					Action: func(_ context.Context, _ *cli.Command) error { return nil },
				},
			},
		}
	}

	t.Run("allowed with matching arg resource", func(t *testing.T) {
		cfg := v1.AccessConfig{
			Roles: []v1.Role{{
				Name:  "deployer",
				Rules: []v1.Rules{{Resources: []string{"stack/myapp"}, Verbs: []string{"create"}}},
			}},
			Users: []v1.User{{Name: testUser, Roles: []string{"deployer"}}},
		}
		ctx := makeRBACCtx(t, cfg)
		err := makeApp().Run(ctx, []string{"test", "deploy", "myapp"})
		require.NoError(t, err)
	})

	t.Run("denied when arg resource does not match", func(t *testing.T) {
		cfg := v1.AccessConfig{
			Roles: []v1.Role{{
				Name:  "deployer",
				Rules: []v1.Rules{{Resources: []string{"stack/other"}, Verbs: []string{"create"}}},
			}},
			Users: []v1.User{{Name: testUser, Roles: []string{"deployer"}}},
		}
		ctx := makeRBACCtx(t, cfg)
		err := makeApp().Run(ctx, []string{"test", "deploy", "myapp"})
		require.ErrorContains(t, err, "permission denied:")
	})

	t.Run("wildcard resource allows any arg", func(t *testing.T) {
		cfg := v1.AccessConfig{
			Roles: []v1.Role{{
				Name:  "deployer",
				Rules: []v1.Rules{{Resources: []string{"stack/*"}, Verbs: []string{"create"}}},
			}},
			Users: []v1.User{{Name: testUser, Roles: []string{"deployer"}}},
		}
		ctx := makeRBACCtx(t, cfg)
		err := makeApp().Run(ctx, []string{"test", "deploy", "anything"})
		require.NoError(t, err)
	})
}
