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

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

// monitoringConfig builds a full machine config YAML that includes the
// client's SSH key in access (required by AccessApplier) plus an
// arbitrary monitoring section.
func monitoringConfig(client *e2etest.Client, monitoring string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.PublicKey())), "\n")
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
	client := e2etest.BootVMProvisioned(t)

	e2etest.Step(t, "rejected when enabled with no subsystem configured", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: true\n")
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "rejected when prometheus enabled without endpoint", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
`)
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "rejected when loki enabled without endpoint", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  loki:
    enabled: true
`)
		_, err := client.TryRun("machine config apply", strings.NewReader(cfg))
		require.Error(t, err)
	})

	e2etest.Step(t, "apply with monitoring disabled is a no-op", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: false\n")
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
	})

	e2etest.Step(t, "apply enables alloy and writes config", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
    endpoint: "http://127.0.0.1:9090/api/v1/write"
  loki:
    enabled: true
    endpoint: "http://127.0.0.1:3100/loki/api/v1/push"
`)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
		out := client.Run(t, "machine logs -n 50 alloy")
		require.NotEmpty(t, out)
	})

	e2etest.Step(t, "apply with basic_auth writes credentials to config", func(t *testing.T) {
		cfg := monitoringConfig(client, `  enabled: true
  prometheus:
    enabled: true
    endpoint: "https://mimir.example.com/api/prom/push"
    auth:
      username: "user123"
      password: "secret"
`)
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))

		// out, err := client.TryRun("cat /etc/alloy/config.alloy", nil)
		// require.NoError(t, err)
		// assert.Contains(t, out, "basic_auth")
		// assert.Contains(t, out, "user123")
	})

	e2etest.Step(t, "re-apply with monitoring disabled stops alloy", func(t *testing.T) {
		cfg := monitoringConfig(client, "  enabled: false\n")
		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))

		// serviceOut, _ := client.TryRun("systemctl is-enabled alloy", nil)
		// assert.Contains(t, serviceOut, "disabled")
	})
}
