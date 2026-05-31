// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

const configPath = "/var/lib/selenite-controller/machine.yml"

type Store interface {
	Load() (*MachineConfig, error)
	Save(*MachineConfig) error
}

type ConfigStore struct{}

func (ConfigStore) Load() (*MachineConfig, error) {
	b, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &MachineConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store load: %w", err)
	}
	return ParseMachineConfig(b)
}

func (ConfigStore) Save(cfg *MachineConfig) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("store marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return fmt.Errorf("store mkdir: %w", err)
	}
	if err := os.WriteFile(configPath, b, 0o600); err != nil {
		return fmt.Errorf("store write: %w", err)
	}
	return nil
}
