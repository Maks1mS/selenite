// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	unitDir    = "/etc/systemd/system"
	dropinDir  = "/var/lib/selenite-podman-executor/.config/systemd/user/podman-restart.service.d"
	dropinName = "10-selenite-storage.conf"
	gateScript = "/usr/libexec/selenite-storage-gate"
)

// Manager reconciles the desired mount set with the selenite-managed mount
// units on disk. Filesystem paths are absolute, optionally prefixed with
// Root
type Manager struct {
	// Root is prepended to all filesystem paths.
	Root string
	// UnitDir is the directory with .mount units.
	UnitDir string
	// DropinDir is the podman-restart.service drop-in directory (podman
	// executor user).
	DropinDir string
	// GateScript is the gate script path rendered into the drop-in.
	GateScript string
	// Systemctl runs the systemd commands.
	Systemctl Systemctl
	// Warn receives warnings about tolerated failures.
	Warn io.Writer
}

func NewManager(sys Systemctl) *Manager {
	return &Manager{
		UnitDir:    unitDir,
		DropinDir:  dropinDir,
		GateScript: gateScript,
		Systemctl:  sys,
		Warn:       os.Stderr,
	}
}

// Apply reconciles the desired mount set with the managed units.
func (m *Manager) Apply(ctx context.Context, mounts []Mount) error {
	if err := Validate(mounts, "mounts"); err != nil {
		return err
	}

	desired := map[string]Mount{} // unit file name -> mount
	for _, mount := range mounts {
		desired[UnitName(mount.Target)] = mount
	}

	if err := m.removeStaleUnits(ctx, desired); err != nil {
		return err
	}
	changed, err := m.writeDesiredUnits(desired)
	if err != nil {
		return err
	}
	if err := m.Systemctl.DaemonReload(ctx); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	if err := m.enableAndStartUnits(ctx, desired, changed); err != nil {
		return err
	}
	if err := m.writeGateDropin(mounts); err != nil {
		return err
	}
	if err := m.Systemctl.ExecutorDaemonReload(ctx); err != nil {
		m.warn("warning: executor daemon-reload: %v\n", err)
	}
	return nil
}

// Check returns an error when any managed mount is not active.
func (m *Manager) Check(ctx context.Context) error {
	managed, err := m.listManagedUnits()
	if err != nil {
		return err
	}
	var inactive []string
	for _, name := range managed {
		if !m.Systemctl.IsActive(ctx, name) {
			inactive = append(inactive, name)
		}
	}
	if len(inactive) > 0 {
		return fmt.Errorf("inactive storage mounts: %s", strings.Join(inactive, ", "))
	}
	return nil
}

// Retry starts managed mounts that are not active.
func (m *Manager) Retry(ctx context.Context) error {
	managed, err := m.listManagedUnits()
	if err != nil {
		return err
	}
	for _, name := range managed {
		if !m.Systemctl.IsActive(ctx, name) {
			_ = m.Systemctl.Start(ctx, name)
		}
	}
	return nil
}

// listManagedUnits returns the names of selenite-managed mount units
// (identified by the marker).
func (m *Manager) listManagedUnits() ([]string, error) {
	entries, err := os.ReadDir(m.fsPath(m.UnitDir))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", m.UnitDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mount") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(m.fsPath(m.UnitDir), e.Name())) //nolint:gosec // file names come from a trusted directory listing
		if err != nil {
			continue
		}
		if strings.Contains(string(b), marker) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (m *Manager) removeStaleUnits(ctx context.Context, desired map[string]Mount) error {
	managed, err := m.listManagedUnits()
	if err != nil {
		return err
	}
	for _, name := range managed {
		if _, keep := desired[name]; keep {
			continue
		}
		// Best effort: a busy mount stays mounted, but the unit is removed so
		// it is not re-established on the next boot.
		_ = m.Systemctl.Stop(ctx, name)
		_ = m.Systemctl.Disable(ctx, name)
		if err := os.Remove(filepath.Join(m.fsPath(m.UnitDir), name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

// writeDesiredUnits writes the unit files that are missing or out of date
// and returns their names.
func (m *Manager) writeDesiredUnits(desired map[string]Mount) ([]string, error) {
	var changed []string
	for _, name := range sortedKeys(desired) {
		mount := desired[name]
		content := UnitContent(mount)
		path := filepath.Join(m.fsPath(m.UnitDir), name)
		//gosec:disable G304 -- unit file names come from validated mount targets
		if old, err := os.ReadFile(path); err == nil && string(old) == content {
			continue
		}
		//gosec:disable G301 -- Expected
		if err := os.MkdirAll(m.fsPath(mount.Target), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", mount.Target, err)
		}
		//gosec:disable G306 -- Expected
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		changed = append(changed, name)
	}
	return changed, nil
}

// enableAndStartUnits enables all desired units and starts them (restart if
// the unit file changed). Start failures are warnings.
func (m *Manager) enableAndStartUnits(ctx context.Context, desired map[string]Mount, changed []string) error {
	for _, name := range sortedKeys(desired) {
		if err := m.Systemctl.Enable(ctx, name); err != nil {
			return fmt.Errorf("enable %s: %w", name, err)
		}
		restart := slices.Contains(changed, name)
		var err error
		if restart {
			err = m.Systemctl.Restart(ctx, name)
		} else {
			err = m.Systemctl.Start(ctx, name)
		}
		if err != nil {
			action := "start"
			if restart {
				action = "restart"
			}
			m.warn("warning: %s %s failed (storage server offline?): %v\n", action, name, err)
		}
	}
	return nil
}

// writeGateDropin writes the podman-restart.service drop-in
// gating container start on mount readiness
func (m *Manager) writeGateDropin(mounts []Mount) error {
	path := filepath.Join(m.fsPath(m.DropinDir), dropinName)
	if len(mounts) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove gate drop-in: %w", err)
		}
		return nil
	}
	//gosec:disable G301 -- Expected
	if err := os.MkdirAll(m.fsPath(m.DropinDir), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", m.DropinDir, err)
	}
	//gosec:disable G306 -- Expected
	if err := os.WriteFile(path, []byte(GateDropinContent(mounts, m.GateScript)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// fsPath resolves an absolute host path under Root.
func (m *Manager) fsPath(path string) string {
	return filepath.Join(m.Root, path)
}

func (m *Manager) warn(format string, a ...any) {
	if m.Warn != nil {
		_, _ = fmt.Fprintf(m.Warn, format, a...)
	}
}

func sortedKeys(m map[string]Mount) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
