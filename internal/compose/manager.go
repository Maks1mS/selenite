// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"altlinux.space/alt-atomic/selenite/internal/executil"
)

var BaseStackDir = "/var/lib/selenite-podman-executor"

const (
	StacksSubdir = "stacks"
	ExecutorBin  = "/usr/libexec/selenite-controller/podman-compose-helper"

	storageHelperBin = "/usr/libexec/selenite-controller/storage-helper"
)

type Manager struct {
	baseDir string
	runner  executil.Runner
}

func NewManager() *Manager {
	return &Manager{
		baseDir: BaseStackDir,
		runner:  executil.ExecRunner{},
	}
}

func (m *Manager) stackPath(name string) string {
	return filepath.Join(m.baseDir, StacksSubdir, name)
}

func (m *Manager) run(ctx context.Context, w io.Writer, workDir string, args ...string) error {
	if err := m.runner.Run(ctx, ExecutorBin, slices.Concat([]string{workDir}, args), executil.WithCombinedOutput(w)); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func (m *Manager) checkStorageMounts(ctx context.Context) error {
	var buf bytes.Buffer
	if err := m.runner.Run(ctx, storageHelperBin, []string{"check"}, executil.WithCombinedOutput(&buf)); err != nil {
		return fmt.Errorf("storage mounts are not ready: %s: %w", strings.TrimSpace(buf.String()), err)
	}
	return nil
}

func (m *Manager) Deploy(ctx context.Context, name string, r io.Reader, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := m.checkStorageMounts(ctx); err != nil {
		return err
	}
	path := m.stackPath(name)

	// group write is required: stacks dir has sgid 2770
	// (_selenite-podman-executor:_selenite-workloads),
	// so group members must be able to traverse it
	//gosec:disable G301 -- Expected
	if err := os.MkdirAll(path, 0o770); err != nil {
		return fmt.Errorf("create stack dir: %w", err)
	}

	// group read is required: _selenite-podman-executor
	// must be able to read compose.yml written by _selenite-controller
	f, err := os.OpenFile(filepath.Join(path, "compose.yml"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640) //gosec:disable G302,G304 -- Expected
	if err != nil {
		return fmt.Errorf("open compose.yml: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return fmt.Errorf("io.Copy to compose.yml: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close compose.yml: %w", err)
	}
	return m.run(ctx, w, path, "up", "-d")
}

func (m *Manager) Remove(ctx context.Context, name string, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}

	if err := m.Down(ctx, name, w); err != nil {
		return fmt.Errorf("down: %w", err)
	}

	if err := os.RemoveAll(m.stackPath(name)); err != nil {
		return fmt.Errorf("remove all: %w", err)
	}

	return nil
}

func (m *Manager) Up(ctx context.Context, name string, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := m.checkStorageMounts(ctx); err != nil {
		return err
	}
	return m.run(ctx, w, m.stackPath(name), "up", "-d")
}

func (m *Manager) Down(ctx context.Context, name string, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}
	return m.run(ctx, w, m.stackPath(name), "down")
}

func (m *Manager) Ps(ctx context.Context, name string, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}
	return m.run(ctx, w, m.stackPath(name), "ps")
}

func (m *Manager) Logs(ctx context.Context, name string, follow bool, service string, w io.Writer) error {
	if err := validateName(name); err != nil {
		return err
	}
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}
	return m.run(ctx, w, m.stackPath(name), args...)
}

func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(m.baseDir, StacksSubdir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read stacks dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
