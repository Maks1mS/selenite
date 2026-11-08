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

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

// networkConfig builds a full machine config YAML that includes the client's
// SSH key in access (required by AccessApplier) plus an arbitrary network
// section.
func networkConfig(client *e2etest.Client, network string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.PublicKey())), "\n")
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
	client := e2etest.BootVMProvisioned(t)

	e2etest.Step(t, "rejected when desired_state is empty", func(t *testing.T) {
		cfg := networkConfig(client, "  desired_state: {}\n")
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "rejected when desired_state is whitespace-only", func(t *testing.T) {
		cfg := networkConfig(client, "  desired_state: |  \n    \n")
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "apply creates dummy interface", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
	})

	e2etest.Step(t, "apply without network section removes it from stored config", func(t *testing.T) {
		keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.PublicKey())), "\n")
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
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))

		out := client.Run(t, "machine config show")
		require.NotContains(t, out, "dummy0")
	})

	e2etest.Step(t, "re-apply same config works", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
	})

	e2etest.Step(t, "invalid state value is rejected", func(t *testing.T) {
		cfg := networkConfig(client, `  desired_state:
    interfaces:
      - name: dummy0
        type: dummy
        state: bogus
`)
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "valid config applies again after error", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
	})

	e2etest.Step(t, "apply with reboot keeps machine reachable", func(t *testing.T) {
		cfg := networkConfig(client, dummyUpConfig)
		client.RunStdin(t, "machine config apply --reboot", strings.NewReader(cfg))

		client.Reconnect(t, 240*time.Second)

		cfg2 := networkConfig(client, dummyUpConfig)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg2))
	})
}
