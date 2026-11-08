// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

// Regression test for podman-restart.service being enabled

// after a reboot, containers with `restart: always` must come back up
// without any manual intervention.
func TestPodmanRestartAlways(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)
	const stackName = "restart-always"
	const stack = `services:
  test:
    image: registry.altlinux.org/sisyphus/distroless-toybox:latest
    command: sh -c 'echo start-log-test && while true; do date; sleep 1; done'
    restart: always
`

	e2etest.Step(t, "deploy stack", func(t *testing.T) {
		client.DeployStack(t, stackName, stack)
		out := client.Run(t, "stack list")
		require.Contains(t, out, stackName)
	})

	e2etest.Step(t, "container is running", func(t *testing.T) {
		client.WaitStackRunning(t, stackName)
	})

	e2etest.Step(t, "reboot", func(t *testing.T) {
		client.Run(t, "machine reboot")
	})

	e2etest.Step(t, "container restarted automatically after reboot", func(t *testing.T) {
		client.Reconnect(t, 120*time.Second)
		client.WaitStackRunning(t, stackName)
	})
}
