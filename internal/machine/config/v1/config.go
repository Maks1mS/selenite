package v1

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// SysctlConfig holds kernel parameters to apply via sysctl.
// Keys are sysctl parameter names, values are their desired string values.
type SysctlConfig struct {
	Params map[string]string `yaml:"params"`
}

type MachineConfig struct {
	Sysctl SysctlConfig `yaml:"sysctl"`
}

func ParseMachineConfig(b []byte) (*MachineConfig, error) {
	config := &MachineConfig{}
	if err := yaml.Unmarshal(b, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return config, nil
}
