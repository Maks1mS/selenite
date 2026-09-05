// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigStore_Save_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	origPath := configPath
	configPath = filepath.Join(dir, "machine.yml")
	t.Cleanup(func() { configPath = origPath })

	cfg := &MachineConfig{}

	if err := (ConfigStore{}).Save(cfg); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	b, err := os.ReadFile(configPath) //nolint:gosec // configPath is test-overridable
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if strings.TrimSpace(string(b)) != "{}" {
		t.Errorf("expected empty YAML map for empty MachineConfig, got %s", string(b))
	}
}

func TestConfigStore_Save_And_Load(t *testing.T) {
	dir := t.TempDir()
	origPath := configPath
	configPath = filepath.Join(dir, "machine.yml")
	t.Cleanup(func() { configPath = origPath })

	cfg, err := ParseMachineConfig([]byte(`
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
`))
	require.NoError(t, err)

	s := ConfigStore{}
	require.NoError(t, s.Save(cfg))

	cfg, err = s.Load()
	require.NoError(t, err)
	require.NotEmpty(t, cfg)
}
