// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// networkConfig builds a full machine config YAML that includes the client's
// SSH key in access (required by AccessApplier) plus an arbitrary network
// section.
func networkConfig(client *testClient, network string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.publicKey)), "\n")
	return fmt.Sprintf(`
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - %q
network:
%s`, keyStr, network)
}

// A nmstate desired state that creates a dummy interface. A dummy interface is
// backed by the kernel (no real hardware) so it is safe to bring up without
// affecting the interface used for SSH connectivity.
const dummyUpConfig = `  desired_state:
    interfaces:
      - name: dummy0
        type: dummy
        state: up
`

func TestNetworkConfig(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	run(t, "rejected when desired_state is empty", func(t *testing.T) {
		cfg := networkConfig(client, "  desired_state: {}\n")
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "rejected when desired_state is whitespace-only", func(t *testing.T) {
		cfg := networkConfig(client, "  desired_state: |  \n    \n")
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "apply creates dummy interface", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
	})

	run(t, "apply without network section removes it from stored config", func(t *testing.T) {
		keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.publicKey)), "\n")
		cfg := fmt.Sprintf(`
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - %q
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: 1024
`, keyStr)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		out := mustRunCmd(t, client, "machine config show")
		require.NotContains(t, out, "dummy0")
	})

	run(t, "re-apply same config works", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
	})

	run(t, "invalid state value is rejected", func(t *testing.T) {
		cfg := networkConfig(client, `  desired_state:
    interfaces:
      - name: dummy0
        type: dummy
        state: bogus
`)
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "valid config applies again after error", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
	})

	run(t, "apply with reboot keeps machine reachable", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		mustRunCmdWithStdin(t, client, "machine config apply --reboot", strings.NewReader(cfg))

		client.reconnect(t, 240*time.Second)

		cfg2 := networkConfig(client, dummyUpConfig)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg2))
	})
}
