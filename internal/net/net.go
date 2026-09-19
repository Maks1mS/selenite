// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package net

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"go.alt-gnome.ru/x/runner"

	"altlinux.space/alt-atomic/selenite/internal/nmstate"
)

func isPhysical(name string) bool {
	target, err := os.Readlink("/sys/class/net/" + name + "/device")
	if err != nil {
		return false
	}
	return !strings.Contains(target, "virtual")
}

func filter[T any](sl []T, f func(T) bool) []T {
	var ret []T
	for _, s := range sl {
		if f(s) {
			ret = append(ret, s)
		}
	}
	return ret
}

func ListPhysicalIfaces() ([]net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("net.Interfaces: %w", err)
	}

	ifaces = filter(ifaces, func(iface net.Interface) bool {
		if iface.Flags&net.FlagLoopback != 0 {
			return false
		}
		if iface.Flags&net.FlagUp == 0 {
			return false
		}
		if !isPhysical(iface.Name) {
			return false
		}

		return true
	})

	return ifaces, nil
}

func ListRealIPs() ([]string, error) {
	ifaces, err := ListPhysicalIfaces()
	if err != nil {
		return nil, err
	}

	var addrs []string
	for _, iface := range ifaces {
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		ifaceAddrs = filter(ifaceAddrs, func(a net.Addr) bool {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				return false
			}
			if ipNet.IP.IsLinkLocalUnicast() || ipNet.IP.IsLoopback() {
				return false
			}
			return true
		})

		for _, a := range ifaceAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.IsLinkLocalUnicast() || ipNet.IP.IsLoopback() {
				continue
			}
			addrs = append(addrs, ipNet.String())
		}
	}

	return addrs, nil
}

func DefaultGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		fields := strings.Fields(line)
		// Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		raw, err := hex.DecodeString(fields[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		// /proc/net/route stores addresses in little-endian byte order.
		return net.IP([]byte{raw[3], raw[2], raw[1], raw[0]}).String()
	}
	return ""
}

func ListDns() ([]string, error) {
	r := runner.NewLocal()

	var b bytes.Buffer
	err := r.Run(context.Background(), "/usr/bin/nmstatectl", []string{"show"}, runner.WithStdout(&b))
	if err != nil {
		return nil, fmt.Errorf("nmstatectl: %w", err)
	}

	var s nmstate.DesiredState
	err = yaml.Unmarshal(b.Bytes(), &s)
	if err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: %w", err)
	}

	return s.DNSResolver.Running.Server, nil
}

func Connectivity() string {
	dns, err := ListDns()
	if err != nil || len(dns) == 0 {
		return "UNKNOWN"
	}

	for _, server := range dns {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "53"), 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return "OK"
		}
	}
	return "DOWN"
}
