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
)

// Regression test for tmpfiles.d/selenite.conf.
//
// It used to have a recursive `Z` rule on /var/lib/selenite-podman-executor.
// Since systemd-tmpfiles runs on every boot, that rule recursively chowned
// the whole tree, including podman's storage (breaking image layers that
// rootless podman intentionally leaves owned by subuid-mapped users).
func TestPodmanStoragePermissions(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)
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

	run(t, "deploy stack", func(t *testing.T) {
		out := mustRunCmd(t, client, "stack list")
		require.Equal(t, "no stacks deployed\n", out)

		mustDeployStack(t, client, stackName, stack)
		out = mustRunCmd(t, client, "stack list")
		require.Contains(t, out, stackName)

		time.Sleep(2 * time.Second)
		out = mustRunCmd(t, client, "stack ps "+stackName)
		require.Contains(t, out, "(healthy)")
	})

	run(t, "reboot", func(t *testing.T) {
		mustRunCmd(t, client, "machine reboot")
	})

	run(t, "stack healthy after reboot", func(t *testing.T) {
		client.reconnect(t, 120*time.Second)
		time.Sleep(2 * time.Second)
		mustRunCmd(t, client, "stack up "+stackName)
		time.Sleep(2 * time.Second)
		out := mustRunCmd(t, client, "stack ps "+stackName)
		assert.Contains(t, out, "(healthy)")
	})
}
