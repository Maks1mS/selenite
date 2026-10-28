// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary_card

import (
	"altlinux.space/alt-atomic/selenite/internal/net"
)

type Summary struct {
	IPs          []string
	DNS          []string
	Gateway      string
	Connectivity string
}

// Source provides the network summary.
type Source interface {
	Summary() Summary
}

// SystemSource reads the network summary from the local system.
type SystemSource struct{}

func (SystemSource) Summary() Summary {
	ips, _ := net.ListRealIPs()
	dns, _ := net.ListDns()
	return Summary{
		IPs:          ips,
		DNS:          dns,
		Gateway:      net.DefaultGateway(),
		Connectivity: net.Connectivity(),
	}
}
