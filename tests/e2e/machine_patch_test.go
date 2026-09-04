// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	gossh "golang.org/x/crypto/ssh"
)

func TestMachinePatch(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	// First, apply a base config so we have a known state to patch.
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.publicKey)), "\n")
	baseCfg := `
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - ` + `"` + keyStr + `"
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: "80"
`
	mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(baseCfg))

	run(t, "merge patch adds new sysctl param", func(t *testing.T) {
		patch := `
sysctl:
  params:
    vm.max_map_count: "262144"
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.Contains(t, out, "vm.max_map_count")
		assert.Contains(t, out, "262144")
		// Original param must survive merge patch.
		assert.Contains(t, out, "net.ipv4.ip_unprivileged_port_start")
	})

	run(t, "merge patch overrides existing value", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: "1024"
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.Contains(t, out, "1024")
	})

	run(t, "json patch (RFC 6902) replaces a value", func(t *testing.T) {
		patch := `
- op: replace
  path: /sysctl/params/net.ipv4.ip_unprivileged_port_start
  value: "80"
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.Contains(t, out, "net.ipv4.ip_unprivileged_port_start")
		assert.Contains(t, out, ": \"80\"")
	})

	run(t, "json patch (RFC 6902) adds a new field", func(t *testing.T) {
		patch := `
- op: add
  path: /sysctl/params/net.ipv4.tcp_syncookies
  value: "1"
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.Contains(t, out, "tcp_syncookies")
		assert.Contains(t, out, "\"1\"")
	})

	run(t, "json patch (RFC 6902) removes a field", func(t *testing.T) {
		patch := `
- op: remove
  path: /sysctl/params/net.ipv4.tcp_syncookies
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.NotContains(t, out, "tcp_syncookies")
	})

	run(t, "merge patch removes field with null", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: null
`
		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(patch))

		out := mustRunCmd(t, client, "machine config show")
		assert.NotContains(t, out, "net.ipv4.ip_unprivileged_port_start")
	})

	run(t, "multiple patches applied sequentially", func(t *testing.T) {
		patch1 := `
sysctl:
  params:
    vm.swappiness: "60"
`
		patch2 := `
- op: add
  path: /sysctl/params/vm.dirty_ratio
  value: "20"
`
		patch3 := `
sysctl:
  params:
    vm.dirty_background_ratio: "5"
`
		combined := patch1 + "---\n" + patch2 + "---\n" + patch3

		mustRunCmdWithStdin(t, client, "machine config patch", strings.NewReader(combined))

		out := mustRunCmd(t, client, "machine config show")
		assert.Contains(t, out, "vm.swappiness")
		assert.Contains(t, out, "60")
		assert.Contains(t, out, "vm.dirty_ratio")
		assert.Contains(t, out, "20")
		assert.Contains(t, out, "vm.dirty_background_ratio")
		assert.Contains(t, out, "5")
	})

	run(t, "patch with --reboot flag", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: "80"
`
		out := mustRunCmdWithStdin(t, client, "machine config patch --reboot", strings.NewReader(patch))
		assert.Contains(t, out, "Rebooting...")
	})
}
