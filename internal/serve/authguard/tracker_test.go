// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package authguard

import (
	"fmt"
	"testing"
	"time"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

func TestGuardAllowDisabled(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{Enabled: mo.Some(false)})
	require.NoError(t, err)
	require.True(t, g.Allow("1.2.3.4"))
}

func TestGuardBanAndExpire(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(3),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(50 * time.Millisecond),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	ip := "1.2.3.4"
	require.True(t, g.Allow(ip))
	require.False(t, g.RecordFailure(ip))
	require.True(t, g.Allow(ip))
	require.False(t, g.RecordFailure(ip))
	require.True(t, g.Allow(ip))
	require.True(t, g.RecordFailure(ip)) // 3rd failure -> banned
	require.False(t, g.Allow(ip))
	time.Sleep(60 * time.Millisecond)
	require.True(t, g.Allow(ip))
}

func TestGuardCleanupRemovesStale(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(5),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	ip := "1.2.3.4"
	require.False(t, g.RecordFailure(ip))
	require.False(t, g.RecordFailure(ip))
	g.Cleanup()
	// attempts still within window
	require.Equal(t, 1, g.attempts.Len())
}

func TestGuardCleanupRemovesExpired(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(2),
		Window:        mo.Some(50 * time.Millisecond),
		BanDuration:   mo.Some(50 * time.Millisecond),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	ip := "1.2.3.4"
	require.False(t, g.RecordFailure(ip))
	require.True(t, g.RecordFailure(ip))
	require.False(t, g.Allow(ip))
	time.Sleep(60 * time.Millisecond)
	g.Cleanup()
	require.True(t, g.Allow(ip))
}

func TestGuardRecordFailureDisabled(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{Enabled: mo.Some(false)})
	require.NoError(t, err)
	require.False(t, g.RecordFailure("1.2.3.4"))
}

func TestGuardLRUEviction(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(5),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(2),
	})
	require.NoError(t, err)

	g.RecordFailure("1.2.3.1")
	g.RecordFailure("1.2.3.2")
	require.Equal(t, 2, g.attempts.Len())

	g.RecordFailure("1.2.3.3") // evicts oldest
	require.Equal(t, 2, g.attempts.Len())
}

func TestGuardMemoryBounded(t *testing.T) {
	g, err := NewGuard(v1.AuthGuardConfig{
		Enabled:       mo.Some(true),
		MaxRetries:    mo.Some(3),
		Window:        mo.Some(time.Hour),
		BanDuration:   mo.Some(time.Hour),
		MaxTrackedIPs: mo.Some(100),
	})
	require.NoError(t, err)

	for i := range 1_000_000 {
		ip := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		_ = g.RecordFailure(ip)
	}

	require.LessOrEqual(t, g.attempts.Len(), g.cfg.MaxTrackedIPs.OrEmpty())
	require.LessOrEqual(t, g.bans.Len(), g.cfg.MaxTrackedIPs.OrEmpty())
}
