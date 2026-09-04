// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/samber/mo"
)

type User struct {
	Name  string   `yaml:"name" json:"name"`
	Roles []string `yaml:"roles,omitempty" json:"roles,omitempty"`
	Keys  []string `yaml:"keys,omitempty" json:"keys,omitempty"`
}

type Rules struct {
	Resources []string `yaml:"resources,omitempty" json:"resources,omitempty"`
	Verbs     []string `yaml:"verbs,omitempty" json:"verbs,omitempty"`
}

type Role struct {
	Name  string  `yaml:"name" json:"name"`
	Rules []Rules `yaml:"rules,omitempty" json:"rules,omitempty"`
}

type AuthGuardConfig struct {
	Enabled       mo.Option[bool]          `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxRetries    mo.Option[int]           `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	Window        mo.Option[time.Duration] `yaml:"window,omitempty" json:"window,omitempty"`
	BanDuration   mo.Option[time.Duration] `yaml:"ban_duration,omitempty" json:"ban_duration,omitempty"`
	MaxTrackedIPs mo.Option[int]           `yaml:"max_tracked_ips,omitempty" json:"max_tracked_ips,omitempty"`
}

type AccessConfig struct {
	Users     []User          `yaml:"users,omitempty" json:"users,omitempty"`
	Roles     []Role          `yaml:"roles,omitempty" json:"roles,omitempty"`
	AuthGuard AuthGuardConfig `yaml:"auth_guard,omitempty" json:"auth_guard,omitempty"`
}

// SysctlConfig holds kernel parameters to apply via sysctl.
// Keys are sysctl parameter names, values are their desired string values.
type SysctlConfig struct {
	Params map[string]string `yaml:"params,omitempty" json:"params,omitempty"`
}

type NetbirdConfig struct {
	Enabled  mo.Option[bool]   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	SetupKey mo.Option[string] `yaml:"setup_key,omitempty" json:"setup_key,omitempty"`
}

type MonitoringAuth struct {
	Username mo.Option[string] `yaml:"username,omitempty" json:"username,omitempty"`
	Password mo.Option[string] `yaml:"password,omitempty" json:"password,omitempty"`
}

type PrometheusConfig struct {
	Enabled  mo.Option[bool]   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Endpoint mo.Option[string] `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Auth     MonitoringAuth    `yaml:"auth,omitempty" json:"auth,omitempty"`
}

type LokiConfig struct {
	Enabled  mo.Option[bool]   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Endpoint mo.Option[string] `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Auth     MonitoringAuth    `yaml:"auth,omitempty" json:"auth,omitempty"`
}

type MonitoringConfig struct {
	Enabled    mo.Option[bool]  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Prometheus PrometheusConfig `yaml:"prometheus,omitempty" json:"prometheus,omitempty"`
	Loki       LokiConfig       `yaml:"loki,omitempty" json:"loki,omitempty"`
}

type NetworkConfig struct {
	DesiredState yaml.RawMessage `yaml:"desired_state,omitempty" json:"desired_state,omitempty"`
}

type ZramConfig struct {
	Enabled      mo.Option[bool]   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Size         mo.Option[string] `yaml:"size,omitempty" json:"size,omitempty"`
	Compression  mo.Option[string] `yaml:"compression,omitempty" json:"compression,omitempty"`
	SwapPriority mo.Option[int]    `yaml:"swap_priority,omitempty" json:"swap_priority,omitempty"`
}

type MemoryConfig struct {
	Zram ZramConfig `yaml:"zram,omitempty" json:"zram,omitempty"`
}

type MachineConfig struct {
	Sysctl     SysctlConfig     `yaml:"sysctl,omitempty" json:"sysctl,omitempty"`
	Access     AccessConfig     `yaml:"access,omitempty" json:"access,omitempty"`
	Netbird    NetbirdConfig    `yaml:"netbird,omitempty" json:"netbird,omitempty"`
	Monitoring MonitoringConfig `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`
	Network    NetworkConfig    `yaml:"network,omitempty" json:"network,omitempty"`
	Memory     MemoryConfig     `yaml:"memory,omitempty" json:"memory,omitempty"`
}

func ParseMachineConfig(b []byte) (*MachineConfig, error) {
	config := &MachineConfig{}
	if err := yaml.UnmarshalWithOptions(b, config, yamlOptionDecodeOptions()...); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return config, nil
}

// MarshalConfig serializes a MachineConfig for persistence and display.
func MarshalConfig(cfg *MachineConfig) ([]byte, error) {
	b, err := yaml.MarshalWithOptions(cfg, yamlOptionEncodeOptions()...)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return b, nil
}

// decodeOption decodes a YAML scalar into a mo.Option. By default goccy routes
// scalar nodes to encoding.TextUnmarshaler, and mo.Option's text unmarshaler
// expects JSON input, so plain values like "ram/2" or "http://..." fail to
// parse. Registering these custom unmarshalers makes option fields accept
// regular YAML scalars while keeping None for null/absent values.
func decodeOption[T any](o *mo.Option[T], b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("~")) {
		*o = mo.None[T]()
		return nil
	}
	var v T
	if err := yaml.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("decode option: %w", err)
	}
	*o = mo.Some(v)
	return nil
}

func yamlOptionDecodeOptions() []yaml.DecodeOption {
	return []yaml.DecodeOption{
		yaml.CustomUnmarshaler(decodeOption[string]),
		yaml.CustomUnmarshaler(decodeOption[bool]),
		yaml.CustomUnmarshaler(decodeOption[int]),
		yaml.CustomUnmarshaler(decodeOption[time.Duration]),
	}
}

// yamlOptionEncodeOptions overrides mo.Option encoding. By default goccy wraps
// encoding.TextMarshaler output (mo.Option's MarshalText is JSON-based) into
// YAML strings, turning booleans into "true" and strings into "\"ram/2\"".
// Emitting the inner value as a regular YAML scalar keeps the stored config
// human-readable and round-trip safe with yamlOptionDecodeOptions.
func yamlOptionEncodeOptions() []yaml.EncodeOption {
	none := []byte("null")
	return []yaml.EncodeOption{
		yaml.UseJSONMarshaler(),
		yaml.CustomMarshaler(func(o mo.Option[string]) ([]byte, error) {
			v, ok := o.Get()
			if !ok {
				return none, nil
			}
			return []byte(strconv.Quote(v)), nil
		}),
		yaml.CustomMarshaler(func(o mo.Option[bool]) ([]byte, error) {
			v, ok := o.Get()
			if !ok {
				return none, nil
			}
			return []byte(strconv.FormatBool(v)), nil
		}),
		yaml.CustomMarshaler(func(o mo.Option[int]) ([]byte, error) {
			v, ok := o.Get()
			if !ok {
				return none, nil
			}
			return []byte(strconv.Itoa(v)), nil
		}),
		yaml.CustomMarshaler(func(o mo.Option[time.Duration]) ([]byte, error) {
			v, ok := o.Get()
			if !ok {
				return none, nil
			}
			return []byte(strconv.FormatInt(int64(v), 10)), nil
		}),
	}
}
