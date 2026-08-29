// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func zramConfig(client *testClient, memory string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.publicKey)), "\n")
	return fmt.Sprintf(`
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - %q
memory:
%s`, keyStr, memory)
}

func TestZramConfig(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	run(t, "rejected when enabled without size", func(t *testing.T) {
		cfg := zramConfig(client, "  zram:\n    enabled: true\n")
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "rejected when invalid compression", func(t *testing.T) {
		cfg := zramConfig(client, `  zram:
    enabled: true
    size: ram/2
    compression: invalid
`)
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "apply with zram disabled is a no-op", func(t *testing.T) {
		cfg := zramConfig(client, `  zram:
    enabled: false
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
	})

	run(t, "apply enables zram and writes generator config", func(t *testing.T) {
		cfg := zramConfig(client, `  zram:
    enabled: true
    size: ram/4
    compression: zstd
    swap_priority: 100
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		out, err := runPtyCmd(t, client.ssh, "debug busybox -- cat /host/etc/systemd/zram-generator.conf")
		require.NoError(t, err)
		assert.Contains(t, out, "[zram0]")
		assert.Contains(t, out, "zram-size=ram/4")
		assert.Contains(t, out, "compression-algorithm=zstd")
		assert.Contains(t, out, "swap-priority=100")
	})

	run(t, "zram swap device is active after apply", func(t *testing.T) {
		out, err := runPtyCmd(t, client.ssh, "debug busybox -- sh -c \"chroot /host /usr/bin/systemctl is-active dev-zram0.swap\"")
		require.NoError(t, err)
		assert.Contains(t, out, "active")
	})

	run(t, "/proc/swaps shows zram0", func(t *testing.T) {
		out, err := runPtyCmd(t, client.ssh, "debug busybox -- cat /host/proc/swaps")
		require.NoError(t, err)
		assert.Contains(t, out, "/dev/zram0")
	})

	run(t, "re-apply with different parameters updates config", func(t *testing.T) {
		cfg := zramConfig(client, `  zram:
    enabled: true
    size: ram/2
    compression: lz4
    swap_priority: 50
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		out, err := runPtyCmd(t, client.ssh, "debug busybox -- cat /host/etc/systemd/zram-generator.conf")
		require.NoError(t, err)
		assert.Contains(t, out, "zram-size=ram/2")
		assert.Contains(t, out, "compression-algorithm=lz4")
		assert.Contains(t, out, "swap-priority=50")
	})

	run(t, "zram swap device is still active after re-apply", func(t *testing.T) {
		out, err := runPtyCmd(t, client.ssh, "debug busybox -- sh -c \"chroot /host /usr/bin/systemctl is-active dev-zram0.swap\"")
		require.NoError(t, err)
		assert.Contains(t, out, "active")
	})

	run(t, "disable zram removes generator config and stops swap", func(t *testing.T) {
		cfg := zramConfig(client, `  zram:
    enabled: false
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		out, err := runPtyCmd(t, client.ssh, "debug busybox -- sh -c \"test -f /host/etc/systemd/zram-generator.conf && echo exists || echo missing\"")
		require.NoError(t, err)
		assert.Contains(t, out, "missing")

		out, err = runPtyCmd(t, client.ssh, "debug busybox -- cat /host/proc/swaps")
		require.NoError(t, err)
		assert.NotContains(t, out, "zram0")
	})
}
