// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

var testIfaces = map[string]nmstate.Interface{
	"eth0": {Name: "eth0", Type: "ethernet"},
}

// formDefaults mirrors a freshly built form: no interface selected, and the
// "Address mode" select pre-filled with its first option.
var formDefaults = FormData{InterfaceName: interfaceNone, Kind: kindDHCP}

func TestToSeleniteConfigEmpty(t *testing.T) {
	t.Parallel()

	cfg, err := formDefaults.ToSeleniteConfig(testIfaces)
	require.NoError(t, err)
	require.Equal(t, nmstate.SeleniteConfig{}, cfg)
}

func TestToSeleniteConfigNoInterface(t *testing.T) {
	t.Parallel()

	cfg, err := FormData{
		Hostname:      "node1",
		Dns:           "1.1.1.1  2606:4700:4700::1111",
		InterfaceName: interfaceNone,
		// Values for hidden form groups must be ignored.
		Kind:    kindStatic,
		Address: "10.0.0.2/24",
		Gateway: "10.0.0.1",
	}.ToSeleniteConfig(testIfaces)
	require.NoError(t, err)

	state := cfg.Network.DesiredState
	require.Equal(t, "node1", state.Hostname.Config)
	require.Equal(t, []string{"1.1.1.1", "2606:4700:4700::1111"}, state.DNSResolver.Config.Server)
	require.Empty(t, state.Interfaces)
	require.Empty(t, state.Routes.Config)
}

func TestToSeleniteConfigDHCP(t *testing.T) {
	t.Parallel()

	cfg, err := FormData{InterfaceName: "eth0", Kind: kindDHCP}.ToSeleniteConfig(testIfaces)
	require.NoError(t, err)

	state := cfg.Network.DesiredState
	require.Equal(t, []nmstate.Interface{{
		Name:  "eth0",
		Type:  "ethernet",
		State: "up",
		Ipv4:  nmstate.Ipv4{Enabled: true, Dhcp: new(true)},
		Ipv6:  nmstate.Ipv6{Enabled: true, Dhcp: new(true), Autoconf: new(true)},
	}}, state.Interfaces)
	require.Empty(t, state.Routes.Config)
}

func TestToSeleniteConfigStaticIPv4(t *testing.T) {
	t.Parallel()

	cfg, err := FormData{
		InterfaceName: "eth0",
		Kind:          kindStatic,
		Address:       "192.168.1.10/24 192.168.2.10/16",
		Gateway:       "192.168.1.1",
	}.ToSeleniteConfig(testIfaces)
	require.NoError(t, err)

	state := cfg.Network.DesiredState
	require.Equal(t, []nmstate.Interface{{
		Name:  "eth0",
		Type:  "ethernet",
		State: "up",
		Ipv4: nmstate.Ipv4{
			Enabled: true,
			Dhcp:    new(false),
			Address: []nmstate.Address{
				{IP: "192.168.1.10", PrefixLength: 24},
				{IP: "192.168.2.10", PrefixLength: 16},
			},
		},
		Ipv6: nmstate.Ipv6{Address: []nmstate.Address{}},
	}}, state.Interfaces)
	require.Equal(t, []nmstate.RoutesConfig{{
		Destination:      "0.0.0.0/0",
		NextHopAddress:   "192.168.1.1",
		NextHopInterface: "eth0",
	}}, state.Routes.Config)
}

func TestToSeleniteConfigStaticDualStack(t *testing.T) {
	t.Parallel()

	cfg, err := FormData{
		InterfaceName: "eth0",
		Kind:          kindStatic,
		Address:       "10.0.0.2/24 2001:db8::2/64",
		Gateway:       "2001:db8::1",
	}.ToSeleniteConfig(testIfaces)
	require.NoError(t, err)

	state := cfg.Network.DesiredState
	require.Len(t, state.Interfaces, 1)
	iface := state.Interfaces[0]

	require.Equal(t, nmstate.Ipv4{
		Enabled: true,
		Dhcp:    new(false),
		Address: []nmstate.Address{{IP: "10.0.0.2", PrefixLength: 24}},
	}, iface.Ipv4)
	require.Equal(t, nmstate.Ipv6{
		Enabled:  true,
		Dhcp:     new(false),
		Autoconf: new(false),
		Address:  []nmstate.Address{{IP: "2001:db8::2", PrefixLength: 64}},
	}, iface.Ipv6)
	require.Equal(t, []nmstate.RoutesConfig{{
		Destination:      "::/0",
		NextHopAddress:   "2001:db8::1",
		NextHopInterface: "eth0",
	}}, state.Routes.Config)
}

// Both address families must be set explicitly: nmstate keeps the current
// settings for a missing key, e.g. DHCPv6 left over from DHCP mode.
func TestToSeleniteConfigYAMLIsExplicit(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		data       FormData
		ipv4, ipv6 map[string]any
	}{
		"static ipv4 only": {
			data: FormData{InterfaceName: "eth0", Kind: kindStatic, Address: "10.0.0.2/24", Gateway: "10.0.0.1"},
			ipv4: map[string]any{"enabled": true, "dhcp": false},
			ipv6: map[string]any{"enabled": false},
		},
		"static ipv6": {
			data: FormData{InterfaceName: "eth0", Kind: kindStatic, Address: "2001:db8::2/64", Gateway: "2001:db8::1"},
			ipv4: map[string]any{"enabled": true, "dhcp": false},
			ipv6: map[string]any{"enabled": true, "dhcp": false, "autoconf": false},
		},
		"dhcp": {
			data: FormData{InterfaceName: "eth0", Kind: kindDHCP},
			ipv4: map[string]any{"enabled": true, "dhcp": true},
			ipv6: map[string]any{"enabled": true, "dhcp": true, "autoconf": true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := tc.data.ToSeleniteConfig(testIfaces)
			require.NoError(t, err)

			b, err := yaml.Marshal(cfg)
			require.NoError(t, err)

			var got struct {
				Network struct {
					DesiredState struct {
						Interfaces []struct {
							Ipv4 map[string]any `yaml:"ipv4"`
							Ipv6 map[string]any `yaml:"ipv6"`
						} `yaml:"interfaces"`
					} `yaml:"desired_state"`
				} `yaml:"network"`
			}
			require.NoError(t, yaml.Unmarshal(b, &got))
			require.Len(t, got.Network.DesiredState.Interfaces, 1)

			iface := got.Network.DesiredState.Interfaces[0]
			for k, v := range tc.ipv4 {
				require.Equal(t, v, iface.Ipv4[k], "ipv4.%s", k)
			}
			for k, v := range tc.ipv6 {
				require.Equal(t, v, iface.Ipv6[k], "ipv6.%s", k)
			}
		})
	}
}

func TestToSeleniteConfigValidation(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		data  FormData
		field string
	}{
		"invalid hostname": {
			FormData{Hostname: "bad_host!", InterfaceName: interfaceNone, Kind: kindDHCP}, "Hostname",
		},
		"invalid dns": {
			FormData{Dns: "1.1.1.1 not-an-ip", InterfaceName: interfaceNone, Kind: kindDHCP}, "Dns[1]",
		},
		"invalid address": {
			FormData{InterfaceName: "eth0", Kind: kindStatic, Address: "10.0.0.2", Gateway: "10.0.0.1"}, "Address[0]",
		},
		"static no gateway": {
			FormData{InterfaceName: "eth0", Kind: kindStatic, Address: "10.0.0.2/24"}, "Gateway",
		},
		"static gateway not ip": {
			FormData{InterfaceName: "eth0", Kind: kindStatic, Address: "10.0.0.2/24", Gateway: "gw"}, "Gateway",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.data.ToSeleniteConfig(testIfaces)

			var errs validator.ValidationErrors
			require.ErrorAs(t, err, &errs)
			require.Len(t, errs, 1)
			require.Equal(t, tc.field, errs[0].Field())
		})
	}
}
