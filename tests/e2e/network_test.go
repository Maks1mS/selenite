// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetwork(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)
	const networkName = "test-network"

	run(t, "list shows default networks", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network ls")
		assert.Contains(t, out, "NETWORK ID")
		assert.Contains(t, out, "NAME")
	})

	run(t, "create network", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network create "+networkName)
		assert.Contains(t, out, networkName)
	})

	run(t, "network appears in list", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network ls")
		assert.Contains(t, out, networkName)
	})

	run(t, "inspect network", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network inspect "+networkName)
		assert.Contains(t, out, `"name": "`+networkName+`"`)
	})

	run(t, "create network with options", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network create "+networkName+"-opts --subnet 10.99.0.0/24 --gateway 10.99.0.1 --internal")
		assert.Contains(t, out, networkName+"-opts")
	})

	run(t, "remove network with options", func(t *testing.T) {
		mustRunCmd(t, client, "infra network rm "+networkName+"-opts")
	})

	run(t, "remove network", func(t *testing.T) {
		mustRunCmd(t, client, "infra network rm "+networkName)
	})

	run(t, "network disappears from list", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra network ls")
		assert.NotContains(t, out, networkName)
	})

	run(t, "prune works", func(t *testing.T) {
		_ = mustRunCmd(t, client, "infra network prune")
	})
}
