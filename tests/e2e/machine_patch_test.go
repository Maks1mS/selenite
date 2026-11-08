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

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

func TestMachinePatch(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)

	// First, apply a base config so we have a known state to patch.
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.PublicKey())), "\n")
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
	client.RunStdin(t, "machine config apply", strings.NewReader(baseCfg))

	e2etest.Step(t, "merge patch adds new sysctl param", func(t *testing.T) {
		patch := `
sysctl:
  params:
    vm.max_map_count: "262144"
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "vm.max_map_count")
		assert.Contains(t, out, "262144")
		// Original param must survive merge patch.
		assert.Contains(t, out, "net.ipv4.ip_unprivileged_port_start")
	})

	e2etest.Step(t, "merge patch overrides existing value", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: "1024"
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "1024")
	})

	e2etest.Step(t, "json patch (RFC 6902) replaces a value", func(t *testing.T) {
		patch := `
- op: replace
  path: /sysctl/params/net.ipv4.ip_unprivileged_port_start
  value: "80"
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "net.ipv4.ip_unprivileged_port_start")
		assert.Contains(t, out, ": \"80\"")
	})

	e2etest.Step(t, "json patch (RFC 6902) adds a new field", func(t *testing.T) {
		patch := `
- op: add
  path: /sysctl/params/net.ipv4.tcp_syncookies
  value: "1"
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "tcp_syncookies")
		assert.Contains(t, out, "\"1\"")
	})

	e2etest.Step(t, "json patch (RFC 6902) removes a field", func(t *testing.T) {
		patch := `
- op: remove
  path: /sysctl/params/net.ipv4.tcp_syncookies
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.NotContains(t, out, "tcp_syncookies")
	})

	e2etest.Step(t, "merge patch removes field with null", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: null
`
		client.RunStdin(t, "machine config patch", strings.NewReader(patch))

		out := client.Run(t, "machine config show")
		assert.NotContains(t, out, "net.ipv4.ip_unprivileged_port_start")
	})

	e2etest.Step(t, "multiple patches applied sequentially", func(t *testing.T) {
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

		client.RunStdin(t, "machine config patch", strings.NewReader(combined))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "vm.swappiness")
		assert.Contains(t, out, "60")
		assert.Contains(t, out, "vm.dirty_ratio")
		assert.Contains(t, out, "20")
		assert.Contains(t, out, "vm.dirty_background_ratio")
		assert.Contains(t, out, "5")
	})

	e2etest.Step(t, "patch with --reboot flag", func(t *testing.T) {
		patch := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: "80"
`
		out := client.RunStdin(t, "machine config patch --reboot", strings.NewReader(patch))
		assert.Contains(t, out, "Rebooting...")
	})
}
