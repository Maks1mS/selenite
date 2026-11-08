// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

var (
	reUptimeDays    = regexp.MustCompile(`(\d+) days`)
	reUptimeHours   = regexp.MustCompile(`(\d+) hours`)
	reUptimeMinutes = regexp.MustCompile(`(\d+) minutes`)
	reUptimeSeconds = regexp.MustCompile(`(\d+) seconds`)
)

func parseUptimeSeconds(t *testing.T, out string) int {
	t.Helper()
	require.Contains(t, out, "Uptime:", "stats output missing Uptime line")
	total := 0
	if m := reUptimeDays.FindStringSubmatch(out); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += v * 86400
	}
	if m := reUptimeHours.FindStringSubmatch(out); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += v * 3600
	}
	if m := reUptimeMinutes.FindStringSubmatch(out); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += v * 60
	}
	m := reUptimeSeconds.FindStringSubmatch(out)
	require.NotNil(t, m, "uptime seconds not found in %q", out)
	v, _ := strconv.Atoi(m[1])
	total += v
	return total
}

func TestMachine(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)

	e2etest.Step(t, "'machine info' works", func(t *testing.T) {
		out := client.Run(t, "machine info")
		assert.Contains(t, out, "Memory:")
		assert.Contains(t, out, "Swap:")
	})

	e2etest.Step(t, "'machine config show' shows config after provision", func(t *testing.T) {
		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "access:")
		assert.Contains(t, out, "provisioner")
	})

	e2etest.Step(t, "'machine config apply' works", func(t *testing.T) {
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
    net.ipv4.ip_unprivileged_port_start: 80
`, keyStr)

		client.RunStdin(t, "machine config apply", strings.NewReader(cfg))
	})

	e2etest.Step(t, "'machine config show' reflects applied config", func(t *testing.T) {
		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "ip_unprivileged_port_start")
	})

	// var uptimeBefore int
	// e2etest.Step(t, "stats returns uptime", func(t *testing.T) {
	// 	time.Sleep(time.Second * 40)
	// 	out := client.Run(t, "machine stats")
	// 	uptimeBefore = parseUptimeSeconds(t, out)
	// 	assert.Greater(t, uptimeBefore, 0)
	// 	assert.Contains(t, out, "Memory:")
	// 	assert.Contains(t, out, "Disk:")
	// })

	e2etest.Step(t, "'machine logs' returns output", func(t *testing.T) {
		out := client.Run(t, "machine logs -n 10")
		assert.NotEmpty(t, out)
	})

	e2etest.Step(t, "'machine logs' filters by service", func(t *testing.T) {
		out := client.Run(t, "machine logs -n 20 selenite-controller")
		assert.NotEmpty(t, out)
		assert.Contains(t, out, "selenite-controller")
	})

	e2etest.Step(t, "'machine dmesg' returns output", func(t *testing.T) {
		out := client.Run(t, "machine dmesg")
		assert.NotEmpty(t, out)
	})

	e2etest.Step(t, "'machine dmesg' with flags returns output", func(t *testing.T) {
		out := client.Run(t, "machine dmesg --level info")
		assert.NotEmpty(t, out)
	})

	e2etest.Step(t, "reboot works", func(t *testing.T) {
		out := client.Run(t, "machine reboot")
		assert.Equal(t, "Rebooting...\n", out)
	})

	// e2etest.Step(t, "uptime resets after reboot", func(t *testing.T) {
	// 	client.Reconnect(t, 120*time.Second)
	// 	out := client.Run(t, "machine stats")
	// 	uptimeAfter := parseUptimeSeconds(t, out)
	// 	assert.Less(t, uptimeAfter, uptimeBefore)
	// })
}
