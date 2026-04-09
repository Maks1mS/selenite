//go:build e2e

package e2e

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	client := bootVMWithProvison(t)

	run(t, "'machine info' works", func(t *testing.T) {
		mustRunCmd(t, client, "machine info")
	})

	run(t, "'machine config apply' works", func(t *testing.T) {
		cfg := `
sysctl:
  params:
    net.ipv4.ip_unprivileged_port_start: 80
`

		mustRunCmdWithStdin(t, client, "machine apply", strings.NewReader(cfg))
	})

	// var uptimeBefore int
	// run(t, "stats returns uptime", func(t *testing.T) {
	// 	time.Sleep(time.Second * 40)
	// 	out := mustRunCmd(t, client, "machine stats")
	// 	uptimeBefore = parseUptimeSeconds(t, out)
	// 	assert.Greater(t, uptimeBefore, 0)
	// 	assert.Contains(t, out, "Memory:")
	// 	assert.Contains(t, out, "Disk:")
	// })

	run(t, "reboot works", func(t *testing.T) {
		out := mustRunCmd(t, client, "machine reboot")
		assert.Equal(t, "Rebooting...\n", out)
	})

	// run(t, "uptime resets after reboot", func(t *testing.T) {
	// 	client.reconnect(t, 120*time.Second)
	// 	out := mustRunCmd(t, client, "machine stats")
	// 	uptimeAfter := parseUptimeSeconds(t, out)
	// 	assert.Less(t, uptimeAfter, uptimeBefore)
	// })
}
