// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
