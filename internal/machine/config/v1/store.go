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
		return nil, nil
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
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("store mkdir: %w", err)
	}
	if err := os.WriteFile(configPath, b, 0o644); err != nil {
		return fmt.Errorf("store write: %w", err)
	}
	return nil
}
