//go:build e2e

package e2e

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

type testClient struct {
	ssh        *gossh.Client
	addr       string
	privateKey ed25519.PrivateKey
	publicKey  gossh.PublicKey
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

// bootVM starts a QEMU VM, waits for SSH, and registers cleanup.
// Each test gets its own VM for clean isolated state.
func bootVM(t *testing.T) *testClient {
	t.Helper()
	diskImage := os.Getenv("E2E_DISK_IMAGE")
	if diskImage == "" {
		diskImage = "/var/tmp/selenite/disk.raw"
	}

	sshPort, err := getFreePort()
	require.NoError(t, err, "get free port")

	cmd, err := startQEMU(diskImage, sshPort)
	require.NoError(t, err, "start QEMU")
	t.Cleanup(func() { cmd.Process.Kill() })

	addr := "127.0.0.1:" + sshPort
	client, err := waitForSSH(addr, 120*time.Second)
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("wait for SSH: %v", err)
	}
	client.addr = addr
	t.Cleanup(func() { client.ssh.Close() })

	return client
}

func (c *testClient) reconnect(t *testing.T, timeout time.Duration) {
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

// run runs a named sub-step; if it fails, the parent test is aborted.
// Use for ordered sequential steps where step N+1 depends on step N.
func run(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.Fatal("step failed, aborting")
	}
}

// mustRunCmd executes cmd over SSH and returns output; fatals on error.
func mustRunCmd(t *testing.T, client *testClient, cmd string) string {
	t.Helper()
	out, err := runCmdWithStdin(t, client.ssh, cmd, nil)
	require.NoError(t, err, "command %q\noutput: %s", cmd, out)
	return out
}

// deployStack sends composeYAML via stdin to `stack deploy <name>`.
func deployStack(t *testing.T, client *gossh.Client, name, composeYAML string) (string, error) {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var buf strings.Builder
	sess.Stderr = &buf
	sess.Stdin = strings.NewReader(composeYAML)

	if err := sess.Run("stack deploy " + name); err != nil {
		var ee *gossh.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee
		}
		return buf.String(), err
	}
	return buf.String(), nil
}

// mustDeployStack fatals if deploy fails.
func mustDeployStack(t *testing.T, client *testClient, name, composeYAML string) {
	t.Helper()
	_, err := deployStack(t, client.ssh, name, composeYAML)
	require.NoError(t, err, "stack deploy %q", name)
}

func runCmdWithStdin(t *testing.T, client *gossh.Client, cmd string, stdin io.Reader) (string, error) {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	if stdin != nil {
		sess.Stdin = stdin
	}

	var buf strings.Builder
	sess.Stdout = &buf
	sess.Stderr = &buf

	if err := sess.Run(cmd); err != nil {
		var ee *gossh.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee
		}
		return buf.String(), err
	}
	return buf.String(), nil
}

func mustRunCmdWithStdin(t *testing.T, client *testClient, cmd string, stdin io.Reader) string {
	t.Helper()
	out, err := runCmdWithStdin(t, client.ssh, cmd, stdin)
	require.NoError(t, err, "command %q\noutput: %s", cmd, out)
	return out
}

func provision(t *testing.T, client *testClient) {
	t.Helper()
	mustRunCmd(
		t,
		client,
		"provision",
	)
}

func bootVMWithProvison(t *testing.T) *testClient {
	client := bootVM(t)
	provision(t, client)
	return client
}
