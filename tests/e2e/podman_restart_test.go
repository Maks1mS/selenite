// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Regression test for podman-restart.service being enabled

// after a reboot, containers with `restart: always` must come back up
// without any manual intervention.
func TestPodmanRestartAlways(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)
	const stackName = "restart-always"
	const stack = `services:
  test:
    image: docker.io/library/busybox:latest
    command: sh -c 'echo start-log-test && while true; do date; sleep 1; done'
    restart: always
`

	run(t, "deploy stack", func(t *testing.T) {
		mustDeployStack(t, client, stackName, stack)
		out := mustRunCmd(t, client, "stack list")
		require.Contains(t, out, stackName)
	})

	run(t, "container is running", func(t *testing.T) {
		waitStackRunning(t, client, stackName)
	})

	run(t, "reboot", func(t *testing.T) {
		mustRunCmd(t, client, "machine reboot")
	})

	run(t, "container restarted automatically after reboot", func(t *testing.T) {
		client.reconnect(t, 120*time.Second)
		waitStackRunning(t, client, stackName)
	})
}
