// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

// Regression test for tmpfiles.d/selenite.conf.
//
// It used to have a recursive `Z` rule on /var/lib/selenite-podman-executor.
// Since systemd-tmpfiles runs on every boot, that rule recursively chowned
// the whole tree, including podman's storage (breaking image layers that
// rootless podman intentionally leaves owned by subuid-mapped users).
func TestPodmanStoragePermissions(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)
	const stackName = "uid-test"
	const stack = `services:
  test:
    image: registry.altlinux.org/sisyphus/alt:latest
    command: sleep infinity
    restart: always
    healthcheck:
      test: ["CMD-SHELL", "[ \"$(stat -c '%U:%G' /var/cache/apt)\" = \"root:rpm\" ]"]
      interval: 30s
      timeout: 5s
      retries: 1
      start_period: 1s
`

	e2etest.Step(t, "deploy stack", func(t *testing.T) {
		out := client.Run(t, "stack list")
		require.Equal(t, "no stacks deployed\n", out)

		client.DeployStack(t, stackName, stack)
		out = client.Run(t, "stack list")
		require.Contains(t, out, stackName)

		time.Sleep(2 * time.Second)
		out = client.Run(t, "stack ps "+stackName)
		require.Contains(t, out, "(healthy)")
	})

	e2etest.Step(t, "reboot", func(t *testing.T) {
		client.Run(t, "machine reboot")
	})

	e2etest.Step(t, "stack healthy after reboot", func(t *testing.T) {
		client.Reconnect(t, 120*time.Second)
		time.Sleep(2 * time.Second)
		out := client.Run(t, "stack ps "+stackName)
		assert.Contains(t, out, "(healthy)")
	})
}
