// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package nmstate

type SeleniteConfig struct {
	Network NetworkConfig `yaml:"network,omitempty"`
}

type NetworkConfig struct {
	DesiredState DesiredState `yaml:"desired_state,omitempty"`
}

type DesiredState struct {
	Hostname    Hostname    `yaml:"hostname,omitempty"`
	DNSResolver DNSResolver `yaml:"dns-resolver,omitempty"`
	Interfaces  []Interface `yaml:"interfaces,omitempty"`
	Routes      Routes      `yaml:"routes,omitempty"`
}

type Hostname struct {
	Config string `yaml:"config"`
}

type DNSResolver struct {
	Running DNSResolverConfig `yaml:"running,omitempty"`
	Config  DNSResolverConfig `yaml:"config,omitempty"`
}

type DNSResolverConfig struct {
	Search []string `yaml:"search,omitempty"`
	Server []string `yaml:"server,omitempty"`
}

type Interface struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	State string `yaml:"state"`
	Ipv4  Ipv4   `yaml:"ipv4"`
	Ipv6  Ipv6   `yaml:"ipv6"`
}

type Ipv4 struct {
	Address []Address `yaml:"address"`
	Dhcp    *bool     `yaml:"dhcp,omitempty"`
	Enabled bool      `yaml:"enabled"`
}

type Ipv6 struct {
	Address  []Address `yaml:"address"`
	Dhcp     *bool     `yaml:"dhcp,omitempty"`
	Autoconf *bool     `yaml:"autoconf,omitempty"`
	Enabled  bool      `yaml:"enabled"`
}

type Address struct {
	IP           string `yaml:"ip"`
	PrefixLength int    `yaml:"prefix-length"`
}

type Routes struct {
	Config []RoutesConfig `yaml:"config"`
}

type RoutesConfig struct {
	Destination      string `yaml:"destination"`
	NextHopAddress   string `yaml:"next-hop-address"`
	NextHopInterface string `yaml:"next-hop-interface"`
}
