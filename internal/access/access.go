// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package access

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	v1 "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
)

type AccessVerb int

const (
	UnknownVerb AccessVerb = iota
	GetVerb
	CreateVerb
	UpdateVerb
	DeleteVerb
	ListVerb
	InvokeVerb
)

func (d AccessVerb) String() string {
	switch d {
	case GetVerb:
		return "get"
	case CreateVerb:
		return "create"
	case UpdateVerb:
		return "update"
	case DeleteVerb:
		return "delete"
	case ListVerb:
		return "list"
	case InvokeVerb:
		return "invoke"
	case UnknownVerb:
		fallthrough
	default:
		return "unknown"
	}
}

const modelText = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && keyMatch(r.act, p.act)
`

// NewEnforcer builds a Casbin enforcer from the given AccessConfig.
// The root role always has full access.
func NewEnforcer(cfg v1.AccessConfig) (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("casbin model: %w", err)
	}

	e, err := casbin.NewEnforcer(m, false)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}

	if _, err := e.AddPolicy("root", "*", "*"); err != nil {
		return nil, fmt.Errorf("add root policy: %w", err)
	}

	for _, role := range cfg.Roles {
		for _, rule := range role.Rules {
			for _, resource := range rule.Resources {
				for _, verb := range rule.Verbs {
					if _, err := e.AddPolicy(role.Name, resource, verb); err != nil {
						return nil, fmt.Errorf("add policy %s %s %s: %w", role.Name, resource, verb, err)
					}
				}
			}
		}
	}

	for _, user := range cfg.Users {
		for _, roleName := range user.Roles {
			if _, err := e.AddGroupingPolicy(user.Name, roleName); err != nil {
				return nil, fmt.Errorf("add grouping policy %s %s: %w", user.Name, roleName, err)
			}
		}
	}

	return e, nil
}
