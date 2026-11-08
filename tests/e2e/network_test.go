// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

func TestNetwork(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)
	const networkName = "test-network"

	e2etest.Step(t, "list shows default networks", func(t *testing.T) {
		out := client.Run(t, "infra network ls")
		assert.Contains(t, out, "NETWORK ID")
		assert.Contains(t, out, "NAME")
	})

	e2etest.Step(t, "create network", func(t *testing.T) {
		out := client.Run(t, "infra network create "+networkName)
		assert.Contains(t, out, networkName)
	})

	e2etest.Step(t, "network appears in list", func(t *testing.T) {
		out := client.Run(t, "infra network ls")
		assert.Contains(t, out, networkName)
	})

	e2etest.Step(t, "inspect network", func(t *testing.T) {
		out := client.Run(t, "infra network inspect "+networkName)
		assert.Contains(t, out, `"name": "`+networkName+`"`)
	})

	e2etest.Step(t, "create network with options", func(t *testing.T) {
		out := client.Run(t, "infra network create "+networkName+"-opts --subnet 10.99.0.0/24 --gateway 10.99.0.1 --internal")
		assert.Contains(t, out, networkName+"-opts")
	})

	e2etest.Step(t, "remove network with options", func(t *testing.T) {
		client.Run(t, "infra network rm "+networkName+"-opts")
	})

	e2etest.Step(t, "remove network", func(t *testing.T) {
		client.Run(t, "infra network rm "+networkName)
	})

	e2etest.Step(t, "network disappears from list", func(t *testing.T) {
		out := client.Run(t, "infra network ls")
		assert.NotContains(t, out, networkName)
	})

	e2etest.Step(t, "prune works", func(t *testing.T) {
		_ = client.Run(t, "infra network prune")
	})
}
