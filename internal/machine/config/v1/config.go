package v1

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

type User struct {
	Name  string   `yaml:"name"`
	Roles []string `yaml:"roles"`
	Keys  []string `yaml:"keys"`
}

type Rules struct {
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

type Role struct {
	Name  string  `yaml:"name"`
	Rules []Rules `yaml:"rules"`
}

type AccessConfig struct {
	Users []User `yaml:"users"`
	Roles []Role `yaml:"roles"`
}

// SysctlConfig holds kernel parameters to apply via sysctl.
// Keys are sysctl parameter names, values are their desired string values.
type SysctlConfig struct {
	Params map[string]string `yaml:"params"`
}

type MachineConfig struct {
	Sysctl SysctlConfig `yaml:"sysctl"`
	Access AccessConfig `yaml:"access"`
}

func ParseMachineConfig(b []byte) (*MachineConfig, error) {
	config := &MachineConfig{}
	if err := yaml.Unmarshal(b, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return config, nil
}
