// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package authguard

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

func TestSaveAndLoad(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("1h"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g.Reset()

	tmp := filepath.Join(t.TempDir(), "state.json")
	g.WithPersistPath(tmp)

	require.False(t, g.RecordFailure("10.0.0.1"))
	require.True(t, g.RecordFailure("10.0.0.1"))
	require.NoError(t, g.Save())

	g2, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("1h"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g2.WithPersistPath(tmp)
	require.NoError(t, g2.load())

	require.False(t, g2.Allow("10.0.0.1"))
	require.True(t, g2.Allow("10.0.0.2"))
}

func TestLoadIgnoresExpired(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("50ms"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g.Reset()

	tmp := filepath.Join(t.TempDir(), "state.json")
	g.WithPersistPath(tmp)

	require.False(t, g.RecordFailure("10.0.0.1"))
	require.True(t, g.RecordFailure("10.0.0.1"))
	require.NoError(t, g.Save())

	time.Sleep(60 * time.Millisecond)

	g2, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some("1h"),
		BanDuration:   mo.Some("50ms"),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)
	g2.WithPersistPath(tmp)
	require.NoError(t, g2.load())

	require.True(t, g2.Allow("10.0.0.1"))
}
