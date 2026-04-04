//go:build e2e

// Package e2e contains end-to-end tests that boot a real Selenite VM in QEMU.
package e2e

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"
)

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

func startQEMU(diskImage, sshPort string) (*exec.Cmd, error) {
	cmd := exec.Command("qemu-system-x86_64",
		"-enable-kvm",
		"-m", "256M",
		"-nographic",
		"-display", "none",
		"-serial", "none",
		"-monitor", "none",
		"-drive", fmt.Sprintf("file=%s,format=raw,if=virtio,snapshot=on", diskImage),
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%s-:%s", sshPort, "23234"),
		"-device", "virtio-net-pci,netdev=net0",
	)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func waitForSSH(addr string, timeout time.Duration) (*testClient, error) {
	_, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	cfg, signer, err := makeSSHConfig(privKey)
	if err != nil {
		panic(err)
	}
	ssh, err := dialSSH(addr, cfg, timeout)
	if err != nil {
		return nil, err
	}
	return &testClient{
		ssh:        ssh,
		privateKey: privKey,
		publicKey:  signer.PublicKey(),
	}, nil
}
