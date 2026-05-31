// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lock

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type Lock struct {
	id string
	fl *flock.Flock
}

func UniqueId(args ...any) string {
	h := sha256.New()
	for _, arg := range args {
		_, _ = fmt.Fprintf(h, "%v", arg)
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

func New(id string) *Lock {
	return &Lock{
		id,
		nil,
	}
}

func (l *Lock) Lock() error {
	p, err := lockPath(l.id)
	if err != nil {
		return fmt.Errorf("failed to get lock path: %w", err)
	}
	l.fl = flock.New(p)
	err = l.fl.Lock()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	return nil
}

func (l *Lock) Unlock() error {
	if l.fl == nil {
		return errors.New("lock is not locked")
	}
	err := l.fl.Unlock()
	if err != nil {
		return fmt.Errorf("unlock: %w", err)
	}
	return nil
}

func lockPath(id string) (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, id+".lock"), nil
	}

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", ".lockdir", os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	return filepath.Join(dir, id+".lock"), nil
}

func With(id string, fn func() error) error {
	l := New(id)
	if err := l.Lock(); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	defer func() {
		_ = l.fl.Unlock()
	}()
	return fn()
}
