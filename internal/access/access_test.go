package access

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
)

func cfg(roles []v1.Role, users []v1.User) v1.AccessConfig {
	return v1.AccessConfig{Roles: roles, Users: users}
}

func role(name string, rules ...v1.Rules) v1.Role {
	return v1.Role{Name: name, Rules: rules}
}

func rules(resources, verbs []string) v1.Rules {
	return v1.Rules{Resources: resources, Verbs: verbs}
}

func user(name string, roles ...string) v1.User {
	return v1.User{Name: name, Roles: roles}
}

func mustEnforcer(t *testing.T, c v1.AccessConfig) interface {
	Enforce(rvals ...any) (bool, error)
} {
	t.Helper()
	e, err := NewEnforcer(c)
	require.NoError(t, err)
	return e
}

func allow(t *testing.T, e interface {
	Enforce(rvals ...any) (bool, error)
}, sub, obj, act string,
) {
	t.Helper()
	ok, err := e.Enforce(sub, obj, act)
	require.NoError(t, err)
	require.True(t, ok, "expected %q to be allowed for %s %s", sub, act, obj)
}

func deny(t *testing.T, e interface {
	Enforce(rvals ...any) (bool, error)
}, sub, obj, act string,
) {
	t.Helper()
	ok, err := e.Enforce(sub, obj, act)
	require.NoError(t, err)
	require.False(t, ok, "expected %q to be denied for %s %s", sub, act, obj)
}

func TestNewEnforcerRootRole(t *testing.T) {
	e := mustEnforcer(t, cfg(nil, []v1.User{user("alice", "root")}))

	allow(t, e, "alice", "machine/info", "get")
	allow(t, e, "alice", "machine/reboot", "invoke")
	allow(t, e, "alice", "stack/anything", "delete")
	allow(t, e, "alice", "secret/foo", "create")
	allow(t, e, "alice", "*", "*")
}

func TestNewEnforcerCustomRole(t *testing.T) {
	r := role("viewer", rules([]string{"machine/info"}, []string{"get"}))
	e := mustEnforcer(t, cfg([]v1.Role{r}, []v1.User{user("alice", "viewer")}))

	allow(t, e, "alice", "machine/info", "get")

	deny(t, e, "alice", "machine/info", "invoke")
	deny(t, e, "alice", "machine/reboot", "invoke")
	deny(t, e, "alice", "stack/myapp", "create")
	deny(t, e, "alice", "secret/foo", "get")
}

func TestNewEnforcerUnknownUserDenied(t *testing.T) {
	r := role("viewer", rules([]string{"machine/info"}, []string{"get"}))
	e := mustEnforcer(t, cfg([]v1.Role{r}, []v1.User{user("alice", "viewer")}))

	deny(t, e, "bob", "machine/info", "get")
	deny(t, e, "", "machine/info", "get")
}

func TestNewEnforcerWildcardResourceMatch(t *testing.T) {
	r := role("deployer", rules([]string{"stack/*"}, []string{"create", "get"}))
	e := mustEnforcer(t, cfg([]v1.Role{r}, []v1.User{user("alice", "deployer")}))

	allow(t, e, "alice", "stack/myapp", "create")
	allow(t, e, "alice", "stack/other", "get")

	deny(t, e, "alice", "stack", "create")
	deny(t, e, "alice", "secret/foo", "create")
	deny(t, e, "alice", "stack/myapp", "delete") // wrong verb
}

func TestNewEnforcerMultipleRoles(t *testing.T) {
	reader := role("reader", rules([]string{"stack"}, []string{"list"}))
	deployer := role("deployer", rules([]string{"stack/*"}, []string{"create"}))

	e := mustEnforcer(t, cfg(
		[]v1.Role{reader, deployer},
		[]v1.User{
			user("alice", "reader", "deployer"),
			user("bob", "reader"),
		},
	))

	// alice has both roles
	allow(t, e, "alice", "stack", "list")
	allow(t, e, "alice", "stack/myapp", "create")

	// bob has only reader
	allow(t, e, "bob", "stack", "list")
	deny(t, e, "bob", "stack/myapp", "create")
}
