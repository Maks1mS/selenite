// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build integration

package integration

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"testing"
	"time"

	"charm.land/ssh"
	"charm.land/wish/v2"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic/selenite/internal/serve/authguard"
)

func TestAuthGuardBlocksAfterRepeatedFailures(t *testing.T) {
	g, err := authguard.NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("1h"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g.WithPersistPath(filepath.Join(t.TempDir(), "authguard.json"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	pubKeyAuth := func(ctx ssh.Context, key ssh.PublicKey) bool {
		// always reject
		host, _, _ := net.SplitHostPort(ctx.RemoteAddr().String())
		g.RecordFailure(host)
		return false
	}

	connCb := func(_ ssh.Context, conn net.Conn) net.Conn {
		host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !g.Allow(host) {
			_ = conn.Close()
			return nil
		}
		return conn
	}

	hostKey := filepath.Join(t.TempDir(), "host_key")
	srv, err := wish.NewServer(
		wish.WithPublicKeyAuth(pubKeyAuth),
		wish.WithHostKeyPath(hostKey),
		func(s *ssh.Server) error {
			s.ConnCallback = connCb
			return nil
		},
	)
	require.NoError(t, err)

	go func() {
		if err := srv.Serve(ln); err != nil && err != ssh.ErrServerClosed {
			t.Logf("server closed: %v", err)
		}
	}()
	defer srv.Close()

	_, fakePriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(fakePriv)
	require.NoError(t, err)

	clientCfg := &gossh.ClientConfig{
		User:            "test",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}

	addr := ln.Addr().String()

	_, err = gossh.Dial("tcp", addr, clientCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to authenticate")

	_, err = gossh.Dial("tcp", addr, clientCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to authenticate")

	_, err = gossh.Dial("tcp", addr, clientCfg)
	require.Error(t, err)
	// Connection closed by ConnCallback, not auth failure.
	require.NotContains(t, err.Error(), "unable to authenticate")
}

func TestAuthGuardAllowsCleanIP(t *testing.T) {
	g, err := authguard.NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(5),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("1h"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g.WithPersistPath(filepath.Join(t.TempDir(), "authguard.json"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	connCb := func(_ ssh.Context, conn net.Conn) net.Conn {
		host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !g.Allow(host) {
			_ = conn.Close()
			return nil
		}
		return conn
	}

	hostKey := filepath.Join(t.TempDir(), "host_key")
	srv, err := wish.NewServer(
		wish.WithHostKeyPath(hostKey),
		func(s *ssh.Server) error {
			s.ConnCallback = connCb
			return nil
		},
	)
	require.NoError(t, err)

	go func() {
		if err := srv.Serve(ln); err != nil && err != ssh.ErrServerClosed {
			t.Logf("server closed: %v", err)
		}
	}()
	defer srv.Close()

	clientCfg := &gossh.ClientConfig{
		User:            "test",
		Auth:            []gossh.AuthMethod{},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}
	client, err := gossh.Dial("tcp", ln.Addr().String(), clientCfg)
	require.NoError(t, err)
	require.NoError(t, client.Close())
}
