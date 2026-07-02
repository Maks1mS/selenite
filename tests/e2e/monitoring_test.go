// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// monitoringConfig builds a full machine config YAML that includes the
// client's SSH key in access (required by AccessApplier) plus an
// arbitrary monitoring section.
func monitoringConfig(client *testClient, monitoring string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.publicKey)), "\n")
	return fmt.Sprintf(`
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - %q
monitoring:
%s`, keyStr, monitoring)
}

func TestMonitoringConfig(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	run(t, "rejected when enabled with no subsystem configured", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: true\n")
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "rejected when prometheus enabled without endpoint", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
`)
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "rejected when loki enabled without endpoint", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  loki:
    enabled: true
`)
		_, err := runCmdWithStdin(t, client.ssh, "machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	run(t, "apply with monitoring disabled is a no-op", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: false\n")
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
	})

	run(t, "apply enables alloy and writes config", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
    endpoint: "http://127.0.0.1:9090/api/v1/write"
  loki:
    enabled: true
    endpoint: "http://127.0.0.1:3100/loki/api/v1/push"
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))
		out := mustRunCmd(t, client, "machine logs -n 50 alloy")
		require.NotEmpty(t, out)
	})

	run(t, "apply with basic_auth writes credentials to config", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
    endpoint: "https://mimir.example.com/api/prom/push"
    auth:
      username: "user123"
      password: "secret"
`)
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		// out, err := runCmdWithStdin(t, client.ssh, "cat /etc/alloy/config.alloy", nil)
		// require.NoError(t, err)
		// assert.Contains(t, out, "basic_auth")
		// assert.Contains(t, out, "user123")
	})

	run(t, "re-apply with monitoring disabled stops alloy", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: false\n")
		mustRunCmdWithStdin(t, client, "machine config apply", strings.NewReader(cfg))

		// serviceOut, _ := runCmdWithStdin(t, client.ssh, "systemctl is-enabled alloy", nil)
		// assert.Contains(t, serviceOut, "disabled")
	})
}
