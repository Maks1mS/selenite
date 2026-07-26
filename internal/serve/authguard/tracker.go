// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package authguard

import (
	"context"
	"errors"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
)

// Guard tracks failed authentication attempts per IP and enforces temporary bans.
type Guard struct {
	cfg         v1.AuthGuardConfig
	attempts    *lru.Cache[string, []time.Time]
	bans        *lru.Cache[string, time.Time]
	persistPath string
}

// NewGuard creates a new Guard from config. If persistence file exists, it is loaded.
func NewGuard(cfg v1.AuthGuardConfig) (*Guard, error) {
	g := &Guard{
		cfg:         cfg,
		persistPath: persistPath,
	}
	if !cfg.Enabled {
		return g, nil
	}
	if cfg.MaxTrackedIPs < 1 {
		return nil, errors.New("authguard: MaxTrackedIPs must be at least 1")
	}
	var err error
	g.attempts, err = lru.New[string, []time.Time](cfg.MaxTrackedIPs)
	if err != nil {
		return nil, err
	}
	g.bans, err = lru.New[string, time.Time](cfg.MaxTrackedIPs)
	if err != nil {
		return nil, err
	}
	if err := g.load(); err != nil {
		return nil, err
	}
	return g, nil
}

// WithPersistPath sets a custom persistence path (for testing).
func (g *Guard) WithPersistPath(path string) *Guard {
	g.persistPath = path
	return g
}

// Allow reports whether the IP is permitted to attempt authentication.
func (g *Guard) Allow(ip string) bool {
	if !g.cfg.Enabled {
		return true
	}
	exp, ok := g.bans.Peek(ip)
	if !ok {
		return true
	}
	if time.Now().Before(exp) {
		return false
	}
	g.bans.Remove(ip)
	return true
}

// RecordFailure increments the failure counter for the IP.
// It returns true if the IP has just been banned.
func (g *Guard) RecordFailure(ip string) (banned bool) {
	if !g.cfg.Enabled {
		return false
	}
	cutoff := time.Now().Add(-g.cfg.Window)

	stamps, _ := g.attempts.Get(ip)
	var fresh []time.Time
	for _, t := range stamps {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	fresh = append(fresh, time.Now())
	g.attempts.Add(ip, fresh)

	if len(fresh) >= g.cfg.MaxRetries {
		g.bans.Add(ip, time.Now().Add(g.cfg.BanDuration))
		g.attempts.Remove(ip)
		_ = g.save()
		return true
	}
	return false
}

// Cleanup removes stale attempts and expired bans, then persists state if changed.
func (g *Guard) Cleanup() {
	if !g.cfg.Enabled {
		return
	}
	now := time.Now()
	cutoff := now.Add(-g.cfg.Window)
	changed := false

	for _, ip := range g.attempts.Keys() {
		stamps, ok := g.attempts.Get(ip)
		if !ok {
			continue
		}
		var fresh []time.Time
		for _, t := range stamps {
			if t.After(cutoff) {
				fresh = append(fresh, t)
			}
		}
		if len(fresh) == 0 {
			g.attempts.Remove(ip)
			changed = true
		} else if len(fresh) != len(stamps) {
			g.attempts.Add(ip, fresh)
			changed = true
		}
	}

	for _, ip := range g.bans.Keys() {
		exp, ok := g.bans.Peek(ip)
		if !ok {
			continue
		}
		if now.After(exp) {
			g.bans.Remove(ip)
			changed = true
		}
	}

	if changed {
		_ = g.save()
	}
}

// Save persists the current state to disk.
func (g *Guard) Save() error {
	if !g.cfg.Enabled {
		return nil
	}
	return g.save()
}

// Reset clears all tracked attempts and bans (for testing).
func (g *Guard) Reset() {
	if g.attempts != nil {
		g.attempts.Purge()
	}
	if g.bans != nil {
		g.bans.Purge()
	}
}

// CleanupLoop runs Cleanup periodically until the context is cancelled.
func (g *Guard) CleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			g.Cleanup()
			return
		case <-ticker.C:
			g.Cleanup()
		}
	}
}
