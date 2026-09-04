// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package authguard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	lru "github.com/hashicorp/golang-lru/v2"
)

const persistPath = "/var/lib/selenite-controller/authguard-state.json"

type snapshot struct {
	Bans     map[string]time.Time   `json:"bans"`
	Attempts map[string][]time.Time `json:"attempts"`
}

func (g *Guard) load() error {
	b, err := os.ReadFile(g.persistPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("authguard load read: %w", err)
	}
	var s snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("authguard load unmarshal: %w", err)
	}

	now := time.Now()
	cutoff := now.Add(-g.cfg.Window.OrEmpty())

	if g.bans == nil && g.cfg.Enabled.OrEmpty() {
		var err error
		g.bans, err = lru.New[string, time.Time](g.cfg.MaxTrackedIPs.OrEmpty())
		if err != nil {
			return fmt.Errorf("authguard load create bans lru: %w", err)
		}
	} else if g.bans != nil {
		g.bans.Purge()
	}
	if g.attempts == nil && g.cfg.Enabled.OrEmpty() {
		var err error
		g.attempts, err = lru.New[string, []time.Time](g.cfg.MaxTrackedIPs.OrEmpty())
		if err != nil {
			return fmt.Errorf("authguard load create attempts lru: %w", err)
		}
	} else if g.attempts != nil {
		g.attempts.Purge()
	}

	for ip, exp := range s.Bans {
		if now.Before(exp) && g.bans != nil {
			g.bans.Add(ip, exp)
		}
	}

	for ip, stamps := range s.Attempts {
		var fresh []time.Time
		for _, t := range stamps {
			if t.After(cutoff) {
				fresh = append(fresh, t)
			}
		}
		if len(fresh) > 0 && g.attempts != nil {
			g.attempts.Add(ip, fresh)
		}
	}
	return nil
}

func (g *Guard) save() error {
	if err := os.MkdirAll(filepath.Dir(g.persistPath), 0o750); err != nil {
		return fmt.Errorf("authguard save mkdir: %w", err)
	}

	lock := flock.New(g.persistPath + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("authguard save flock: %w", err)
	}
	if !locked {
		return fmt.Errorf("authguard save: could not acquire flock")
	}
	defer lock.Unlock() //nolint:errcheck

	now := time.Now()
	cutoff := now.Add(-g.cfg.Window.OrEmpty())
	s := snapshot{
		Bans:     make(map[string]time.Time),
		Attempts: make(map[string][]time.Time),
	}

	if g.bans != nil {
		for _, ip := range g.bans.Keys() {
			exp, ok := g.bans.Peek(ip)
			if !ok {
				continue
			}
			if now.Before(exp) {
				s.Bans[ip] = exp
			}
		}
	}

	if g.attempts != nil {
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
			if len(fresh) > 0 {
				s.Attempts[ip] = fresh
			}
		}
	}

	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("authguard save marshal: %w", err)
	}
	if err := os.WriteFile(g.persistPath, b, 0o600); err != nil {
		return fmt.Errorf("authguard save write: %w", err)
	}
	return nil
}
