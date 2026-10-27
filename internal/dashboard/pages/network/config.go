// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-playground/validator/v10"

	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

type FormData struct {
	Hostname      string
	Dns           string
	InterfaceName string
	Kind          string
	Address       string
	Gateway       string
	Save          bool
}

type FormDataNormalized struct {
	Hostname      string   `validate:"omitempty,hostname"`
	Dns           []string `validate:"dive,ip"`
	InterfaceName string
	Kind          string
	Address       []string `validate:"dive,cidr"`
	Gateway       string   `validate:"required_unless=Kind dhcp,omitempty,ip"`
}

// ToSeleniteConfig validates the form data and converts it into a network config.
// ifaces supplies the type of the selected interface.
func (d FormData) ToSeleniteConfig(ifaces map[string]nmstate.Interface) (nmstate.SeleniteConfig, error) {
	input := FormDataNormalized{
		Hostname:      d.Hostname,
		Dns:           strings.Fields(d.Dns),
		InterfaceName: d.InterfaceName,
		Kind:          d.Kind,
		Address:       strings.Fields(d.Address),
		Gateway:       d.Gateway,
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(input); err != nil {
		return nmstate.SeleniteConfig{}, fmt.Errorf("validate error: %w", err)
	}

	state := nmstate.DesiredState{}

	if input.Hostname != "" {
		state.Hostname.Config = input.Hostname
	}
	if len(input.Dns) != 0 {
		state.DNSResolver.Config.Server = input.Dns
	}
	if input.InterfaceName != interfaceNone {
		iface := nmstate.Interface{
			Name:  input.InterfaceName,
			Type:  ifaces[input.InterfaceName].Type,
			State: "up",
			Ipv4:  nmstate.Ipv4{Enabled: true},
		}

		switch input.Kind {
		case kindDHCP:
			iface.Ipv4.Dhcp = new(true)
			iface.Ipv6 = nmstate.Ipv6{Enabled: true, Dhcp: new(true), Autoconf: new(true)}
		case kindStatic:
			applyStatic(&iface, input)
			state.Routes.Config = []nmstate.RoutesConfig{defaultRoute(input)}
		}

		state.Interfaces = []nmstate.Interface{iface}
	}

	return nmstate.SeleniteConfig{Network: nmstate.NetworkConfig{DesiredState: state}}, nil
}

func applyStatic(iface *nmstate.Interface, input FormDataNormalized) {
	iface.Ipv4.Dhcp = new(false)
	iface.Ipv4.Address = []nmstate.Address{}
	iface.Ipv6.Address = []nmstate.Address{}

	for _, addr := range input.Address {
		ipAddr, ipNet, err := net.ParseCIDR(addr)
		if err != nil {
			continue
		}

		prefixLength, _ := ipNet.Mask.Size()

		address := nmstate.Address{
			IP:           ipAddr.String(),
			PrefixLength: prefixLength,
		}

		if ipAddr.To4() != nil {
			iface.Ipv4.Address = append(iface.Ipv4.Address, address)
		} else {
			iface.Ipv6.Address = append(iface.Ipv6.Address, address)
		}
	}

	if len(iface.Ipv6.Address) != 0 {
		iface.Ipv6.Enabled = true
		iface.Ipv6.Dhcp = new(false)
		iface.Ipv6.Autoconf = new(false)
	}
}

func defaultRoute(input FormDataNormalized) nmstate.RoutesConfig {
	destination := "0.0.0.0/0"
	if gw := net.ParseIP(input.Gateway); gw != nil && gw.To4() == nil {
		destination = "::/0"
	}

	return nmstate.RoutesConfig{
		Destination:      destination,
		NextHopAddress:   input.Gateway,
		NextHopInterface: input.InterfaceName,
	}
}
