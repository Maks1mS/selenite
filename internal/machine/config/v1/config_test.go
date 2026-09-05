// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

func TestParseMachineConfigOptions(t *testing.T) {
	b := []byte(`
access:
  auth_guard:
    enabled: true
    max_retries: 3
    window: 1h
    ban_duration: 24h
    max_tracked_ips: 100
netbird:
  enabled: true
  setup_key: "abc123"
monitoring:
  enabled: true
  prometheus:
    enabled: true
    endpoint: "http://127.0.0.1:9090/api/v1/write"
    auth:
      username: "user123"
      password: "secret"
  loki:
    enabled: false
    endpoint: "http://loki:3100/loki/api/v1/push"
memory:
  zram:
    enabled: true
    size: ram/4
    compression: zstd
    swap_priority: 100
`)
	cfg, err := ParseMachineConfig(b)
	require.NoError(t, err)

	t.Run("auth_guard", func(t *testing.T) {
		g := cfg.Access.AuthGuard
		require.Equal(t, mo.Some(true), g.Enabled)
		require.Equal(t, mo.Some(3), g.MaxRetries)
		require.Equal(t, mo.Some("1h"), g.Window)
		require.Equal(t, mo.Some("24h"), g.BanDuration)
		require.Equal(t, mo.Some(100), g.MaxTrackedIPs)
	})

	t.Run("netbird", func(t *testing.T) {
		require.Equal(t, mo.Some(true), cfg.Netbird.Enabled)
		require.Equal(t, mo.Some("abc123"), cfg.Netbird.SetupKey)
	})

	t.Run("monitoring", func(t *testing.T) {
		m := cfg.Monitoring
		require.Equal(t, mo.Some(true), m.Enabled)
		require.Equal(t, mo.Some(true), m.Prometheus.Enabled)
		require.Equal(t, mo.Some("http://127.0.0.1:9090/api/v1/write"), m.Prometheus.Endpoint)
		require.Equal(t, mo.Some("user123"), m.Prometheus.Auth.Username)
		require.Equal(t, mo.Some("secret"), m.Prometheus.Auth.Password)
		require.Equal(t, mo.Some(false), m.Loki.Enabled)
		require.Equal(t, mo.Some("http://loki:3100/loki/api/v1/push"), m.Loki.Endpoint)
	})

	t.Run("zram", func(t *testing.T) {
		z := cfg.Memory.Zram
		require.Equal(t, mo.Some(true), z.Enabled)
		require.Equal(t, mo.Some("ram/4"), z.Size)
		require.Equal(t, mo.Some("zstd"), z.Compression)
		require.Equal(t, mo.Some(100), z.SwapPriority)
	})
}

func TestParseMachineConfigAbsentFieldsAreNone(t *testing.T) {
	cfg, err := ParseMachineConfig([]byte(`access:
  users:
    - name: root
      roles: [root]
      keys: ["ssh-ed25519 AAAA"]
`))
	require.NoError(t, err)

	require.False(t, cfg.Netbird.Enabled.IsPresent())
	require.False(t, cfg.Netbird.SetupKey.IsPresent())
	require.False(t, cfg.Monitoring.Enabled.IsPresent())
	require.False(t, cfg.Memory.Zram.Enabled.IsPresent())
	require.False(t, cfg.Memory.Zram.SwapPriority.IsPresent())
	require.False(t, cfg.Access.AuthGuard.Enabled.IsPresent())
}

func TestParseMachineConfigNullIsNone(t *testing.T) {
	cfg, err := ParseMachineConfig([]byte(`
netbird:
  enabled: null
  setup_key: ~
monitoring:
  enabled: null
`))
	require.NoError(t, err)
	require.False(t, cfg.Netbird.Enabled.IsPresent())
	require.False(t, cfg.Netbird.SetupKey.IsPresent())
	require.False(t, cfg.Monitoring.Enabled.IsPresent())
}

func TestOptionRoundTripThroughStore(t *testing.T) {
	dir := t.TempDir()
	origPath := configPath
	configPath = dir + "/machine.yml"
	t.Cleanup(func() { configPath = origPath })

	cfg, err := ParseMachineConfig([]byte(`
monitoring:
  enabled: true
  prometheus:
    enabled: true
    endpoint: "http://127.0.0.1:9090/api/v1/write"
memory:
  zram:
    enabled: true
    size: ram/4
    compression: zstd
    swap_priority: 100
`))
	require.NoError(t, err)

	require.NoError(t, (ConfigStore{}).Save(cfg))

	loaded, err := (ConfigStore{}).Load()
	require.NoError(t, err)
	require.Equal(t, cfg.Monitoring.Enabled, loaded.Monitoring.Enabled)
	require.Equal(t, cfg.Monitoring.Prometheus.Endpoint, loaded.Monitoring.Prometheus.Endpoint)
	require.Equal(t, cfg.Memory.Zram, loaded.Memory.Zram)
}
