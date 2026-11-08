// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

// Package e2etest provides the VM and SSH plumbing used by the e2e tests:
// booting a Selenite VM in QEMU and talking to its restricted SSH API.
package e2etest

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// Client is a booted Selenite VM with an SSH connection to its API.
type Client struct {
	ssh        *gossh.Client
	addr       string
	privateKey ed25519.PrivateKey
	publicKey  gossh.PublicKey
}

// BootVM starts a QEMU VM, waits for SSH, and registers cleanup.
// Extra scratch disks may be attached (see BootVMWithDisk).
func BootVM(t testing.TB, extraDisks ...string) *Client {
	t.Helper()
	return BootVMWithDisk(t, BaseDiskImage(), "raw", extraDisks...)
}

// BootVMProvisioned is BootVM followed by `provision`.
func BootVMProvisioned(t testing.TB, extraDisks ...string) *Client {
	t.Helper()
	c := BootVM(t, extraDisks...)
	c.Provision(t)
	return c
}

// BootVMWithDisk is like BootVM, but boots the given disk image. Extra
// scratch disks appear as /dev/vdb, /dev/vdc, ... in the order given.
func BootVMWithDisk(t testing.TB, diskImage, diskFormat string, extraDisks ...string) *Client {
	t.Helper()
	sshPort, err := getFreePort()
	require.NoError(t, err, "get free port")

	cmd, err := startQEMU(diskImage, diskFormat, sshPort, extraDisks...)
	require.NoError(t, err, "start QEMU")
	t.Cleanup(func() { cmd.Process.Kill() })

	addr := "127.0.0.1:" + sshPort
	client, err := waitForSSH(addr, 240*time.Second)
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("wait for SSH: %v", err)
	}
	client.addr = addr
	t.Cleanup(func() { client.ssh.Close() })

	return client
}

// BaseDiskImage returns the base disk image (overridable via
// E2E_DISK_IMAGE).
func BaseDiskImage() string {
	if diskImage := os.Getenv("E2E_DISK_IMAGE"); diskImage != "" {
		return diskImage
	}
	return "/var/tmp/selenite/disk.raw"
}

// Provision runs `provision` on the VM.
func (c *Client) Provision(t testing.TB) {
	t.Helper()
	c.Run(t, "provision")
}

// PublicKey returns the SSH public key the VM trusts.
func (c *Client) PublicKey() gossh.PublicKey {
	return c.publicKey
}

// Reconnect re-establishes the SSH connection after a reboot, waiting for
// the VM to go offline and come back within the timeout.
func (c *Client) Reconnect(t testing.TB, timeout time.Duration) {
	t.Helper()
	c.ssh.Close()
	cfg, _, err := makeSSHConfig(c.privateKey)
	if err != nil {
		t.Fatalf("reconnect: create signer: %v", err)
	}
	deadline := time.Now().Add(timeout)

	// wait for the VM to go offline
	offlineDeadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(offlineDeadline) {
		conn, err := gossh.Dial("tcp", c.addr, cfg)
		if err != nil {
			break // VM is shutting down
		}
		conn.Close()
		time.Sleep(2 * time.Second)
	}

	// wait for the VM to come back online
	ssh, err := dialSSH(c.addr, cfg, time.Until(deadline))
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	c.ssh = ssh
	t.Cleanup(func() { ssh.Close() })
}

func makeSSHConfig(privKey ed25519.PrivateKey) (*gossh.ClientConfig, gossh.Signer, error) {
	signer, err := gossh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, nil, err
	}
	return &gossh.ClientConfig{
		User:            "selenite",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
	}, signer, nil
}

func dialSSH(addr string, cfg *gossh.ClientConfig, timeout time.Duration) (*gossh.Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ssh, err := gossh.Dial("tcp", addr, cfg)
		if err == nil {
			return ssh, nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("SSH not ready after %s: %w", timeout, lastErr)
}

func waitForSSH(addr string, timeout time.Duration) (*Client, error) {
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
	return &Client{
		ssh:        ssh,
		privateKey: privKey,
		publicKey:  signer.PublicKey(),
	}, nil
}

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

func startQEMU(diskImage, diskFormat, sshPort string, extraDisks ...string) (*exec.Cmd, error) {
	args := []string{
		"-enable-kvm",
		"-m", "384M",
		"-nographic",
		"-display", "none",
		"-serial", "none",
		"-monitor", "none",
		"-drive", fmt.Sprintf("file=%s,format=%s,if=virtio,snapshot=on", diskImage, diskFormat),
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%s-:%s", sshPort, "23234"),
		"-device", "virtio-net-pci,netdev=net0",
	}
	// Optional scratch disks (e.g. a formatted data disk for storage tests).
	for _, extra := range extraDisks {
		args = append(args,
			"-drive", fmt.Sprintf("file=%s,format=raw,if=virtio", extra))
	}
	cmd := exec.Command("qemu-system-x86_64", args...) //nolint:gosec // test VM arguments are built from test inputs
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
