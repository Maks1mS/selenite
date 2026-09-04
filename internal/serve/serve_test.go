// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package serve

import (
	"net"
	"testing"
	"time"

	"charm.land/ssh"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic/selenite/internal/serve/authguard"
	"altlinux.space/alt-atomic/selenite/internal/serve/mocks"
)

type mockAddr string

func (a mockAddr) Network() string { return "tcp" }
func (a mockAddr) String() string  { return string(a) }

func TestBanConnCallbackBlocksBannedIP(t *testing.T) {
	g, err := authguard.NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	require.False(t, g.RecordFailure("10.0.0.1"))
	require.True(t, g.RecordFailure("10.0.0.1"))
	require.False(t, g.Allow("10.0.0.1"))

	cb := banConnCallback(g)
	conn := &mockConn{remoteAddr: mockAddr("10.0.0.1:12345")}
	result := cb(mocks.NewMockContext(t), conn)
	require.Nil(t, result)
	require.True(t, conn.closed)
}

func TestBanConnCallbackAllowsCleanIP(t *testing.T) {
	g, err := authguard.NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	cb := banConnCallback(g)
	conn := &mockConn{remoteAddr: mockAddr("10.0.0.2:12345")}
	result := cb(mocks.NewMockContext(t), conn)
	require.NotNil(t, result)
	require.False(t, conn.closed)
}

func TestPublicKeyAuthWrapperCountsFailures(t *testing.T) {
	g, err := authguard.NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	inner := func(ssh.Context, ssh.PublicKey) bool { return false }
	handler := publicKeyAuthWithInner(g, inner)

	ctx := mocks.NewMockContext(t)
	ctx.EXPECT().RemoteAddr().Return(mockAddr("10.0.0.3:12345")).Twice()

	ok := handler(ctx, nil)
	require.False(t, ok)
	require.True(t, g.Allow("10.0.0.3"))

	ok = handler(ctx, nil)
	require.False(t, ok)
	require.False(t, g.Allow("10.0.0.3"))
}

type mockConn struct {
	net.Conn
	remoteAddr net.Addr
	closed     bool
}

func (m *mockConn) RemoteAddr() net.Addr { return m.remoteAddr }
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}
